package screen

import (
	"fmt"
	"strings"

	"github.com/Formulary-Labs/substrate/artifact"
)

// --- Result and findings ----------------------------------------------------

// Severity ranks a finding from harmless to blocking.
type Severity string

const (
	SeverityNone     Severity = "none"     // clean
	SeverityLow      Severity = "low"      // advisory
	SeverityMedium   Severity = "medium"   // needs owner action
	SeverityHigh     Severity = "high"     // needs attention
	SeverityCritical Severity = "critical" // blocks the run
)

// Verdict is the screen's overall outcome.
type Verdict string

const (
	VerdictPass    Verdict = "pass"    // exit 0 — everything screens clean
	VerdictPartial Verdict = "partial" // exit 1 — defects found, recoverable
	VerdictReject  Verdict = "reject"  // exit 1 — hard-gate failure (missing control)
)

// Finding is a single screenable issue.
type Finding struct {
	ControlID string   `json:"control_id"`
	Severity  Severity `json:"severity"`
	Code      string   `json:"code"`
	Detail    string   `json:"detail"`
}

// ClaimStatus summarizes one required claim's evidence pairing.
type ClaimStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Satisfied bool   `json:"satisfied"`
	Missing   string `json:"missing,omitempty"`
}

// ClaimResult is the output of Stage 2 (claim→evidence pairing).
type ClaimResult struct {
	Required []ClaimDef  `json:"required"`
	Paired   []ClaimStatus `json:"paired"`
	Missing  int          `json:"missing"`
}

// CatalogInfo is the summary of the gemara catalog used to screen.
type CatalogInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Controls int    `json:"controls"`
}

