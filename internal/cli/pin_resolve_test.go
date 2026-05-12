package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolvePathToBundleKey_HappyPath verifies the normal case:
// user types a relative path that exactly matches what's on disk, we return a
// "./..."-prefixed bundle key with forward slashes.
func TestResolvePathToBundleKey_HappyPath(t *testing.T) {
	cwd := t.TempDir()
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/foo/SKILL.md": "x",
	})

	got, err := resolvePathToBundleKey(cwd, "skills/foo")
	if err != nil {
		t.Fatalf("resolvePathToBundleKey: %v", err)
	}
	if got != "./skills/foo" {
		t.Errorf("got %q, want %q", got, "./skills/foo")
	}
}

// TestResolvePathToBundleKey_AcceptsAbsolute verifies that an absolute path
// inside cwd works the same as a relative one. Users reaching for
// tab-completion typically end up with absolute paths.
func TestResolvePathToBundleKey_AcceptsAbsolute(t *testing.T) {
	cwd := t.TempDir()
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/foo/SKILL.md": "x",
	})

	got, err := resolvePathToBundleKey(cwd, filepath.Join(cwd, "skills/foo"))
	if err != nil {
		t.Fatalf("resolvePathToBundleKey: %v", err)
	}
	if got != "./skills/foo" {
		t.Errorf("got %q, want %q", got, "./skills/foo")
	}
}

// TestResolvePathToBundleKey_OutsideProject pins the inside-project guard: a
// path that escapes cwd via ".." or absolute-elsewhere
// MUST error, never silently retarget. Without this guard, `seal pin
// ../other-repo/foo` would write a bundle key the lockfile can't resolve back
// consistently.
func TestResolvePathToBundleKey_OutsideProject(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir() // sibling tempdir, definitively outside cwd

	_, err := resolvePathToBundleKey(cwd, outside)
	if err == nil {
		t.Fatal("expected error for path outside cwd")
	}
	if !strings.Contains(err.Error(), "outside") {
		t.Errorf("error should mention 'outside': %v", err)
	}
}

// TestResolvePathToBundleKey_RejectsProjectRoot pins the "root is not a
// bundle" rule. Even with a valid input, asking pin to pin the project root
// itself is a usage error - bundles are sub-dirs of the project, not the
// project itself.
func TestResolvePathToBundleKey_RejectsProjectRoot(t *testing.T) {
	cwd := t.TempDir()
	_, err := resolvePathToBundleKey(cwd, cwd)
	if err == nil {
		t.Fatal("expected error for cwd itself")
	}
}

// TestResolvePathToBundleKey_RejectsCaseMismatch is the load-bearing the
// actual on-disk case, we MUST refuse, NOT silently pin.
// the wrong key. We probe whether the filesystem is case-insensitive at
// runtime (macOS APFS, Windows NTFS) - on case-sensitive Linux ext4 the OS
// already rejects the mismatched lookup, but our segment- walk produces the
// same error regardless of filesystem, so the test runs everywhere.
func TestResolvePathToBundleKey_RejectsCaseMismatch(t *testing.T) {
	cwd := t.TempDir()
	writeTreeForOnDisk(t, cwd, map[string]string{
		"Skills/foo/SKILL.md": "x", // note: capital S
	})

	// Ask for lowercase "skills/foo" - user-typed case differs.
	// disk-stored case. On case-insensitive FSes the OS would happily open this;
	// we MUST reject anyway so the bundle key is byte-faithful to what's on
	// disk.
	_, err := resolvePathToBundleKey(cwd, "skills/foo")
	if err == nil {
		t.Fatal("expected error for case mismatch (disk has 'Skills' not 'skills')")
	}
}

// TestResolvePathToBundleKey_NonExistent verifies a clean error when the path
// simply doesn't exist on disk. This is the most common user mistake (typo);
// the message should help them debug.
func TestResolvePathToBundleKey_NonExistent(t *testing.T) {
	cwd := t.TempDir()
	_, err := resolvePathToBundleKey(cwd, "nope")
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

// TestResolvePathToBundleKey_AcceptsRegularFile verifies file sealed roots are
// valid pin targets. Discovery can surface regular files, so targeted pin must
// be able to derive the same bundle key for an explicitly named file.
func TestResolvePathToBundleKey_AcceptsRegularFile(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "prompt.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolvePathToBundleKey(cwd, "prompt.md")
	if err != nil {
		t.Fatalf("resolvePathToBundleKey: %v", err)
	}
	if got != "./prompt.md" {
		t.Errorf("got %q, want %q", got, "./prompt.md")
	}
}

func TestResolvePathToBundleKey_RejectsSymlinkedIntermediate(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cwd, "link")); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}

	_, err := resolvePathToBundleKey(cwd, "link/SKILL.md")
	if err == nil {
		t.Fatal("expected symlinked intermediate path segment to be rejected")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink: %v", err)
	}
}
