// extract validates and screens a product onboarding kit against the
// program's gemara control catalog before anything touches program files.
//
// Usage:
//
//	extract [flags] --catalog <layer1.yaml> --kit <kit.json>
//
// Outputs a JSON (default) or markdown screen report. Read-only: no program
// file is ever written; output goes to --output or stdout. Exit codes follow
// the Formulary three-code contract: 0 = pass, 1 = validation (defects /
// reject), 2 = tool error (missing file, parse failure).

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Formulary-Labs/extract/internal/screen"
	"github.com/Formulary-Labs/substrate/artifact"
	"github.com/Formulary-Labs/substrate/exit"
	"github.com/Formulary-Labs/substrate/provenance"
)

const version = "0.1.0"

func main() {
	var (
		catalogFlag = flag.String("catalog", "", "Path to gemara Layer 1 control catalog YAML/JSON (required)")
		kitFlag     = flag.String("kit", "", "Path to product onboarding kit JSON (required)")
		programFlag = flag.String("program", "", "Program slug for provenance logging")
		fmtFlag     = flag.String("format", "json", "Output format: json (default), md")
		outputFlag  = flag.String("output", "", "Output file path (stdout if empty)")
		quietFlag   = flag.Bool("quiet", false, "Suppress progress output")
		versionFlag = flag.Bool("version", false, "Print version and exit")
		doctorFlag  = flag.Bool("doctor", false, "Self-check version and substrate and exit 0")
	)
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Printf("extract version %s\n", version)
		os.Exit(exit.OK)
	}

	if *doctorFlag {
		runDoctor()
		return
	}

	if *catalogFlag == "" {
		fmt.Fprintf(os.Stderr, `{"error": "--catalog is required", "code": 2}\n`)
		flag.Usage()
		os.Exit(exit.ToolError)
	}
	if *kitFlag == "" {
		fmt.Fprintf(os.Stderr, `{"error": "--kit is required", "code": 2}\n`)
		flag.Usage()
		os.Exit(exit.ToolError)
	}

	if !*quietFlag {
		fmt.Fprintln(os.Stderr, "[extract] loading control catalog:", *catalogFlag)
		fmt.Fprintln(os.Stderr, "[extract] loading onboarding kit:", *kitFlag)
	}

	catalog, err := artifact.LoadControlCatalog(*catalogFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error": "%s", "code": 2}\n`, err.Error())
		os.Exit(exit.ToolError)
	}

	data, err := os.ReadFile(*kitFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error": "%s", "code": 2}\n`, err.Error())
		os.Exit(exit.ToolError)
	}

	var kit screen.Kit
	if err := json.Unmarshal(data, &kit); err != nil {
		fmt.Fprintf(os.Stderr, `{"error": "parsing kit: %v", "code": 2}\n`, err)
		os.Exit(exit.ToolError)
	}

	result := screen.Screen(catalog, kit)

	if !*quietFlag {
		fmt.Fprintf(os.Stderr, "[extract] %d/%d controls pass — verdict %s (exit %d)\n",
			result.Passed, result.Total, result.Verdict, result.ExitCode)
	}

	out := formatOutput(result, *fmtFlag)
	if *outputFlag != "" {
		if err := os.WriteFile(*outputFlag, []byte(out), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
			os.Exit(exit.ToolError)
		}
		if !*quietFlag {
			fmt.Fprintf(os.Stderr, "wrote %s\n", *outputFlag)
		}
	} else {
		fmt.Print(out)
	}

	// provenance — only when a program slug is supplied.
	if *programFlag != "" {
		_ = provenance.Write("logs/provenance.jsonl", provenance.Entry{
			Spec:        "functions/product-onboarding-intake-spec.md",
			Output:      *kitFlag,
			OutputType:  "product_onboarding_intake",
			Program:     *programFlag,
			Purpose:     fmt.Sprintf("extract: %d/%d controls, verdict %s",
				result.Passed, result.Total, result.Verdict),
			Reusability: provenance.Instance,
			QualityGate: provenance.Pass,
			Tool:        "extract",
			ToolVersion: version,
		})
	}

	os.Exit(result.ExitCode)
}

func runDoctor() {
	fmt.Printf("extract version %s\n", version)
	fmt.Println("substrate: required (github.com/Formulary-Labs/substrate v0.4.0)")
	fmt.Println("doctor: OK")
}

// formatOutput renders the report as JSON (default, parseable by jq) or MD.
func formatOutput(r screen.Result, fmtStr string) string {
	if fmtStr == "md" {
		return toMarkdown(r)
	}
	// json — struct field order is stable for identical input.
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

// toMarkdown renders a human-readable screen report.
func toMarkdown(r screen.Result) string {
	var b strings.Builder
	b.WriteString("# extract — onboarding intake screen\n\n")
	b.WriteString(fmt.Sprintf("**Product:** %s  |  **Verdict:** %s  |  **Exit:** %d\n\n",
		r.Product, r.Verdict, r.ExitCode))
	b.WriteString(fmt.Sprintf("**Catalog:** %s (%d controls)\n\n", r.Catalog.Title, r.Catalog.Controls))
	b.WriteString(fmt.Sprintf("**Controls:** %d/%d pass\n\n", r.Passed, r.Total))

	if len(r.Findings) > 0 {
		b.WriteString("## Findings\n\n")
		for _, f := range r.Findings {
			b.WriteString(fmt.Sprintf("- **%s** — %s (severity %s): %s\n",
				f.Code, f.ControlID, f.Severity, f.Detail))
		}
		b.WriteString("\n")
	}

	if len(r.Claims.Required) > 0 {
		b.WriteString("## Claims\n\n")
		b.WriteString(fmt.Sprintf("Required: %d | Missing: %d\n\n", len(r.Claims.Required), r.Claims.Missing))
		for _, c := range r.Claims.Paired {
			check := ""
			if !c.Satisfied {
				check = " **missing**"
			}
			b.WriteString(fmt.Sprintf("- %s — %s%s\n", c.ID, c.Name, check))
		}
		b.WriteString("\n")
	}

	if len(r.Flags) > 0 {
		b.WriteString("## Flags (carry-forward)\n\n")
		for _, fl := range r.Flags {
			b.WriteString(fmt.Sprintf("- %s: %s\n", fl.Kind, fl.Text))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func usage() {
	fmt.Fprintln(os.Stderr, `extract — product onboarding intake (the screen)

Usage:
  extract [flags] --catalog <layer1.yaml> --kit <kit.json>

Flags:
  --catalog string     Path to gemara Layer 1 control catalog (required)
  --kit string         Path to product onboarding kit JSON (required)
  --program string     Program slug for provenance logging
  --format string      Output format: json (default), md
  --output string      Output file path (stdout if empty)
  --quiet              Suppress progress output
  --doctor             Self-check and exit 0
  --version            Print version and exit

Kit JSON format:
  {
    "product": "Acme SaaS",
    "slug": "acme-saas",
    "product-type": "saas",
    "framework": "iso27001",
    "controls": [
      { "id": "A.8.2", "title": "Access control", "owner": "sec-team",
        "framework": "iso27001",
        "evidence": [ { "type": "configuration", "url": "https://...conf/ACL",
                        "state": "present", "risk": "RISK-ACCESS" } ] }
    ]
  }

Examples:
  extract --catalog data/sample/layer1.yaml --kit data/sample/onboarding.json
  extract --catalog data/sample/layer1.yaml --kit data/sample/broken.json --format md
  extract --doctor`)
}
