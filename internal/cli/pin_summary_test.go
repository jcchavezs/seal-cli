package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestWritePinSummary_AllKinds verifies the basic shape: one line per changed
// bundle prefixed by an ASCII marker (+ / ~ / -) and a human label, plus an
// unchanged tally on a trailing line. The markers are the same ones diff
// tools use, so the visual maps.
// what reviewers already expect.
func TestWritePinSummary_AllKinds(t *testing.T) {
	targets := []pinTarget{
		{Key: "./skills/new", Kind: pinNew, FileCount: 3},
		{Key: "./skills/mod", Kind: pinModified, FileCount: 5},
		{Key: "./skills/old", Kind: pinRemoved, FileCount: 0},
		{Key: "./skills/same", Kind: pinUnchanged, FileCount: 2},
	}

	var buf bytes.Buffer
	writePinSummary(&buf, targets)
	out := buf.String()

	// Each change kind must surface with its bundle key. The "+" marker for new,
	// "~" for modified, "-" for removed are pinned here so a future refactor
	// can't silently change the prefix.
	for _, want := range []string{
		"+ new       ./skills/new",
		"~ modified  ./skills/mod",
		"- removed   ./skills/old",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing substring %q in:\n%s", want, out)
		}
	}

	// Unchanged bundles MUST NOT appear individually - they'd drown the signal
	// in real projects.
	if strings.Contains(out, "./skills/same") {
		t.Errorf("unchanged bundle leaked into per-line list:\n%s", out)
	}

	// But the unchanged COUNT should be visible so the user knows pin saw the
	// rest of their bundles and decided not to touch them. Without this line, a
	// sparse output reads as "wait, where did everything go?"
	if !strings.Contains(out, "1 unchanged") {
		t.Errorf("missing unchanged tally in:\n%s", out)
	}
}

// TestWritePinSummary_OnlyUnchanged confirms the trivial case renders
// cleanly: zero changes, every bundle accounted for. This is what the
// orchestrator prints BEFORE bailing with "No changes", so it must not panic
// on empty change-lists or leave a hanging "Changes:" header.
func TestWritePinSummary_OnlyUnchanged(t *testing.T) {
	targets := []pinTarget{
		{Key: "./a", Kind: pinUnchanged, FileCount: 1},
		{Key: "./b", Kind: pinUnchanged, FileCount: 1},
	}

	var buf bytes.Buffer
	writePinSummary(&buf, targets)
	out := buf.String()

	if !strings.Contains(out, "2 unchanged") {
		t.Errorf("expected '2 unchanged' tally in:\n%s", out)
	}
	// No "+", "~", or "-" lines should appear.
	for _, marker := range []string{"+ new", "~ modified", "- removed"} {
		if strings.Contains(out, marker) {
			t.Errorf("unexpected change line %q in:\n%s", marker, out)
		}
	}
}

// TestWritePinSummary_SortedByKey verifies determinism: changed bundles
// appear in lexicographic order regardless of the input slice's order. Pin's
// caller may build the list from a map iter (randomized) so we must sort
// here. Stability matters.
// screenshots / bug reports / piped output to file.
func TestWritePinSummary_SortedByKey(t *testing.T) {
	targets := []pinTarget{
		{Key: "./c", Kind: pinNew, FileCount: 1},
		{Key: "./a", Kind: pinNew, FileCount: 1},
		{Key: "./b", Kind: pinNew, FileCount: 1},
	}

	var buf bytes.Buffer
	writePinSummary(&buf, targets)
	out := buf.String()

	// Check positions: ./a must appear before ./b which must appear before ./c.
	posA := strings.Index(out, "./a")
	posB := strings.Index(out, "./b")
	posC := strings.Index(out, "./c")
	if !(posA != -1 && posA < posB && posB < posC) {
		t.Fatalf("expected ./a < ./b < ./c; got positions %d/%d/%d\n%s",
			posA, posB, posC, out)
	}
}
