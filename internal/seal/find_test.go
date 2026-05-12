package seal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestFindLockfile_Present verifies the function returns the seal.json path
// when present in the supplied working directory.
func TestFindLockfile_Present(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, LockfileName)
	if err := os.WriteFile(want, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindLockfile(dir)
	if err != nil {
		t.Fatalf("FindLockfile: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestFindLockfile_Missing verifies the canonical not-found error is returned
// (as fs.ErrNotExist) so the CLI can render its standard "run init first"
// hint via errors.Is.
//
// Using errors.Is rather than the legacy os.IsNotExist is intentional:
// errors.Is correctly unwraps wrapped errors, which os.IsNotExist does not.
// Same idiom as ReadFile's contract.
func TestFindLockfile_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := FindLockfile(dir)
	if err == nil {
		t.Fatalf("expected error for missing seal.json, got nil")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected errors.Is(err, fs.ErrNotExist), got %v", err)
	}
}

// TestFindLockfile_NoTraversal verifies
// NOT traverse parent directories to locate seal.json. We place the lockfile
// in a parent directory and confirm FindLockfile in a child directory returns
// not-found, NOT the parent's path.
//
// Why this matters: without this rule, running `seal verify` from a random
// subdirectory inside a monorepo could accidentally validate against the
// monorepo's root lockfile, masking missing per-project lockfiles.
func TestFindLockfile_NoTraversal(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "sub")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	// Place seal.json in the parent only - child has nothing.
	if err := os.WriteFile(filepath.Join(parent, LockfileName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := FindLockfile(child)
	if err == nil {
		t.Fatalf("expected not-found error in child dir; got nil (parent traversal happened)")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected errors.Is(err, fs.ErrNotExist), got %v", err)
	}
}