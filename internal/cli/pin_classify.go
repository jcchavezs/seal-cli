package cli

import "github.com/SamyGhannad/seal-cli/internal/seal"

// pinKind is the per-target classification `seal pin` produces - input to
// the summary printer and the "any changes pending?" gate on the [y/N]
// prompt.
//
// Separate from seal.Status (verify's enum) because pin and verify ask
// different questions: verify is "does the lockfile still tell the truth?",
// pin is "what would change if I wrote a fresh lockfile?" The vocabularies
// overlap conceptually but the user-visible words differ (Mismatch vs.
// modified). Keeping them split prevents accidental cross-pollination.
type pinKind int

const (
	// pinUnset is the zero value. classifyPinTarget never returns it; if it
	// ever shows up in output that's a bug.
	pinUnset pinKind = iota

	// pinNew: not in the lockfile yet; writing would ADD this bundle.
	pinNew

	// pinUnchanged: every on-disk hash matches what's recorded.
	pinUnchanged

	// pinModified: in the lockfile, but at least one hash differs or a file
	// was added/removed.
	pinModified

	// pinRemoved (bulk only): recorded in the lockfile but the directory has
	// vanished from disk. --prune decides whether the entry is dropped.
	pinRemoved
)

// String returns the user-visible label. These strings appear in pin's
// stderr summary; renaming any of them is a user-visible breaking change
// (CI logs, screenshots, bug reports all quote them). pinUnset renders as
// "unset" so a stray unclassified target looks obviously wrong.
func (k pinKind) String() string {
	switch k {
	case pinUnset:
		return "unset"
	case pinNew:
		return "new"
	case pinUnchanged:
		return "unchanged"
	case pinModified:
		return "modified"
	case pinRemoved:
		return "removed"
	}
	return "unknown"
}

// classifyPinTarget decides whether a single bundle would be added, left
// alone, or replaced. Pure function, no I/O.
//
// hasLockEnt distinguishes "no entry exists for this key" from "entry exists
// but is the zero value" (Validate forbids the latter, but defence in depth).
// onDisk is non-nil - we only call this when the dir is present.
//
// Removed is out of scope here: it requires the bulk orchestrator's
// set-difference view of which keys didn't show up in rediscovery.
func classifyPinTarget(existing seal.Bundle, hasLockEnt bool, onDisk map[string]string) pinKind {
	if !hasLockEnt {
		return pinNew
	}
	if equalFileMaps(existing.Files, onDisk) {
		return pinUnchanged
	}
	// Any delta (different hash, added file, missing file) is "modified" -
	// the summary writer uses diffFileMaps later to render the specifics.
	return pinModified
}

// equalFileMaps is the same byte-for-byte equality classify.go uses.
// Duplicated here rather than exported from seal so classify's contract
// stays narrow.
func equalFileMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok || vb != va {
			return false
		}
	}
	return true
}