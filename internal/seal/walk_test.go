package seal

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"testing"
)

// TestWalkRegularFiles_ExcludesGit verifies that .git directories AND bare
// .git files are skipped.
//
// Why this matters: git rewrites .git/* on every operation, so hashing it
// would churn the lockfile on every commit/checkout.
// it so verify is stable against git's internal state changes.
//
// We build the fixture at runtime (rather than committing it) because git
// refuses to track a nested .git/ directory - it would treat the fixture as a
// submodule. Runtime construction also lets each test describe its own
// minimal tree without coupling to a shared fixture.
func TestWalkRegularFiles_ExcludesGit(t *testing.T) {
	dir := t.TempDir()

	// Real tracked files - should appear in the walk output.
	mustWriteFile(t, filepath.Join(dir, "SKILL.md"), "skill\n")
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(dir, "scripts", "run.sh"), "run\n")

	// .git directory with a HEAD file inside - must be skipped wholesale.
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(dir, ".git", "HEAD"), "head\n")

	// Bare .git file (mimics a submodule pointer) - also excluded.
	if err := os.MkdirAll(filepath.Join(dir, "submodule"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(dir, "submodule", ".git"), "gitdir: ../.git\n")
	mustWriteFile(t, filepath.Join(dir, "submodule", "code.go"), "code\n")

	var got []string
	err := WalkRegularFiles(dir, func(rel, _ string) error {
		got = append(got, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkRegularFiles: %v", err)
	}
	// Sort so the test isn't tied to filesystem reporting order. The walk caller
	// (HashSealedRoot) does its own deterministic ordering;
	// the walk itself just visits every file once.
	sort.Strings(got)
	want := []string{"SKILL.md", "scripts/run.sh", "submodule/code.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// mustWriteFile writes content to path or fails the test. Helper exists so
// each fixture-building test reads as setup-then-assert rather than being
// half error-handling.
func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestWalkRegularFiles_IncludesSealJSON verifies seal.json is ordinary bundle
// content when it appears inside a sealed root. Seal v1 does not support the
// project root itself as a sealed root, so there is no circular lockfile
// exclusion here.
func TestWalkRegularFiles_IncludesSealJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "seal.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "vendored")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "seal.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	var got []string
	if err := WalkRegularFiles(dir, func(rel, _ string) error {
		got = append(got, rel)
		return nil
	}); err != nil {
		t.Fatalf("WalkRegularFiles: %v", err)
	}
	sort.Strings(got)
	want := []string{"SKILL.md", "seal.json", "vendored/seal.json"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// TestWalkRegularFiles_RejectsSymlink verifies are not supported in v1 and
// must cause an error rather than being silently followed or dereferenced.
//
// We create the symlink at runtime instead of committing it so the test works
// on Windows CI without git's symlink-handling quirks.
func TestWalkRegularFiles_RejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows requires elevated privileges to create symlinks, and the
		// semantics differ enough from POSIX that this test would be brittle.
		t.Skip("symlink semantics differ on Windows")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a symlink pointing at a real file. The walk must error.
	// the symlink even though the target itself is a perfectly valid regular
	// file - what it points.
	if err := os.Symlink("real.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	err := WalkRegularFiles(dir, func(string, string) error { return nil })
	if err == nil {
		t.Fatalf("expected error on symlink, got nil")
	}
}
