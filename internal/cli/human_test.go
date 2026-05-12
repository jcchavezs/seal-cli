package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// TestStatusLabel pins the human-output label for every per-bundle status. We
// pad to a fixed width (10) so multiple bundles line up visually in the
// summary block, no matter their status. Reviewers scan such blocks
// vertically; uneven left-edges hide drift.
func TestStatusLabel(t *testing.T) {
	cases := []struct {
		s    seal.Status
		want string
	}{
		{seal.Verified, "Verified  "},
		{seal.Unverified, "Unverified"},
		{seal.Removed, "Removed   "},
		{seal.Mismatch, "Mismatch  "},
	}
	for _, c := range cases {
		got := statusLabel(c.s)
		if got != c.want {
			t.Errorf("statusLabel(%v) = %q, want %q", c.s, got, c.want)
		}
		// Width invariant: every label must be exactly 10 columns wide, so column
		// alignment is preserved across all four.
		if len(got) != 10 {
			t.Errorf("statusLabel(%v) width %d, want 10 (label %q)",
				c.s, len(got), got)
		}
	}
}

// TestWriteVerifyHuman_HappyPath verifies the structure of the human summary
// block: a tally line, no per-bundle lines when everything is verified, and a
// result line.
func TestWriteVerifyHuman_HappyPath(t *testing.T) {
	states := []seal.State{
		{Key: "./a", Status: seal.Verified},
		{Key: "./b", Status: seal.Verified},
	}
	var buf bytes.Buffer
	writeVerifyHuman(&buf, "Verified", "block", states, false)

	out := buf.String()
	// Tally line is the headline; CI greps for this counts.
	if !strings.Contains(out, "2 verified, 0 unverified, 0 removed, 0 mismatch") {
		t.Errorf("missing tally line in:\n%s", out)
	}
	// Result trailer encodes both the outcome and the policy mode so a reader
	// can tell "Verified (block mode)" apart.
	// "Verified (warn mode)" - the latter is still surprising.
	if !strings.Contains(out, "Result: Verified (block mode)") {
		t.Errorf("missing result line in:\n%s", out)
	}
	// No bundle should be enumerated individually when all are
	// Verified - the summary is the whole story.
	if strings.Contains(out, "./a") || strings.Contains(out, "./b") {
		t.Errorf("verified bundles should not be listed individually:\n%s", out)
	}
}

// TestWriteVerifyHuman_NonVerifiedListed checks the rendering when drift is
// present: every NON-verified bundle gets its own line (labelled by status),
// verified bundles stay summarised.
func TestWriteVerifyHuman_NonVerifiedListed(t *testing.T) {
	states := []seal.State{
		{Key: "./good", Status: seal.Verified},
		{Key: "./gone", Status: seal.Removed},
		{Key: "./new", Status: seal.Unverified},
	}
	var buf bytes.Buffer
	writeVerifyHuman(&buf, "Blocked", "block", states, false)

	out := buf.String()
	// The two drift bundles must appear by name.
	for _, key := range []string{"./gone", "./new"} {
		if !strings.Contains(out, key) {
			t.Errorf("non-verified bundle %q missing from output:\n%s", key, out)
		}
	}
	// The verified bundle should still NOT be listed.
	if strings.Contains(out, "./good") {
		t.Errorf("verified bundle should be summarised, not listed:\n%s", out)
	}
}

// TestWriteVerifyHuman_VerboseMismatch verifies the per-file detail block
// under --verbose for a Mismatch bundle. Each of modified, added, missing
// should appear as its own indented line - that's the information a user
// needs to debug "why is verify failing".
//
// Non-verbose runs must NOT print these details (it's noisy for the common
// case).
func TestWriteVerifyHuman_VerboseMismatch(t *testing.T) {
	states := []seal.State{
		{
			Key:      "./drifted",
			Status:   seal.Mismatch,
			Modified: []string{"a.md"},
			Added:    []string{"b.md"},
			Missing:  []string{"c.md"},
		},
	}

	// Verbose ON: details visible.
	var verbose bytes.Buffer
	writeVerifyHuman(&verbose, "Blocked", "block", states, true)
	vout := verbose.String()
	for _, snippet := range []string{
		"modified: a.md",
		"added:    b.md",
		"missing:  c.md",
	} {
		if !strings.Contains(vout, snippet) {
			t.Errorf("verbose output missing %q in:\n%s", snippet, vout)
		}
	}

	// Verbose OFF: only the bundle line; no per-file details.
	var terse bytes.Buffer
	writeVerifyHuman(&terse, "Blocked", "block", states, false)
	tout := terse.String()
	for _, leak := range []string{"modified:", "added:", "missing:"} {
		if strings.Contains(tout, leak) {
			t.Errorf("non-verbose output leaked %q in:\n%s", leak, tout)
		}
	}
}