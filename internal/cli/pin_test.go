package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// pinTargetedSetup writes a fixture project containing one bundle and a
// matching seal.json so a freshly-built targeted pin against that bundle
// starts in the "all unchanged" state. Returns the cwd.
// Caller can then modify the bundle to force a re-pin or pass a new path to
// add a new bundle.
func pinTargetedSetup(t *testing.T) string {
	t.Helper()
	cwd := t.TempDir()

	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/foo/SKILL.md": "original",
	})

	files, ch, err := seal.HashSealedRoot(filepath.Join(cwd, "skills/foo"))
	if err != nil {
		t.Fatal(err)
	}
	lf := &seal.Lockfile{
		Version:   1,
		Policy:    "block",
		Discovery: []string{"skills/*"},
		Bundles: map[string]seal.Bundle{
			"./skills/foo": {ContentHash: ch, Files: files},
		},
	}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatal(err)
	}
	return cwd
}

// TestRunPin_TargetedNoOp covers the "every target unchanged" path:
// , pin MUST exit 0 with "No changes" on stderr and
// MUST NOT prompt or touch seal.json. This is the load-bearing idempotency
// contract - running `seal pin <path>` after no actual changes should be a
// no-op, period.
func TestRunPin_TargetedNoOp(t *testing.T) {
	cwd := pinTargetedSetup(t)

	// Read the lockfile bytes before so we can prove byte-equality after - any
	// rewrite (even of identical content) would produce a different mtime and a
	// non-zero diff.
	before, err := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills/foo"},
		Stderr: &stderr,
		// Stdin intentionally nil - if pin tries to prompt this test would hang.
	})

	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "No changes") {
		t.Errorf("missing 'No changes' in:\n%s", stderr.String())
	}
	after, err := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("seal.json was rewritten despite no-op pin")
	}
}

// TestRunPin_TargetedAddsNewBundle: user pins a brand-new directory.
// On "y" the lockfile gains a new entry; on success we should print a
// confirmation summarising what landed.
func TestRunPin_TargetedAddsNewBundle(t *testing.T) {
	cwd := pinTargetedSetup(t)

	// Add a second bundle on disk that the lockfile doesn't know about yet.
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/bar/SKILL.md": "newer",
	})

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills/bar"},
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}

	// Round-trip the lockfile to verify the new bundle landed with a real hash +
	// files map, not as a stub.
	lf, err := seal.ReadFile(filepath.Join(cwd, seal.LockfileName))
	if err != nil {
		t.Fatal(err)
	}
	b, ok := lf.Bundles["./skills/bar"]
	if !ok {
		t.Fatalf("./skills/bar missing from lockfile; have %v", bundleKeys(lf))
	}
	if b.ContentHash == "" {
		t.Errorf("new bundle ContentHash empty")
	}
	if _, ok := b.Files["SKILL.md"]; !ok {
		t.Errorf("new bundle missing SKILL.md: %v", b.Files)
	}
}

func TestRunPin_TargetedRejectsInsideExistingRoot(t *testing.T) {
	cwd := pinTargetedSetup(t)
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/foo/bar/SKILL.md": "nested\n",
	})

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills/foo/bar"},
		Stderr: &stderr,
	})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "inside existing sealed root") {
		t.Fatalf("stderr should reject nested pin target:\n%s", stderr.String())
	}
}

func TestRunPin_TargetedRejectsParentWhenChildExists(t *testing.T) {
	cwd := pinTargetedSetup(t)

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills"},
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "inside target") {
		t.Fatalf("stderr should reject parent pin when child exists:\n%s", stderr.String())
	}
}

func TestRunPin_TargetedAddsDiscoveryCoverage(t *testing.T) {
	cwd := pinTargetedSetup(t)
	writeTreeForOnDisk(t, cwd, map[string]string{
		"docs/prompt.md": "prompt\n",
	})

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"docs/prompt.md"},
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}

	lf, err := seal.ReadFile(filepath.Join(cwd, seal.LockfileName))
	if err != nil {
		t.Fatal(err)
	}
	if !seal.BundleKeyCoveredByDiscovery(lf.Discovery, "./docs/prompt.md") {
		t.Fatalf("discovery %v does not cover ./docs/prompt.md", lf.Discovery)
	}
}

func TestRunPin_TargetedAddsNewFileRoot(t *testing.T) {
	cwd := pinTargetedSetup(t)
	if err := os.MkdirAll(filepath.Join(cwd, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "skills", "m3.md"), []byte("file-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills/m3.md"},
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}

	lf, err := seal.ReadFile(filepath.Join(cwd, seal.LockfileName))
	if err != nil {
		t.Fatal(err)
	}
	b, ok := lf.Bundles["./skills/m3.md"]
	if !ok {
		t.Fatalf("./skills/m3.md missing from lockfile; have %v", bundleKeys(lf))
	}
	if _, ok := b.Files["."]; !ok {
		t.Errorf("file-root bundle missing . file entry: %v", b.Files)
	}
}

// TestRunPin_TargetedUpdatesModified: user pins an existing bundle whose
// contents drifted on disk. On "y" the lockfile entry's hash + files map is
// replaced with the fresh values.
func TestRunPin_TargetedUpdatesModified(t *testing.T) {
	cwd := pinTargetedSetup(t)

	// Mutate the tracked file so the on-disk hash drifts.
	if err := os.WriteFile(
		filepath.Join(cwd, "skills/foo/SKILL.md"),
		[]byte("MODIFIED CONTENT"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills/foo"},
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}

	// Verify by running pin AGAIN with no change - should be a no-op now. This
	// proves the lockfile was actually re-pinned to match disk, not just
	// successfully written with stale data.
	var stderr2 bytes.Buffer
	code = RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills/foo"},
		Stderr: &stderr2,
		// No Stdin - must be a no-op so we never prompt.
	})
	if code != 0 {
		t.Fatalf("second pin not no-op: exit %d, stderr:\n%s", code, stderr2.String())
	}
	if !strings.Contains(stderr2.String(), "No changes") {
		t.Errorf("expected second pin to be no-op; stderr:\n%s", stderr2.String())
	}
}

// TestRunPin_TargetedDecline.
// no changes written, "pin aborted" on stderr.
func TestRunPin_TargetedDecline(t *testing.T) {
	cwd := pinTargetedSetup(t)
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/bar/SKILL.md": "newer",
	})

	before, err := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills/bar"},
		Stderr: &stderr,
		Stdin:  strings.NewReader("n\n"),
	})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "pin aborted") {
		t.Errorf("missing 'pin aborted':\n%s", stderr.String())
	}
	// Lockfile bytes must be untouched.
	after, err := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("seal.json should NOT change on declined pin")
	}
}

// TestRunPin_TargetedMissingLockfile.
// seal.json to exist. Without it: exit 2, instructive stderr.
func TestRunPin_TargetedMissingLockfile(t *testing.T) {
	cwd := t.TempDir() // no seal.json
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/foo/SKILL.md": "x",
	})

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"skills/foo"},
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "seal.json") {
		t.Errorf("stderr should mention seal.json:\n%s", stderr.String())
	}
}

// TestRunPin_TargetedBadPath: a path that doesn't exist (or that fails the
// exit 2, and NOT touch seal.json.
func TestRunPin_TargetedBadPath(t *testing.T) {
	cwd := pinTargetedSetup(t)
	before, _ := os.ReadFile(filepath.Join(cwd, "seal.json"))

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Args:   []string{"nope-does-not-exist"},
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
	})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	after, _ := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if string(before) != string(after) {
		t.Error("seal.json changed despite path-resolution error")
	}
}
