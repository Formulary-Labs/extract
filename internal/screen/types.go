package screen

// --- Kit: the onboarding package handed off for screening -----------------

// Evidence is a single piece of evidence attached to a control.
type Evidence struct {
	// Type is the evidence kind (e.g. "policy", "configuration", "screenshot").
	Type string `json:"type"`
	// URL is the evidence location. PENDING/TODO placeholders are detected
	// by evidence.ClassifyLink (see internal use of the substrate evidence pkg).
	URL string `json:"url"`
	// State is the evidence status: "present", "pending", "todo", or empty.
	State string `json:"state,omitempty"`
	// Risk is the associated risk entry id. PENDING evidence without a Risk
	// is a package defect (DATA NEEDED: risk).
	Risk string `json:"risk,omitempty"`
}

// ControlRef is one control claim from the onboarding kit. The "id" and "title"
// are what `extract` screens against the gemara control catalog.
type ControlRef struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Owner    string   `json:"owner,omitempty"`
	Framework string  `json:"framework,omitempty"`
	Evidence []Evidence `json:"evidence,omitempty"`
}

// Kit is the parsed onboarding package.
type Kit struct {
	Product     string       `json:"product"`
	Slug        string       `json:"slug"`
	ProductType string       `json:"product-type"`
	Framework   string       `json:"framework"`
	Controls    []ControlRef `json:"controls"`
}

// --- Claim registry (the screen matrix) ------------------------------------

// ClaimDef is a deterministic claim definition in the built-in registry.
// A claim is "required" for a (productType, framework) combination when
// the combination lies within the claim's applicability set.
type ClaimDef struct {
	ID             string
	Name           string
	// ProductTypes: the claim applies to these types. Empty = all types.
	ProductTypes []string
	// Frameworks: the claim applies to these frameworks. Empty = all frameworks.
	Frameworks   []string
	// RequiredEvidence is the evidence link type that must be attached to the
	// claim's control for the screen to pass.
	RequiredEvidence string
	// RiskKey is the risk entry name expected when evidence is present.
	RiskKey string
}

// Frameworks is the canonical list of supported frameworks (the screen domain).
var Frameworks = []string{
	"nists800-53", "iso27001", "iso27701", "iso42001", "soc2", "soc1", "hipaa",
	"fedramp-high", "fedramp-medium", "pci-dss", "cmmc", "cisa-css",
	"eu-ai-act", "iec-62443", "nist-ai-rmf", "cis-benchmark",
}

// ProductTypes is the canonical list of supported product types (the screen domain).
var ProductTypes = []string{
	"saas", "mobile-app", "desktop", "api", "iot",
	"cloud-infra", "data-warehouse", "ml-model",
}

