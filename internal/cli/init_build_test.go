package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// TestBuildInitLockfile_DetectsAndPins covers the central case: a project
// with a recognised Claude layout (`.claude/skills/<name>`) gets matching
// patterns proposed AND the discovered bundle hashed - i.e. the draft
// lockfile already has real ContentHash + Files values, so writing it would
// be a complete pin.
func TestBuildInitLockfile_DetectsAndPins(t *testing.T) {
	cwd := t.TempDir()
	writeTreeForOnDisk(t, cwd, map[string]string{
		".claude/skills/foo/SKILL.md": "hello",
	})

	lf, err := buildInitLockfile(cwd, "block")
	if err != nil {
		t.Fatalf("buildInitLockfile: %v", err)
	}

	// Version + Policy must be set so the caller doesn't have.
	// fix them up.
	if lf.Version != 1 {
		t.Errorf("Version = %d, want 1", lf.Version)
	}
	if lf.Policy != "block" {
		t.Errorf("Policy = %q, want %q", lf.Policy, "block")
	}

	// Heuristic matched ⇒ the pattern is proposed.
	found := false
	for _, p := range lf.Discovery {
		if p == ".claude/skills/*" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Claude skills pattern in Discovery, got %v", lf.Discovery)
	}

	// The matched dir is hashed into a real Bundle entry - not just proposed as
	// a pattern, actually pinned.
	b, ok := lf.Bundles["./.claude/skills/foo"]
	if !ok {
		t.Fatalf("expected bundle ./.claude/skills/foo; got keys %v", bundleKeys(lf))
	}
	if b.ContentHash == "" {
		t.Errorf("bundle ContentHash should be set")
	}
	if _, ok := b.Files["SKILL.md"]; !ok {
		t.Errorf("bundle Files should include SKILL.md; got %v", b.Files)
	}
}

// TestBuildInitLockfile_EmptyProject covers the "fresh project, no known
// layouts" path: we should still return a valid draft lockfile - empty
// Discovery, empty Bundles, but Version + Policy set - so RunInit can prompt
// the user with the "create empty seal.json?" fallback text rather than
// crashing or returning an error.
func TestBuildInitLockfile_EmptyProject(t *testing.T) {
	cwd := t.TempDir() // No .claude, .codex, .agents directories.

	lf, err := buildInitLockfile(cwd, "warn")
	if err != nil {
		t.Fatalf("buildInitLockfile: %v", err)
	}
	if lf.Version != 1 || lf.Policy != "warn" {
		t.Errorf("Version=%d, Policy=%q; want 1 / warn", lf.Version, lf.Policy)
	}
	if len(lf.Discovery) != 0 {
		t.Errorf("expected empty Discovery; got %v", lf.Discovery)
	}
	if len(lf.Bundles) != 0 {
		t.Errorf("expected empty Bundles; got %v", lf.Bundles)
	}
}

// TestBuildInitLockfile_HeuristicMatchesNothingOnDisk covers the "parent dir
// exists but is empty" subtle case: ProposeFor will add the pattern,
// ExpandPatterns will return zero keys (no matches), and we should still
// surface the pattern in the proposal so the user can see what we tried - but
// no bundles get pinned.
func TestBuildInitLockfile_HeuristicMatchesNothingOnDisk(t *testing.T) {
	cwd := t.TempDir()
	// Create the parent .claude/skills/ but leave it empty. ProposeFor gates on
	// parent-dir existence, so this is enough to trigger the pattern proposal.
	writeTreeForOnDisk(t, cwd, map[string]string{
		// MkdirAll handles the chain; an explicit placeholder file inside an
		// UNRELATED dir keeps the parent under test empty.
		"unrelated/keep.txt": "x",
	})
	// Now explicitly mkdir .claude/skills/ with no contents.
	if err := mkdirEmpty(t, filepath.Join(cwd, ".claude/skills")); err != nil {
		t.Fatal(err)
	}

	lf, err := buildInitLockfile(cwd, "block")
	if err != nil {
		t.Fatalf("buildInitLockfile: %v", err)
	}

	// Pattern proposed because parent exists.
	if len(lf.Discovery) == 0 {
		t.Errorf("expected pattern proposal for empty .claude/skills/")
	}
	// No bundles because expansion found no children.
	if len(lf.Bundles) != 0 {
		t.Errorf("expected zero bundles for empty parent dir; got %v", bundleKeys(lf))
	}
}

// bundleKeys is a small test helper to pretty-print the bundle key set in
// failure messages. Saves typing fmt.Sprintf at every t.Errorf.
func bundleKeys(lf *seal.Lockfile) []string {
	out := make([]string, 0, len(lf.Bundles))
	for k := range lf.Bundles {
		out = append(out, k)
	}
	return out
}

// mkdirEmpty creates dir with 0755 permissions. We use a helper because
// os.MkdirAll already returns nil if dir exists, which matches our "make sure
// this dir is here" intent.
func mkdirEmpty(t *testing.T, dir string) error {
	t.Helper()
	return os.MkdirAll(dir, 0o755)
}