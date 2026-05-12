package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSealLabsScript_ListAndGenerateRunnableLabs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("seal-labs.sh is a bash teaching script")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not available: %v", err)
	}

	root := findModuleRoot(t)
	script := filepath.Join(root, "scripts", "seal-labs.sh")

	listCmd := exec.Command(bash, script, "--list")
	listCmd.Dir = root
	listOut, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--list failed: %v\n%s", err, listOut)
	}
	for _, want := range []string{
		"happy-path",
		"mismatch-modified-file",
		"mismatch-file-removed-from-lockfile",
		"removed-bundle-drift",
		"invalid-unknown-field",
		"pin-targeted-file-root",
		"invalid-static-coverage",
		"invalid-static-coverage-wrong-pattern",
		"invalid-static-coverage-removed-key",
		"invalid-narrowed-discovery",
		"removed-covered-bundle-drift",
		"discovery-file-sibling-unverified",
		"invalid-nested-bundle-keys",
		"invalid-overlapping-discovery",
		"invalid-overlapping-discovery-wildcard-literal",
		"pin-targeted-adds-discovery",
		"pin-targeted-nested-rejected",
		"discovery-question-is-literal",
		"invalid-discovery-recursive-glob",
		"pin-bulk-removed-prune",
		"verify-quiet-output",
	} {
		if !strings.Contains(string(listOut), want) {
			t.Fatalf("--list output missing %q:\n%s", want, listOut)
		}
	}

	matrixCmd := exec.Command(bash, script, "--matrix")
	matrixCmd.Dir = root
	matrixOut, err := matrixCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--matrix failed: %v\n%s", err, matrixOut)
	}
	for _, want := range []string{
		"verification outcomes",
		"per-bundle states",
		"discovery and hashing",
		"lockfile validity",
		"pin workflow",
	} {
		if !strings.Contains(string(matrixOut), want) {
			t.Fatalf("--matrix output missing category %q:\n%s", want, matrixOut)
		}
	}
	for _, stale := range []string{
		"known-gap-static-coverage",
		"known-gap-narrowed-discovery",
		"known-gap-nested-bundle-keys",
		"known-gap-overlapping-discovery",
		"known-gap-targeted-pin-does-not-add-discovery",
		"known-gap-",
	} {
		if strings.Contains(string(matrixOut), stale) {
			t.Fatalf("--matrix contains stale lab %q after conformance fixes:\n%s", stale, matrixOut)
		}
	}
	if rows := strings.Count(string(matrixOut), "\n| `"); rows < 50 {
		t.Fatalf("--matrix should enumerate broad lab coverage, got %d rows:\n%s", rows, matrixOut)
	}

	labRoot := t.TempDir()
	generateCmd := exec.Command(
		bash,
		script,
		"--root", labRoot,
		"--lab", "happy-path",
		"--lab", "mismatch-modified-file",
		"--lab", "invalid-static-coverage-wrong-pattern",
		"--lab", "invalid-static-coverage-removed-key",
		"--lab", "removed-covered-bundle-drift",
		"--lab", "discovery-file-sibling-unverified",
		"--lab", "invalid-nested-bundle-keys",
		"--lab", "invalid-overlapping-discovery",
		"--lab", "invalid-overlapping-discovery-wildcard-literal",
		"--lab", "pin-targeted-adds-discovery",
		"--lab", "pin-targeted-nested-rejected",
	)
	generateCmd.Dir = root
	generateOut, err := generateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate labs failed: %v\n%s", err, generateOut)
	}
	if _, err := os.Stat(filepath.Join(labRoot, "bin", "seal")); err != nil {
		t.Fatalf("compiled lab binary missing: %v\noutput:\n%s", err, generateOut)
	}

	for _, lab := range []string{
		"happy-path",
		"mismatch-modified-file",
		"invalid-static-coverage-wrong-pattern",
		"invalid-static-coverage-removed-key",
		"removed-covered-bundle-drift",
		"discovery-file-sibling-unverified",
		"invalid-nested-bundle-keys",
		"invalid-overlapping-discovery",
		"invalid-overlapping-discovery-wildcard-literal",
		"pin-targeted-adds-discovery",
		"pin-targeted-nested-rejected",
	} {
		readme, err := os.ReadFile(filepath.Join(labRoot, lab, "README.md"))
		if err != nil {
			t.Fatalf("%s README missing: %v", lab, err)
		}
		if !strings.Contains(string(readme), "Expected") {
			t.Fatalf("%s README should contain expected-result instructions:\n%s", lab, readme)
		}

		demo := exec.Command(bash, filepath.Join(labRoot, lab, "demo.sh"))
		demo.Dir = filepath.Join(labRoot, lab)
		demoOut, err := demo.CombinedOutput()
		if err != nil {
			t.Fatalf("%s demo failed: %v\n%s", lab, err, demoOut)
		}
		if !strings.Contains(string(demoOut), "PASS") {
			t.Fatalf("%s demo did not report PASS:\n%s", lab, demoOut)
		}
	}
}
