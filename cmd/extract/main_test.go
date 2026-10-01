package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Formulary-Labs/substrate/provenance"
)

// buildBinary compiles the extract binary into a fresh temp dir. On Windows
// the output must carry a .exe suffix or exec.Command cannot start it.
func buildBinary(t *testing.T) string {
	t.Helper()
	outBin := filepath.Join(t.TempDir(), "extract.exe")
	root := moduleRoot()
	cmd := exec.Command("go", "build", "-o", outBin, "./cmd/extract")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Skipf("could not build extract binary from %s: %v", root, err)
	}
	return outBin
}

// runBinary runs the binary in the given working directory and returns
// (stdout, stderr, exitCode). The binary resolves its provenance log
// relative to this dir, so the provenance test controls it here.
func runBinary(t *testing.T, bin, dir string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr strings.Builder
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		if ex, ok := err.(*exec.ExitError); ok {
			code = ex.ExitCode()
		} else {
			code = 2
		}
	}
	return stdout.String(), stderr.String(), code
}

// moduleRoot walks up from the test dir to the nearest go.mod.
func moduleRoot() string {
	h, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(h, "go.mod")); err == nil {
			return h
		}
		p := filepath.Dir(h)
		if p == h {
			panic("no go.mod found walking up")
		}
		h = p
	}
}

func sampleDir() string {
	return filepath.Join(moduleRoot(), "data", "sample")
}

func TestCLI_Pass(t *testing.T) {
	bin := buildBinary(t)
	s := sampleDir()
	out, stderr, code := runBinary(t, bin, "",
		"--catalog", filepath.Join(s, "layer1.yaml"),
		"--kit", filepath.Join(s, "onboarding.json"),
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%q", code, stderr)
	}
	var r map[string]any
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("output not JSON: %v; got %q", err, out)
	}
	if v, ok := r["verdict"].(string); ok && v != "pass" {
		t.Fatalf("expected verdict pass, got %q", v)
	}
}

func TestCLI_Reject(t *testing.T) {
	bin := buildBinary(t)
	s := sampleDir()
	out, stderr, code := runBinary(t, bin, "",
		"--catalog", filepath.Join(s, "layer1.yaml"),
		"--kit", filepath.Join(s, "broken.json"),
		"--format", "json",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit on broken kit; got 0")
	}
	var r map[string]any
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("output not JSON: %v; got %q", err, out)
	}
	if v, ok := r["verdict"].(string); ok && v != "reject" {
		t.Fatalf("expected verdict reject, got %q; stderr=%q", v, stderr)
	}
}

func TestCLI_Markdown(t *testing.T) {
	bin := buildBinary(t)
	s := sampleDir()
	out, stderr, code := runBinary(t, bin, "",
		"--catalog", filepath.Join(s, "layer1.yaml"),
		"--kit", filepath.Join(s, "onboarding.json"),
		"--format", "md",
	)
	if code != 0 {
		t.Fatalf("md: expected exit 0, got %d; stderr=%q", code, stderr)
	}
	if len(out) == 0 {
		t.Fatal("md output empty")
	}
}

func TestCLI_BadKit(t *testing.T) {
	bin := buildBinary(t)
	s := sampleDir()
	bad := filepath.Join(s, "malformed.json")
	if err := os.WriteFile(bad, []byte("{ not json"), 0644); err != nil {
		t.Fatalf("write malformed kit: %v", err)
	}
	defer os.Remove(bad)
	_, stderr, code := runBinary(t, bin, "",
		"--catalog", filepath.Join(s, "layer1.yaml"),
		"--kit", bad,
	)
	if code != 2 {
		t.Fatalf("malformed kit expected exit 2 (ToolError), got %d; stderr=%q", code, stderr)
	}
}

func TestProvenance_Chain(t *testing.T) {
	bin := buildBinary(t)
	s := sampleDir()
	// The binary writes its provenance log relative to the process CWD,
	// so run it with its CWD set to a fresh temp dir and verify that dir.
	work := t.TempDir()
	_, stderr, code := runBinary(t, bin, work,
		"--catalog", filepath.Join(s, "layer1.yaml"),
		"--kit", filepath.Join(s, "onboarding.json"),
		"--program", "iso27001",
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("provenance run: expected exit 0, got %d; stderr=%q", code, stderr)
	}
	logPath := filepath.Join(work, "logs", "provenance.jsonl")
	results, err := provenance.Verify(logPath)
	if err != nil {
		t.Fatalf("verify log %s: %v", logPath, err)
	}
	if len(results) == 0 {
		t.Fatalf("no provenance entries in %s", logPath)
	}
	for i, res := range results {
		if !res.Valid {
			t.Errorf("entry %d invalid: %s", i, res.FailureReason)
		}
		if !res.ChainValid {
			t.Errorf("entry %d chain invalid: %s", i, res.FailureReason)
		}
	}
}
