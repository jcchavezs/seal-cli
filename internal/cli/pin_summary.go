package cli

import (
	"fmt"
	"io"
	"sort"
)

// pinTarget is one row in the pin summary: a bundle key, what pin would do
// to it, and (for new/modified) the post-pin file count. FileCount is
// suppressed for pinRemoved because the dir is gone and the lockfile's
// recorded count would be a stale, misleading number.
type pinTarget struct {
	Key       string
	Kind      pinKind
	FileCount int // 0 for pinRemoved; post-pin file count otherwise.
}

// writePinSummary renders the pin proposal to w (stderr in production)
// before the [y/N] prompt. Lines are sorted by Key, with diff-style markers
// (+ new, ~ modified, - removed) borrowing visual vocabulary from git/jj.
//
// The trailing "N unchanged" line is always printed so a user with a few
// changes amid many stable bundles can account for everything pin saw -
// without it the summary looks like bundles went missing.
//
// Sorting is centralised here because callers build their slices from map
// iterations (randomised in Go); stable output matters for screenshots and
// piped logs.
func writePinSummary(w io.Writer, targets []pinTarget) {
	// Sort defensively so the contract is "output is always sorted"
	// regardless of caller insertion order.
	sorted := make([]pinTarget, len(targets))
	copy(sorted, targets)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Key < sorted[j].Key
	})

	// Per-bundle change lines. Unchanged targets only contribute to the
	// trailing tally.
	var unchanged int
	for _, t := range sorted {
		if t.Kind == pinUnchanged {
			unchanged++
			continue
		}

		marker := markerFor(t.Kind)
		// Width 9 fits the longest label ("unchanged") plus headroom.
		label := padRight(t.Kind.String(), 9)

		// Removed dirs have no live file count to show.
		if t.Kind == pinRemoved {
			fmt.Fprintf(w, "  %s %s %s\n", marker, label, t.Key)
		} else {
			fmt.Fprintf(w, "  %s %s %s  (%d files)\n",
				marker, label, t.Key, t.FileCount)
		}
	}

	// Always emit the tally (even "0 unchanged") so consumers grepping the
	// footer don't have to special-case its absence.
	fmt.Fprintf(w, "\n%d unchanged\n", unchanged)
}

// markerFor maps a pinKind to its single-character diff marker. We borrow
// (+ add, ~ change, - remove) from git/jj so the visual vocabulary is
// already familiar. pinUnchanged never appears in the per-line section; the
// space fallback is just defensive.
func markerFor(k pinKind) string {
	switch k {
	case pinNew:
		return "+"
	case pinModified:
		return "~"
	case pinRemoved:
		return "-"
	}
	return " "
}

// padRight right-pads s with spaces to at least n chars. Returns s
// unchanged if already wider than n - columns may drift, but no information
// is truncated.
func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	pad := make([]byte, n-len(s))
	for i := range pad {
		pad[i] = ' '
	}
	return s + string(pad)
}