// ClaimDefs is the built-in registry of screenable claims (33 definitions).
// It is the app's first-party, deterministic knowledge of which claims a
// (productType, framework) combination must satisfy. All of them are required
// by at least one combination, so the full 8x15 matrix references all 33.
var ClaimDefs = []ClaimDef{
	// --- base claims: required by every combination ------------------------
	{ID: "CLM-001", Name: "Security policy", ProductTypes: nil, Frameworks: nil, RequiredEvidence: "policy", RiskKey: "RISK-POLICY"},
	{ID: "CLM-002", Name: "Roles and responsibilities", ProductTypes: nil, Frameworks: nil, RequiredEvidence: "policy", RiskKey: "RISK-ROLE"},
	{ID: "CLM-003", Name: "Asset inventory", ProductTypes: nil, Frameworks: nil, RequiredEvidence: "configuration", RiskKey: "RISK-ASSET"},
	{ID: "CLM-004", Name: "Access control", ProductTypes: nil, Frameworks: nil, RequiredEvidence: "configuration", RiskKey: "RISK-ACCESS"},
	{ID: "CLM-005", Name: "Encryption in transit", ProductTypes: nil, Frameworks: nil, RequiredEvidence: "configuration", RiskKey: "RISK-ENCRYPT"},
	{ID: "CLM-006", Name: "Encryption at rest", ProductTypes: nil, Frameworks: nil, RequiredEvidence: "configuration", RiskKey: "RISK-ENCRYPT"},
	{ID: "CLM-007", Name: "Vulnerability management", ProductTypes: nil, Frameworks: nil, RequiredEvidence: "scan", RiskKey: "RISK-VULN"},
	{ID: "CLM-008", Name: "Incident response", ProductTypes: nil, Frameworks: nil, RequiredEvidence: "policy", RiskKey: "RISK-INCIDENT"},

	// --- type-specific ---------------------------------------------------
	{ID: "CLM-009", Name: "Data at rest protection", ProductTypes: []string{"data-warehouse", "saas", "ml-model"}, Frameworks: nil, RequiredEvidence: "configuration", RiskKey: "RISK-DATA"},
	{ID: "CLM-010", Name: "Device hardening", ProductTypes: []string{"desktop", "mobile-app", "iot"}, Frameworks: nil, RequiredEvidence: "configuration", RiskKey: "RISK-DEVICE"},
	{ID: "CLM-011", Name: "Network segmentation", ProductTypes: []string{"cloud-infra", "data-warehouse"}, Frameworks: nil, RequiredEvidence: "architecture", RiskKey: "RISK-NET"},
	{ID: "CLM-012", Name: "API security testing", ProductTypes: []string{"api", "saas"}, Frameworks: nil, RequiredEvidence: "scan", RiskKey: "RISK-API"},
	{ID: "CLM-013", Name: "Mobile data exfiltration guard", ProductTypes: []string{"mobile-app"}, Frameworks: nil, RequiredEvidence: "code-review", RiskKey: "RISK-EXFIL"},
	{ID: "CLM-014", Name: "IoT device attestation", ProductTypes: []string{"iot"}, Frameworks: nil, RequiredEvidence: "configuration", RiskKey: "RISK-IOT"},
	{ID: "CLM-015", Name: "Model input validation", ProductTypes: []string{"ml-model"}, Frameworks: nil, RequiredEvidence: "test", RiskKey: "RISK-MODEL"},

	// --- framework-specific ------------------------------------------------
	{ID: "CLM-016", Name: "FedRAMP authorizing officer approval", ProductTypes: nil, Frameworks: []string{"fedramp-high"}, RequiredEvidence: "policy", RiskKey: "RISK-FEDRAMP"},
	{ID: "CLM-017", Name: "PCI DSS network controls", ProductTypes: nil, Frameworks: []string{"pci-dss"}, RequiredEvidence: "configuration", RiskKey: "RISK-PCI"},
	{ID: "CLM-018", Name: "HIPAA minimum necessary", ProductTypes: nil, Frameworks: []string{"hipaa"}, RequiredEvidence: "policy", RiskKey: "RISK-HIPAA"},
	{ID: "CLM-019", Name: "CMMC zero trust", ProductTypes: nil, Frameworks: []string{"cmmc"}, RequiredEvidence: "architecture", RiskKey: "RISK-ZTC"},
	{ID: "CLM-020", Name: "CISA secure software", ProductTypes: nil, Frameworks: []string{"cisa-css"}, RequiredEvidence: "policy", RiskKey: "RISK-CSS"},
	{ID: "CLM-021", Name: "EU AI Act governance", ProductTypes: nil, Frameworks: []string{"eu-ai-act"}, RequiredEvidence: "policy", RiskKey: "RISK-AIACT"},
	{ID: "CLM-022", Name: "IEC 62443 operational security", ProductTypes: nil, Frameworks: []string{"iec-62443"}, RequiredEvidence: "configuration", RiskKey: "RISK-62443"},
	{ID: "CLM-023", Name: "NIST AI RMF AI controls", ProductTypes: nil, Frameworks: []string{"nist-ai-rmf"}, RequiredEvidence: "policy", RiskKey: "RISK-AIRMF"},
	{ID: "CLM-024", Name: "SOC 2 CC1 governance", ProductTypes: nil, Frameworks: []string{"soc2"}, RequiredEvidence: "policy", RiskKey: "RISK-SOC2"},
	{ID: "CLM-025", Name: "SOC 1 operational controls", ProductTypes: nil, Frameworks: []string{"soc1"}, RequiredEvidence: "configuration", RiskKey: "RISK-SOC1"},
	{ID: "CLM-026", Name: "ISO 27701 privacy", ProductTypes: nil, Frameworks: []string{"iso27701"}, RequiredEvidence: "policy", RiskKey: "RISK-PRIV"},
	{ID: "CLM-027", Name: "NIST 800-53 control mapping", ProductTypes: nil, Frameworks: []string{"nists800-53"}, RequiredEvidence: "policy", RiskKey: "RISK-800"},
	{ID: "CLM-028", Name: "CIS benchmark compliance", ProductTypes: nil, Frameworks: []string{"cis-benchmark"}, RequiredEvidence: "scan", RiskKey: "RISK-CIS"},
	{ID: "CLM-029", Name: "ISO 27001 risk treatment", ProductTypes: nil, Frameworks: []string{"iso27001"}, RequiredEvidence: "policy", RiskKey: "RISK-RISK"},
	{ID: "CLM-030", Name: "ISO 27001 context", ProductTypes: nil, Frameworks: []string{"iso27001"}, RequiredEvidence: "policy", RiskKey: "RISK-CTX"},
	{ID: "CLM-031", Name: "NIST 800-53 baseline alignment", ProductTypes: []string{"saas", "cloud-infra"}, Frameworks: []string{"nists800-53"}, RequiredEvidence: "policy", RiskKey: "RISK-BASE"},
	{ID: "CLM-032", Name: "AI model risk management", ProductTypes: []string{"ml-model", "saas"}, Frameworks: []string{"eu-ai-act", "nist-ai-rmf"}, RequiredEvidence: "policy", RiskKey: "RISK-MLR"},
	{ID: "CLM-033", Name: "Federated identity", ProductTypes: []string{"saas", "mobile-app", "api"}, Frameworks: []string{"fedramp-high", "soc2", "iso27001"}, RequiredEvidence: "configuration", RiskKey: "RISK-IDM"},
}

// requiredClaims returns the claim definitions required for the given
// (productType, framework) combination. Empty = no extra claim (all are base).
func requiredClaims(productType, framework string) []ClaimDef {
	var out []ClaimDef
	for _, c := range ClaimDefs {
		if !inSet(productType, c.ProductTypes) {
			continue
		}
		if !inSet(framework, c.Frameworks) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// inSet returns true when set is nil (apply to all) or s is a member.
func inSet(s string, set []string) bool {
	if len(set) == 0 {
		return true
	}
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// productTypeIn and frameworkIn validate the kit's declared domain against the registry.
func productTypeIn(pt string) bool { return inSet(pt, ProductTypes) }
func frameworkIn(fw string) bool { return inSet(fw, Frameworks) }
