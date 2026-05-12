package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// writeInitSummary prints the preview `seal init` shows before the [y/N]
// prompt: discovery patterns, then bundles with file counts. Verbose mode
// expands each bundle into a sorted per-file listing.
//
// Everything is sorted because Bundles comes from a map (random iteration
// order). Stable output matters for diffing two runs and for screenshots in
// bug reports.
func writeInitSummary(w io.Writer, lf *seal.Lockfile, verbose bool) {
	fmt.Fprintln(w, "Discovery patterns to add:")
	if len(lf.Discovery) == 0 {
		// Explicit "(none)" so a silent section doesn't look like a crash.
		fmt.Fprintln(w, "  (none)")
	}
	for _, p := range lf.Discovery {
		fmt.Fprintf(w, "  + %s\n", p)
	}

	bundleKeys := make([]string, 0, len(lf.Bundles))
	for k := range lf.Bundles {
		bundleKeys = append(bundleKeys, k)
	}
	sort.Strings(bundleKeys)

	fmt.Fprintf(w, "\nBundles to add (%d):\n", len(bundleKeys))
	for _, k := range bundleKeys {
		b := lf.Bundles[k]
		fmt.Fprintf(w, "  + %s  (%d files)\n", k, len(b.Files))

		if !verbose {
			continue
		}

		// Re-sort the file paths here rather than trusting Bundle internals to
		// stay ordered - keeps determinism local.
		paths := make([]string, 0, len(b.Files))
		for p := range b.Files {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			fmt.Fprintf(w, "    %s\n", p)
		}
	}

	// Trailing blank line so the [y/N] prompt doesn't smush against output.
	fmt.Fprintln(w)
}