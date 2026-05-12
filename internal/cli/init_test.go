package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// TestRunInit_HappyPath: a project with a recognised Claude skills layout,
// user says "y" → seal.json gets written, exit 0, the success line names the
// bundle/pattern counts so the user can sanity- check what landed without
// opening the file.
func TestRunInit_HappyPath(t *testing.T) {
	cwd := t.TempDir()
	writeTreeForOnDisk(t, cwd, map[string]string{
		".claude/skills/foo/SKILL.md": "x",
	})

	var stderr bytes.Buffer
	code := RunInit(InitOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})

	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}
	// The file must exist on disk.
	if _, err := os.Stat(filepath.Join(cwd, "seal.json")); err != nil {
		t.Fatalf("seal.json not created: %v", err)
	}
	// And the file must be a valid seal lockfile - round-trip through ReadFile
	// to prove the bytes are well-formed and pass Validate.
	if _, err := seal.ReadFile(filepath.Join(cwd, "seal.json")); err != nil {
		t.Fatalf("written seal.json failed ReadFile: %v", err)
	}
	// Success summary is the user's confirmation that something actually
	// happened - pin it.
	if !strings.Contains(stderr.String(), "Wrote seal.json") {
		t.Errorf("missing success message:\n%s", stderr.String())
	}
}

// TestRunInit_AlreadyExists.
// overwrite an existing seal.json. Exit 2 + stderr message + original file
// untouched.
func TestRunInit_AlreadyExists(t *testing.T) {
	cwd := t.TempDir()
	existing := []byte(`{"existing":"untouched"}`)
	lockPath := filepath.Join(cwd, "seal.json")
	if err := os.WriteFile(lockPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunInit(InitOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})

	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	// The error must mention the file by name so the user knows what's blocking
	// them.
	if !strings.Contains(stderr.String(), "seal.json") {
		t.Errorf("stderr should mention seal.json:\n%s", stderr.String())
	}
	// Critically: the original contents must be byte-identical.
	got, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(existing) {
		t.Errorf("init clobbered existing file: got %q, want %q", got, existing)
	}
}

// TestRunInit_Decline: user types "n" at the prompt → exit 2, no file
// written, stderr says "init aborted" so the user knows their choice was
// honored (silent exit would feel like a crash).
func TestRunInit_Decline(t *testing.T) {
	cwd := t.TempDir()
	writeTreeForOnDisk(t, cwd, map[string]string{
		".claude/skills/foo/SKILL.md": "x",
	})

	var stderr bytes.Buffer
	code := RunInit(InitOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdin:  strings.NewReader("n\n"),
	})

	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if _, err := os.Stat(filepath.Join(cwd, "seal.json")); !os.IsNotExist(err) {
		t.Errorf("seal.json should NOT exist after decline; stat err = %v", err)
	}
	if !strings.Contains(stderr.String(), "init aborted") {
		t.Errorf("stderr should say 'init aborted':\n%s", stderr.String())
	}
}

// TestRunInit_EmptyProjectUsesFallbackPrompt: no known layouts.
// disk → init still runs, but the prompt MUST be the explicit fallback text.
// We assert the prompt substring on stderr to pin that specific UX.
func TestRunInit_EmptyProjectUsesFallbackPrompt(t *testing.T) {
	cwd := t.TempDir() // no .claude / .codex / .agents

	var stderr bytes.Buffer
	code := RunInit(InitOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})

	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}
	want := "No agent bundles or known agent layouts detected in this project. Create empty seal.json with policy: block?"
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("missing fallback prompt:\nwant substring: %q\ngot:\n%s", want, stderr.String())
	}
}

// TestRunInit_WarnFlagSetsPolicy: --warn changes the policy field in the
// written lockfile from "block" to "warn". This is the only thing the flag
// does, so the test is small but worth pinning.
func TestRunInit_WarnFlagSetsPolicy(t *testing.T) {
	cwd := t.TempDir()
	writeTreeForOnDisk(t, cwd, map[string]string{
		".claude/skills/foo/SKILL.md": "x",
	})

	code := RunInit(InitOpts{
		Cwd:    cwd,
		Stderr: &bytes.Buffer{},
		Stdin:  strings.NewReader("y\n"),
		Warn:   true,
	})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	lf, err := seal.ReadFile(filepath.Join(cwd, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if lf.Policy != "warn" {
		t.Errorf("Policy = %q, want %q", lf.Policy, "warn")
	}
}