package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// TestWriteInitSummary_PatternsAndBundles is the central case: the user has
// matching agent layouts on disk, init has hashed them, and we now show the
// proposal. Every pattern + bundle must appear by name with a "+" marker so
// the user can scan what `init` is about to apply.
func TestWriteInitSummary_PatternsAndBundles(t *testing.T) {
	lf := &seal.Lockfile{
		Discovery: []string{".claude/skills/*"},
		Bundles: map[string]seal.Bundle{
			"./.claude/skills/foo": {
				Files: map[string]string{
					"SKILL.md": "sha256:abc",
					"a/b.md":   "sha256:def",
				},
			},
		},
	}

	var buf bytes.Buffer
	writeInitSummary(&buf, lf, false)
	out := buf.String()

	// Discovery section
	if !strings.Contains(out, "Discovery patterns to add:") {
		t.Errorf("missing Discovery header:\n%s", out)
	}
	if !strings.Contains(out, "+ .claude/skills/*") {
		t.Errorf("missing pattern line:\n%s", out)
	}

	// Bundles section - header includes count, each line shows the file count so
	// users see at a glance "is this the right scope?"
	if !strings.Contains(out, "Bundles to add (1):") {
		t.Errorf("missing Bundles header with count:\n%s", out)
	}
	if !strings.Contains(out, "+ ./.claude/skills/foo  (2 files)") {
		t.Errorf("missing bundle line with file count:\n%s", out)
	}

	// Non-verbose: per-file paths must NOT leak. Real projects can have hundreds
	// of files; printing them by default would drown the prompt.
	if strings.Contains(out, "SKILL.md") {
		t.Errorf("non-verbose summary should not list filenames:\n%s", out)
	}
}

// TestWriteInitSummary_Verbose verifies the --verbose path: every file under
// every bundle is listed. The user can audit exactly what will be pinned
// before saying yes.
func TestWriteInitSummary_Verbose(t *testing.T) {
	lf := &seal.Lockfile{
		Bundles: map[string]seal.Bundle{
			"./bundle": {
				Files: map[string]string{
					"alpha.md": "sha256:1",
					"beta.md":  "sha256:2",
				},
			},
		},
	}

	var buf bytes.Buffer
	writeInitSummary(&buf, lf, true)
	out := buf.String()

	for _, want := range []string{"alpha.md", "beta.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose summary missing file %q:\n%s", want, out)
		}
	}
}

// TestWriteInitSummary_EmptyShowsNone covers the no-heuristic-match case: the
// summary still renders, with explicit "(none)" markers so the user
// understands the prompt that follows ("create empty seal.json?") is not a UI
// bug.
func TestWriteInitSummary_EmptyShowsNone(t *testing.T) {
	lf := &seal.Lockfile{}
	var buf bytes.Buffer
	writeInitSummary(&buf, lf, false)
	out := buf.String()

	if !strings.Contains(out, "(none)") {
		t.Errorf("empty Discovery should print '(none)':\n%s", out)
	}
	// 0-bundle header is informative - the user sees explicitly that init found
	// nothing to pin.
	if !strings.Contains(out, "Bundles to add (0):") {
		t.Errorf("empty bundles should still print count 0:\n%s", out)
	}
}