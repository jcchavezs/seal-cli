package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// pinBulkSetup writes a project with two discovery-matched bundles (foo and
// bar) pinned in seal.json, all unchanged on disk. Lets subsequent test cases
// mutate the tree to simulate new/modified/ removed scenarios.
func pinBulkSetup(t *testing.T) string {
	t.Helper()
	cwd := t.TempDir()

	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/foo/SKILL.md": "foo-original",
		"skills/bar/SKILL.md": "bar-original",
	})

	// Hash both bundles and write a matching seal.json with a discovery glob so
	// bulk mode actually walks something.
	fooFiles, fooCh, err := seal.HashSealedRoot(filepath.Join(cwd, "skills/foo"))
	if err != nil {
		t.Fatal(err)
	}
	barFiles, barCh, err := seal.HashSealedRoot(filepath.Join(cwd, "skills/bar"))
	if err != nil {
		t.Fatal(err)
	}

	lf := &seal.Lockfile{
		Version:   1,
		Policy:    "block",
		Discovery: []string{"skills/*"},
		Bundles: map[string]seal.Bundle{
			"./skills/foo": {ContentHash: fooCh, Files: fooFiles},
			"./skills/bar": {ContentHash: barCh, Files: barFiles},
		},
	}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatal(err)
	}
	return cwd
}

// TestRunPin_BulkNoOp covers the all-unchanged case: empty Args ⇒ bulk mode,
// every discovered + lockfile-recorded bundle is unchanged ⇒ "No changes" +
// exit 0 + lockfile untouched. Same idempotency contract as targeted no-op.
func TestRunPin_BulkNoOp(t *testing.T) {
	cwd := pinBulkSetup(t)

	before, err := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		// No Args → bulk mode. No Stdin → if we accidentally prompt, this test
		// hangs (we won't).
	})

	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "No changes") {
		t.Errorf("missing 'No changes':\n%s", stderr.String())
	}
	after, err := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("seal.json rewritten despite no-op bulk pin")
	}
}

// TestRunPin_BulkAddsNewAndUpdatesModified: a new dir.
// skills/* and a modified file inside an existing bundle should both be
// picked up in a single bulk run.
func TestRunPin_BulkAddsNewAndUpdatesModified(t *testing.T) {
	cwd := pinBulkSetup(t)

	// New dir matched by discovery glob.
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/baz/SKILL.md": "baz-new",
	})
	// Modify foo's file so it drifts.
	if err := os.WriteFile(
		filepath.Join(cwd, "skills/foo/SKILL.md"),
		[]byte("foo-MODIFIED"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
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
	// New bundle should have a real ContentHash.
	if _, ok := lf.Bundles["./skills/baz"]; !ok {
		t.Errorf("./skills/baz missing from lockfile; have %v", bundleKeys(lf))
	}
	// foo should be re-pinned: a second bulk pin must be no-op.
	var stderr2 bytes.Buffer
	code = RunPin(PinOpts{Cwd: cwd, Stderr: &stderr2})
	if code != 0 || !strings.Contains(stderr2.String(), "No changes") {
		t.Errorf("second bulk pin not no-op: exit %d, stderr:\n%s",
			code, stderr2.String())
	}
}

func TestRunPin_BulkAddsNewFileRoot(t *testing.T) {
	cwd := pinBulkSetup(t)
	if err := os.WriteFile(filepath.Join(cwd, "skills", "m3.md"), []byte("file-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
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

// TestRunPin_BulkRemovedWithoutPrune: a bundle whose dir has been deleted
// from disk SHOULD be reported in the summary but MUST NOT be dropped from
// the lockfile unless --prune is passed ( 377). With no --prune AND no other
// changes, the operation is a no-op write-wise - but the user still sees the
// report.
func TestRunPin_BulkRemovedWithoutPrune(t *testing.T) {
	cwd := pinBulkSetup(t)

	// Delete bar's directory entirely.
	if err := os.RemoveAll(filepath.Join(cwd, "skills/bar")); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(filepath.Join(cwd, "seal.json"))

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		// No Stdin → no prompt expected when the only change is a
		// reportable-but-not-applied removal.
	})

	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}
	// Lockfile must still contain bar - without --prune.
	after, err := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("seal.json modified despite missing --prune")
	}
	lf, _ := seal.ReadFile(filepath.Join(cwd, "seal.json"))
	if _, ok := lf.Bundles["./skills/bar"]; !ok {
		t.Error("./skills/bar dropped from lockfile without --prune")
	}
}

// TestRunPin_BulkRemovedWithPrune: same scenario, but --prune is.
// ⇒ the removed bundle gets dropped from the lockfile after the confirm
// prompt.
func TestRunPin_BulkRemovedWithPrune(t *testing.T) {
	cwd := pinBulkSetup(t)
	if err := os.RemoveAll(filepath.Join(cwd, "skills/bar")); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdin:  strings.NewReader("y\n"),
		Prune:  true,
	})
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}

	lf, err := seal.ReadFile(filepath.Join(cwd, seal.LockfileName))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lf.Bundles["./skills/bar"]; ok {
		t.Errorf("./skills/bar should have been pruned; remaining keys %v",
			bundleKeys(lf))
	}
}

// TestRunPin_BulkDecline.
// changes applied, "pin aborted" on stderr. Same contract as targeted
// decline.
func TestRunPin_BulkDecline(t *testing.T) {
	cwd := pinBulkSetup(t)
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/baz/SKILL.md": "new",
	})

	before, _ := os.ReadFile(filepath.Join(cwd, "seal.json"))

	var stderr bytes.Buffer
	code := RunPin(PinOpts{
		Cwd:    cwd,
		Stderr: &stderr,
		Stdin:  strings.NewReader("n\n"),
	})
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "pin aborted") {
		t.Errorf("missing 'pin aborted':\n%s", stderr.String())
	}
	after, _ := os.ReadFile(filepath.Join(cwd, "seal.json"))
	if string(before) != string(after) {
		t.Error("seal.json modified despite declined pin")
	}
}
