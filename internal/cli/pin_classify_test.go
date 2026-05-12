package cli

import (
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// TestClassifyPinTarget pins the per-bundle pin-classification truth table.
// Pure logic: given a bundle key, the lockfile's existing entry for it (or
// zero-value if absent), and the freshly-computed on-disk files map, decide
// if this target is new / unchanged / modified.
//
// The "removed" pinKind is intentionally NOT covered here - it applies only
// in bulk mode, where the lockfile has an entry but disk has no dir. That
// case is handled separately by the bulk orchestrator since it doesn't have
// an on-disk files map to feed this function.
func TestClassifyPinTarget(t *testing.T) {
	// Helper to build a Bundle from a files map so the cases stay readable;
	// ContentHash is irrelevant here (Classify works off the per-file map, not
	// the aggregate).
	mkBundle := func(files map[string]string) seal.Bundle {
		return seal.Bundle{Files: files}
	}

	cases := []struct {
		name       string
		existing   seal.Bundle // zero-value if "not in lockfile"
		hasLockEnt bool
		onDisk     map[string]string
		want       pinKind
	}{
		// 1. No entry in lockfile + something on disk ⇒ new.
		{
			name:       "no lockfile entry ⇒ new",
			hasLockEnt: false,
			onDisk:     map[string]string{"a.md": "sha256:1"},
			want:       pinNew,
		},
		// 2. Exact match between lockfile and disk ⇒ unchanged.
		{
			name:       "byte-identical maps ⇒ unchanged",
			existing:   mkBundle(map[string]string{"a.md": "sha256:1"}),
			hasLockEnt: true,
			onDisk:     map[string]string{"a.md": "sha256:1"},
			want:       pinUnchanged,
		},
		// 3. Same keys, different hash ⇒ modified.
		{
			name:       "same key, drifted hash ⇒ modified",
			existing:   mkBundle(map[string]string{"a.md": "sha256:1"}),
			hasLockEnt: true,
			onDisk:     map[string]string{"a.md": "sha256:2"},
			want:       pinModified,
		},
		// 4. Extra file on disk ⇒ modified.
		{
			name:       "extra file on disk ⇒ modified",
			existing:   mkBundle(map[string]string{"a.md": "sha256:1"}),
			hasLockEnt: true,
			onDisk:     map[string]string{"a.md": "sha256:1", "b.md": "sha256:9"},
			want:       pinModified,
		},
		// 5. Lockfile has a file that's gone from disk ⇒ modified.
		// Different from bulk "removed" (which is whole-dir gone);
		// here the dir is present but a file is missing.
		{
			name:       "lockfile-recorded file missing ⇒ modified",
			existing:   mkBundle(map[string]string{"a.md": "sha256:1", "b.md": "sha256:2"}),
			hasLockEnt: true,
			onDisk:     map[string]string{"a.md": "sha256:1"},
			want:       pinModified,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyPinTarget(c.existing, c.hasLockEnt, c.onDisk)
			if got != c.want {
				t.Fatalf("classifyPinTarget(...) = %v, want %v", got, c.want)
			}
		})
	}
}

// TestPinKindString pins the user-visible labels for each pinKind.
// Each label appears in the prompt summary; CI logs and bug reports will
// quote them, so a silent rename is a real regression.
func TestPinKindString(t *testing.T) {
	cases := []struct {
		k    pinKind
		want string
	}{
		{pinNew, "new"},
		{pinUnchanged, "unchanged"},
		{pinModified, "modified"},
		{pinRemoved, "removed"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("pinKind(%d).String() = %q, want %q", c.k, got, c.want)
		}
	}
}