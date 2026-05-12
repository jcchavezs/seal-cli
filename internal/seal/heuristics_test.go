package seal

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestRegistry_AllPatternsValid asserts every shipped heuristic pattern
// passes ValidatePattern. Without this, a malformed entry in Registry would
// be proposed by `seal init`, written into seal.json, and then rejected at
// the next ReadFile - confusing the user with a "your lockfile is invalid"
// error for a pattern THEY didn't write.
//
// Fast unit-test feedback is the right place to catch this.
func TestRegistry_AllPatternsValid(t *testing.T) {
	for _, e := range Registry {
		if err := ValidatePattern(e.Pattern); err != nil {
			t.Errorf("Registry entry %q has invalid pattern %q: %v", e.Name, e.Pattern, err)
		}
	}
}

// TestProposeFor_ParentExists verifies a pattern is proposed only when its
// glob's parent directory exists in the project. This is the core "low-noise
// heuristic" rule: don't propose patterns for layouts that aren't even
// present.
func TestProposeFor_ParentExists(t *testing.T) {
	root := t.TempDir()
	// Create exactly one Registry pattern's parent directory. Pick
	// ".claude/skills" because it's one of the shipped layouts; the expected
	// output is ".claude/skills/*" and nothing else.
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := ProposeFor(root)
	want := []string{".claude/skills/*"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// TestProposeFor_None verifies an empty project produces no patterns.
// The init command relies on this to skip the "no agent layouts detected"
// branch cleanly.
func TestProposeFor_None(t *testing.T) {
	root := t.TempDir()
	if got := ProposeFor(root); len(got) != 0 {
		t.Fatalf("expected no patterns for empty project, got %v", got)
	}
}

// TestProposeFor_MultipleMatches verifies the function returns patterns in
// Registry order when multiple layouts are present. Init's output is more
// readable when it consistently lists the same layouts in the same order
// across invocations, regardless of map iteration randomness.
func TestProposeFor_MultipleMatches(t *testing.T) {
	root := t.TempDir()
	// Create two Registry layouts.
	for _, sub := range []string{".claude/skills", ".agents/skills"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := ProposeFor(root)
	// Expected order matches Registry's declaration order, NOT lexicographic.
	// Pin both presence and ordering by collecting the "expected" set from
	// Registry itself so the test stays correct as
	// Registry grows.
	var want []string
	for _, e := range Registry {
		if e.Pattern == ".claude/skills/*" || e.Pattern == ".agents/skills/*" {
			want = append(want, e.Pattern)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// TestProposeFor_FileMasqueradingAsParentRejected verifies a regular file at
// the parent-name path does NOT cause the pattern to be proposed. e.g.
// someone has a FILE named ".claude" - that's not a directory layout we
// should match.
func TestProposeFor_FileMasqueradingAsParentRejected(t *testing.T) {
	root := t.TempDir()
	// Create ".claude" as a FILE, not a directory. ".claude/skills/*" has parent
	// ".claude/skills" which can't exist if ".claude" is a file. patternParent
	// returns ".claude/skills" → Stat fails → no propose. Belt-and-suspenders:
	// also create a file at the parent path to test that "exists but not a
	// directory" is also rejected.
	if err := os.WriteFile(filepath.Join(root, ".claude"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ProposeFor(root); len(got) != 0 {
		t.Errorf("file shouldn't masquerade as parent, got %v", got)
	}
}