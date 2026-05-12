package seal

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
)

// TestEscapeSpecLiterals_IdentityForSafeChars verifies the helper is a no-op
// on segments with only `*` and ordinary path characters. We test the no-op
// path explicitly so a future refactor can't accidentally over-escape a
// string that didn't need touching.
func TestEscapeSpecLiterals_IdentityForSafeChars(t *testing.T) {
	for _, in := range []string{
		"",
		"foo",
		"*",
		"skills/*",
		"foo-bar.baz",
	} {
		if got := escapeSpecLiterals(in); got != in {
			t.Errorf("escape(%q): want unchanged, got %q", in, got)
		}
	}
}

// TestEscapeSpecLiterals_EscapesSpecMetacharsAsLiteral verifies the three. We
// don't pin down the exact byte-level form because path.Match accepts both
// `\?` and `[?]` as literal-?; the meaningful contract is "the result matches
// the input ONLY as a literal under path.Match," which the next test asserts
// directly.
func TestEscapeSpecLiterals_EscapesSpecMetacharsAsLiteral(t *testing.T) {
	for _, in := range []string{"?", "[", "]", "foo?", "foo[bar]"} {
		got := escapeSpecLiterals(in)
		if got == in {
			t.Errorf("escape(%q): expected modification, got identity", in)
		}
	}
}

// TestEscapeSpecLiterals_PathMatchTreatsResultAsLiteral pins the actual
// contract: the escaped form, fed to path.Match, matches the original string
// as a literal - not as a wildcard pattern. This is what "literal defang"
// means in operational terms.
func TestEscapeSpecLiterals_PathMatchTreatsResultAsLiteral(t *testing.T) {
	for _, c := range []struct {
		in    string // original (literal) name
		other string // a string that would match `in` under glob rules
	}{
		// "?" as a wildcard would match any single char; we want it.
		// match ONLY a literal "?".
		{"foo?", "fooX"},
		// "[abc]" as a wildcard would match any of a/b/c; we want it.
		// match ONLY the literal 5-char string "[abc]".
		{"x[abc]y", "xay"},
	} {
		escaped := escapeSpecLiterals(c.in)

		// Self-match: the escaped pattern must match the original literal.
		ok, err := path.Match(escaped, c.in)
		if err != nil {
			t.Errorf("Match(%q, %q): unexpected error %v", escaped, c.in, err)
			continue
		}
		if !ok {
			t.Errorf("Match(%q, %q): want literal self-match, got false", escaped, c.in)
		}

		// Anti-match: the escaped pattern must NOT match a string that would have
		// matched under wildcard semantics.
		ok, err = path.Match(escaped, c.other)
		if err != nil {
			t.Errorf("Match(%q, %q): unexpected error %v", escaped, c.other, err)
			continue
		}
		if ok {
			t.Errorf("Match(%q, %q): want literal-only, got wildcard match", escaped, c.other)
		}
	}
}

// TestExpandPatterns_Basic verifies the happy path: a "*"-leaf pattern
// returns each non-empty child directory once, sorted, with the ./" prefix.
// The fixture has three children (foo, bar.
// empty); empty contains only ".gitkeep" which is NOT exclusion list, so it
// counts as a tracked file and the directory is returned.
func TestExpandPatterns_Basic(t *testing.T) {
	got, err := ExpandPatterns("testdata/projects/basic", []string{".claude/skills/*"})
	if err != nil {
		t.Fatalf("ExpandPatterns: %v", err)
	}
	want := []string{
		"./.claude/skills/bar",
		"./.claude/skills/empty",
		"./.claude/skills/foo",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// TestExpandPatterns_IncludesRegularFiles verifies discovery does not miss
// project-local loadable artifacts represented as single files. A standalone
// CLI has no agent-native loading context, so a matched regular file must
// become its own sealed root rather than being silently dropped.
func TestExpandPatterns_IncludesRegularFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "skills", "m1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills", "m2"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(dir, "skills", "m1", "SKILL.md"), "m1\n")
	mustWriteFile(t, filepath.Join(dir, "skills", "m2", "SKILL.md"), "m2\n")
	mustWriteFile(t, filepath.Join(dir, "skills", "m3.md"), "m3\n")

	got, err := ExpandPatterns(dir, []string{"skills/*"})
	if err != nil {
		t.Fatalf("ExpandPatterns: %v", err)
	}
	want := []string{
		"./skills/m1",
		"./skills/m2",
		"./skills/m3.md",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// TestExpandPatterns_HiddenStarMatchesDot verifies `*` wildcard matches names
// that begin with `.`. Many shells skip dotfiles by default.
// directories are common in agent-tool conventions (`.claude/`, `.agents/`,
// etc.) and skipping them would surprise users.
func TestExpandPatterns_HiddenStarMatchesDot(t *testing.T) {
	got, err := ExpandPatterns("testdata/projects/hidden", []string{".agents/skills/*"})
	if err != nil {
		t.Fatalf("ExpandPatterns: %v", err)
	}
	want := []string{"./.agents/skills/.hidden"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// TestExpandPatterns_NoParent verifies a pattern whose parent does not exist
// returns an empty slice without error. Discovery patterns are advisory - a
// project may legitimately list patterns that don't yet match anything (e.g.,
// a skills directory that hasn't been created).
func TestExpandPatterns_NoParent(t *testing.T) {
	got, err := ExpandPatterns("testdata/projects/basic", []string{"nope/never/*"})
	if err != nil {
		t.Fatalf("ExpandPatterns: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

// TestExpandPatterns_QuestionLiteral verifies
// LITERAL filename character, not a wildcard. Under shell-style globbing
// "foo?" would match "foo" + any single char (so it would match "foo" itself
// if shells used `?` for "zero or one", or e.g. "fooX");.
//
// Our fixture has "foo" but no "foo?", so the expected result is an empty
// match - proving both that the pattern is accepted (no error)
// AND that path.Match treats the escaped form as a literal.
func TestExpandPatterns_QuestionLiteral(t *testing.T) {
	got, err := ExpandPatterns("testdata/projects/basic", []string{".claude/skills/foo?"})
	if err != nil {
		t.Fatalf("ExpandPatterns: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected ? to be literal (no fixture matches `foo?`), got %v", got)
	}
}
