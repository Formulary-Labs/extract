package screen

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Formulary-Labs/substrate/artifact"
)

// --- helpers ----------------------------------------------------------------

func loadCatalog(t *testing.T) *artifact.ControlCatalog {
	t.Helper()
	p := fixturePath(t, "layer1.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read catalog %s: %v", p, err)
	}
	tmp := filepath.Join(t.TempDir(), "layer1.yaml")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatalf("write temp catalog: %v", err)
	}
	loaded, err := artifact.LoadControlCatalog(tmp)
	if err != nil {
		t.Fatalf("load catalog %s: %v", tmp, err)
	}
	if len(loaded.Controls) == 0 {
		t.Fatalf("catalog loaded with %d controls", len(loaded.Controls))
	}
	return loaded
}

func loadKit(t *testing.T, name string) *Kit {
	t.Helper()
	p := fixturePath(t, name)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read kit %s: %v", p, err)
	}
	var k Kit
	if err := json.Unmarshal(data, &k); err != nil {
		t.Fatalf("parse kit %s: %v", p, err)
	}
	return &k
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	root := moduleRoot(t)
	return filepath.Join(root, "data", "sample", name)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	here, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(here, "go.mod")); err == nil {
			return here
		}
		parent := filepath.Dir(here)
		if parent == here {
			t.Fatal("no go.mod found walking up")
		}
		here = parent
	}
}

func findingCode(r *Result, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// --- pass --------------------------------------------------------------------

func TestScreen_Pass(t *testing.T) {
	cat := loadCatalog(t)
	kit := loadKit(t, "onboarding.json")
	r := Screen(cat, *kit)
	if r.Verdict != VerdictPass {
		t.Fatalf("expected verdict %q, got %q", VerdictPass, r.Verdict)
	}
	if r.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", r.ExitCode)
	}
	if len(r.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(r.Findings))
	}
	if r.Total != len(kit.Controls) {
		t.Fatalf("expected total %d, got %d", len(kit.Controls), r.Total)
	}
	if r.Passed != len(kit.Controls) {
		t.Fatalf("expected all controls passed, got passed=%d failed=%d", r.Passed, r.Failed)
	}
	if r.Claims.Missing != 0 {
		t.Fatalf("expected 0 missing claims, got %d", r.Claims.Missing)
	}
}

// --- reject (hard gate: unknown control id) ---------------------------------

func TestScreen_Reject(t *testing.T) {
	cat := loadCatalog(t)
	kit := &Kit{
		Product:   "Broken",
		ProductType: "saas",
		Framework: "iso27001",
		Controls:  []ControlRef{{ID: "ZZ.Z99", Title: "Not a real control", Owner: "x"}},
	}
	r := Screen(cat, *kit)
	if r.Verdict != VerdictReject {
		t.Fatalf("expected verdict %q, got %q (exit %d)", VerdictReject, r.Verdict, r.ExitCode)
	}
	if r.ExitCode != 1 {
		t.Fatalf("reject maps to Validation (1), got %d", r.ExitCode)
	}
	if !findingCode(&r, "missing_control_id") {
		t.Fatalf("expected missing_control_id finding")
	}
}

// --- partial (recoverable defect) -------------------------------------------

func TestScreen_Partial_PendingNoRisk(t *testing.T) {
	cat := loadCatalog(t)
	kit := &Kit{
		Product:     "Sample SaaS",
		ProductType: "saas",
		Framework:   "iso27001",
		Controls: []ControlRef{
			{ID: "A.5.1", Title: "Information Security Policy", Owner: "policy-team",
				Framework: "iso27001",
				Evidence: []Evidence{{Type: "policy", URL: "https://conf.example.com/policy", State: "pending"}}},
			{ID: "A.8.2", Title: "Access Control", Owner: "sec-team",
				Framework: "iso27001",
				Evidence: []Evidence{{Type: "configuration", URL: "https://conf.example.com/acl", State: "present", Risk: "RISK-ACCESS"}}},
		},
	}
	r := Screen(cat, *kit)
	if r.Verdict != VerdictPartial {
		t.Fatalf("expected %q, got %q (exit %d)", VerdictPartial, r.Verdict, r.ExitCode)
	}
	if r.ExitCode != 1 {
		t.Fatalf("partial maps to Validation (1), got %d", r.ExitCode)
	}
	if !findingCode(&r, "evidence_pending_no_risk") {
		t.Fatalf("expected evidence_pending_no_risk")
	}
}

// --- individual finding codes -----------------------------------------------

func TestScreen_TitleMismatch(t *testing.T) {
	cat := loadCatalog(t)
	kit := &Kit{
		Product:     "Sample",
		ProductType: "saas",
		Framework:   "iso27001",
		Controls: []ControlRef{{ID: "A.5.1", Title: "Totally Different Title", Owner: "t", Framework: "iso27001"}},
	}
	r := Screen(cat, *kit)
	if !findingCode(&r, "title_mismatch") {
		t.Fatalf("expected title_mismatch; findings=%v", r.Findings)
	}
}

func TestScreen_OwnerNeeded(t *testing.T) {
	cat := loadCatalog(t)
	kit := &Kit{
		Product:     "Sample",
		ProductType: "saas",
		Framework:   "iso27001",
		Controls:  []ControlRef{{ID: "A.5.1", Title: "Information Security Policy", Owner: "", Framework: "iso27001"}},
	}
	r := Screen(cat, *kit)
	if !findingCode(&r, "owner_needed") {
		t.Fatalf("expected owner_needed")
	}
}

// --- flags -------------------------------------------------------------------

func TestScreen_Flags(t *testing.T) {
	cat := loadCatalog(t)
	kit := &Kit{
		Product:     "Sample",
		ProductType: "saas",
		Framework:   "iso27001",
		Controls: []ControlRef{{ID: "A.5.1", Title: "Information Security Policy", Owner: "t", Framework: "iso27001"}},
	}
	r := Screen(cat, *kit)
	if len(r.Flags) == 0 {
		t.Fatalf("expected flags for unsatisfied claims, got none")
	}
}

// --- determinism (no clock, no randomness) -----------------------------------

func TestScreen_Determinism(t *testing.T) {
	cat := loadCatalog(t)
	kit := loadKit(t, "onboarding.json")
	a := Screen(cat, *kit)
	b := Screen(cat, *kit)
	ab, _ := json.MarshalIndent(a, "", "  ")
	bb, _ := json.MarshalIndent(b, "", "  ")
	if !bytes.Equal(ab, bb) {
		t.Fatalf("determinism violated:\nA:\n%s\nB:\n%s", ab, bb)
	}
}

// --- byte-stable JSON ---------------------------------------------------------

func TestScreen_ResultJSONStable(t *testing.T) {
	cat := loadCatalog(t)
	kit := loadKit(t, "onboarding.json")
	r := Screen(cat, *kit)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty result JSON")
	}
}
