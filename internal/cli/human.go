package cli

import (
	"fmt"
	"io"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// writeVerifyHuman renders the human-readable `seal verify` summary to w
// (typically stderr): tally headline, per-bundle drift list, result trailer.
//
// Verified bundles are deliberately suppressed from the drift list - they're
// already in the tally, and including them buries the signal in common runs.
//
// Verbose mode expands each Mismatch bundle into modified/added/missing
// sub-lines; off by default because real mismatches can span hundreds of
// files.
func writeVerifyHuman(w io.Writer, outcome, policy string, states []seal.State, verbose bool) {
	var nVerified, nUnverified, nRemoved, nMismatch int
	for _, s := range states {
		switch s.Status {
		case seal.Verified:
			nVerified++
		case seal.Unverified:
			nUnverified++
		case seal.Removed:
			nRemoved++
		case seal.Mismatch:
			nMismatch++
		}
	}

	fmt.Fprintf(w, "%d verified, %d unverified, %d removed, %d mismatch\n",
		nVerified, nUnverified, nRemoved, nMismatch)

	for _, s := range states {
		if s.Status == seal.Verified {
			continue
		}
		fmt.Fprintf(w, "  %s %s\n", statusLabel(s.Status), s.Key)

		// Verbose detail only fires for Mismatch. Unverified/Removed have no
		// per-file diff to render (nothing recorded to compare, or the whole
		// directory is gone).
		if verbose && s.Status == seal.Mismatch {
			for _, p := range s.Modified {
				fmt.Fprintf(w, "    modified: %s\n", p)
			}
			for _, p := range s.Added {
				// Padding keeps "modified:", "added:", "missing:" filename columns
				// vertically aligned.
				fmt.Fprintf(w, "    added:    %s\n", p)
			}
			for _, p := range s.Missing {
				fmt.Fprintf(w, "    missing:  %s\n", p)
			}
		}
	}

	fmt.Fprintf(w, "Result: %s (%s mode)\n", outcome, policy)
}

// statusLabel renders a Status as a fixed-width (10-column) human label for
// the drift list. Width is set by "Unverified", the longest entry; shorter
// labels are right-padded. TestStatusLabel pins the width so new statuses
// can't silently break alignment.
//
// Deliberately separate from seal.Status.String(), which returns lowercase
// JSON-key forms; the two renderings should evolve independently.
func statusLabel(s seal.Status) string {
	switch s {
	case seal.Verified:
		return "Verified  "
	case seal.Unverified:
		return "Unverified"
	case seal.Removed:
		return "Removed   "
	case seal.Mismatch:
		return "Mismatch  "
	}
	// Fallback for zero value or a future Status that forgets to extend the
	// switch; same width so columns still line up.
	return "?         "
}