// Flag mirrors the regimen flag vocabulary (carry-forward, never corrected).
type Flag struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// Result is the full screen report. Byte-stable for identical input: all
// fields are derived from deterministic logic over the catalog + kit; no
// clock, no randomness, no LLM.
type Result struct {
	Product   string        `json:"product"`
	Verdict   Verdict       `json:"verdict"`
	ExitCode  int           `json:"exit_code"` // 0 pass / 1 validation
	Total     int           `json:"total_controls"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Catalog   CatalogInfo   `json:"catalog"`
	Findings  []Finding     `json:"findings,omitempty"`
	Claims    ClaimResult   `json:"claims"`
	Flags     []Flag        `json:"flags,omitempty"`
}

// --- helpers ----------------------------------------------------------------

// normTitle collapses whitespace so a title comparison is robust.
func normTitle(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// rankSeverity gives an order so maxSeverity is trivial to compute.
func rankSeverity(s Severity) int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	default:
		return 0
	}
}

// maxSeverity returns the worst severity in the set (worst wins).
func maxSeverity(sevs []Severity) Severity {
	worst := SeverityNone
	for _, s := range sevs {
		if rankSeverity(s) > rankSeverity(worst) {
			worst = s
		}
	}
	return worst
}

// --- Stage 1: screen controls against the catalog ---------------------------

// screenControls validates every kit control against the gemara catalog
// (ID existence + title match) and checks owner and evidence. It returns
// a flat list of findings and a per-control worst severity.
func screenControls(catalog map[string]artifact.Control, controls []ControlRef) ([]Finding, []Severity) {
	var findings []Finding
	var perCtrl []Severity
	for _, cr := range controls {
		ctrls := []Severity{}
		ctrl, ok := catalog[cr.ID]
		if !ok {
			ctrls = append(ctrls, SeverityCritical)
			findings = append(findings, Finding{
				ControlID: cr.ID,
				Severity:  SeverityCritical,
				Code:      "missing_control_id",
				Detail:    fmt.Sprintf("control %q not present in gemara catalog — verify the catalog or the kit", cr.ID),
			})
			perCtrl = append(perCtrl, maxSeverity(ctrls))
			continue
		}
		// title match — the catalog is the source of truth; never auto-correct.
		if cr.Title != "" && normTitle(cr.Title) != normTitle(ctrl.Title) {
			ctrls = append(ctrls, SeverityHigh)
			findings = append(findings, Finding{
				ControlID: cr.ID,
				Severity:  SeverityHigh,
				Code:      "title_mismatch",
				Detail:    fmt.Sprintf("kit title %q differs from catalog %q — [CONFLICT — VERIFY]", cr.Title, ctrl.Title),
			})
		}
		// owner.
		if strings.TrimSpace(cr.Owner) == "" {
			ctrls = append(ctrls, SeverityMedium)
			findings = append(findings, Finding{
				ControlID: cr.ID,
				Severity:  SeverityMedium,
				Code:      "owner_needed",
				Detail:    "no control owner named — [OWNER NEEDED]",
			})
		}
		// PENDING/TODO evidence without a risk entry is a package defect.
		for i, ev := range cr.Evidence {
			up := strings.ToUpper(strings.TrimSpace(ev.State))
			if (up == "PENDING" || up == "TODO" || ev.URL == "") && ev.Risk == "" {
				ctrls = append(ctrls, SeverityHigh)
				findings = append(findings, Finding{
					ControlID: cr.ID,
					Severity:  SeverityHigh,
					Code:      "evidence_pending_no_risk",
					Detail:    fmt.Sprintf("evidence %d is %q with no associated risk entry — [DATA NEEDED: risk]", i+1, strings.TrimSpace(ev.State)),
				})
			}
		}
		perCtrl = append(perCtrl, maxSeverity(ctrls))
	}
	return findings, perCtrl
}

// --- Stage 2: claim→evidence pairing ----------------------------------------

// pairClaims checks each required claim for a matching, non-placeholder
// evidence entry on the kit. A claim is satisfied when at least one kit
// control attaches evidence of the required type that is not PENDING/TODO.
func pairClaims(controls []ControlRef, required []ClaimDef) ClaimResult {
	var paired []ClaimStatus
	missing := 0
	for _, c := range required {
		sat := false
		for _, cr := range controls {
			for _, ev := range cr.Evidence {
				u := strings.TrimSpace(ev.URL)
				if u == "" {
					continue
				}
				if !isPlaceholder(ev) && ev.Type == c.RequiredEvidence {
					sat = true
					break
				}
			}
			if sat {
				break
			}
		}
		if sat {
			paired = append(paired, ClaimStatus{ID: c.ID, Name: c.Name, Satisfied: true})
		} else {
			missing++
			paired = append(paired, ClaimStatus{
				ID:        c.ID,
				Name:      c.Name,
				Satisfied: false,
				Missing:   fmt.Sprintf("claim %s (%s) has no real evidence on the kit", c.ID, c.Name),
			})
		}
	}
	return ClaimResult{Required: required, Paired: paired, Missing: missing}
}

// isPlaceholder reports whether an evidence entry is PENDING/TODO/empty.
func isPlaceholder(ev Evidence) bool {
	u := strings.TrimSpace(ev.URL)
	if u == "" {
		return true
	}
	if strings.HasPrefix(strings.ToUpper(u), "PENDING") {
		return true
	}
	if strings.ToUpper(strings.TrimSpace(ev.State)) == "TODO" {
		return true
	}
	return false
}

// --- Screen: the top-level entrypoint ---------------------------------------

// Screen runs the full screen: Stage 1 (catalog) + Stage 2 (claims), and
// produces a byte-stable Result. It is pure — no I/O, no clock, no LLM.
func Screen(catalog *artifact.ControlCatalog, kit Kit) Result {
	// build an index of catalog controls for O(1) ID lookups.
	index := make(map[string]artifact.Control)
	for _, c := range catalog.Controls {
		index[c.Id] = c
	}

	findings, _ := screenControls(index, kit.Controls)

	// required claims for this product+framework.
	reqClaims := requiredClaims(kit.ProductType, kit.Framework)
	claims := pairClaims(kit.Controls, reqClaims)

	// count passed vs failed controls (a control failed if it has any finding).
	failed := len(findings)
	passed := len(kit.Controls) - failed
	// recompute: a control "passes" if it has no findings.
	failedSet := make(map[string]bool)
	for _, f := range findings {
		failedSet[f.ControlID] = true
	}
	for _, cr := range kit.Controls {
		if !failedSet[cr.ID] {
			// control is clean
			_ = cr
		}
	}
	passed = 0
	for _, cr := range kit.Controls {
		if !failedSet[cr.ID] {
			passed++
		}
	}
	failed = len(kit.Controls) - passed

	// verdict + exit code.
	var verdict Verdict
	// reject if any control ID is missing from the catalog (hard gate).
	for _, f := range findings {
		if f.Code == "missing_control_id" {
			verdict = VerdictReject
			break
		}
	}
	if verdict == "" {
		if len(findings) == 0 && claims.Missing == 0 {
			verdict = VerdictPass
		} else {
			verdict = VerdictPartial
		}
	}

	// flags (carry-forward, never corrected).
	flags := buildFlags(findings, claims, kit)

	return Result{
		Product:  kit.Product,
		Verdict:  verdict,
		ExitCode: exitCodeFor(verdict),
		Total:    len(kit.Controls),
		Passed:   passed,
		Failed:   failed,
		Catalog: CatalogInfo{
			ID:       catalog.Metadata.Id,
			Title:    catalog.Title,
			Controls: len(catalog.Controls),
		},
		Findings: findings,
		Claims:   claims,
		Flags:    flags,
	}
}

// exitCodeFor maps verdict to the house three-code contract
// (0 OK, 1 Validation, 2 ToolError — ToolError is set by the caller on
// load/parse errors).
func exitCodeFor(v Verdict) int {
	switch v {
	case VerdictPass:
		return 0
	default:
		return 1 // Validation
	}
}

// buildFlags produces the carry-forward flag list from findings + claims.
func buildFlags(findings []Finding, claims ClaimResult, kit Kit) []Flag {
	var flags []Flag
	for _, f := range findings {
		switch f.Code {
		case "owner_needed":
			flags = append(flags, Flag{Kind: "owner_needed", Text: fmt.Sprintf("control %s has no owner", f.ControlID)})
		case "evidence_pending_no_risk":
			flags = append(flags, Flag{Kind: "data_needed_risk", Text: fmt.Sprintf("control %s has pending evidence without a risk entry", f.ControlID)})
		case "title_mismatch":
			flags = append(flags, Flag{Kind: "conflict_verify", Text: f.Detail})
		}
	}
	// any unsatisfied required claim is a data-needed flag.
	for _, cs := range claims.Paired {
		if !cs.Satisfied {
			flags = append(flags, Flag{Kind: "missing_evidence", Text: cs.Missing})
		}
	}
	_ = kit
	return flags
}
