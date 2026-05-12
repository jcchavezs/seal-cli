package cli

import (
	"fmt"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// buildInitLockfile produces the draft Lockfile that `seal init` previews to
// the user and (on confirmation) writes. It touches the filesystem to hash
// bundle contents but performs no writes and never prompts.
//
// Pipeline:
//  1. ProposeFor(cwd) returns patterns whose parent directory exists. This is
//     the entire opt-in surface; we never auto-add a pattern whose layout has
//     no on-disk presence.
//  2. ExpandPatterns walks cwd and returns the matched bundle keys.
//  3. HashSealedRoot for each key produces the NFC-keyed files map and
//     aggregate ContentHash. The returned draft has real hashes - writing it
//     would be a complete pin with no further work.
//
// Split from RunInit so tests can drive it with a fixture tree and assert on
// the *seal.Lockfile directly, without simulating stdin/stderr/TTY.
//
// Error policy: any HashSealedRoot failure aborts. An empty bundle or
// unreadable file at init time is a producer-side error the user should fix
// before pinning; silently dropping it would write a lockfile quietly missing
// data the user expected.
func buildInitLockfile(cwd, policy string) (*seal.Lockfile, error) {
	// ProposeFor returns patterns in Registry declaration order; we keep that
	// order in the draft for consistent preview output across runs.
	patterns := seal.ProposeFor(cwd)

	// A pattern whose parent exists may still match zero children (empty
	// directory). That's fine - we surface the pattern in the preview and just
	// produce no bundles for it.
	keys, err := seal.ExpandPatterns(cwd, patterns)
	if err != nil {
		// ExpandPatterns only fails on malformed patterns. Our patterns come from
		// the in-binary Registry (validated by TestRegistry_AllPatternsValid), so
		// this should never fire - surface it anyway in case the Registry grows
		// a bad entry.
		return nil, fmt.Errorf("expand heuristic patterns: %w", err)
	}

	bundles := make(map[string]seal.Bundle, len(keys))
	for _, k := range keys {
		full := bundleKeyToPath(cwd, k)
		files, ch, err := seal.HashSealedRoot(full)
		if err != nil {
			// Wrap with the bundle key so the user can see which directory failed.
			return nil, fmt.Errorf("hash bundle %s: %w", k, err)
		}
		bundles[k] = seal.Bundle{
			ContentHash: ch,
			Files:       files,
			// Revision intentionally empty; the field is informational and we
			// don't derive it yet. JSON omitempty drops it on write.
		}
	}

	// Nil-slice (not empty slice) so the JSON encoder's omitempty drops the
	// Discovery key from seal.json entirely when no patterns matched.
	var discovery []string
	if len(patterns) > 0 {
		discovery = patterns
	}

	return &seal.Lockfile{
		Version:   1,
		Policy:    policy,
		Discovery: discovery,
		Bundles:   bundles,
	}, nil
}
