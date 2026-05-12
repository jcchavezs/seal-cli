package cli

import (
	"errors"
	"os"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// buildOnDiskMap computes the on-disk side of the verify comparison: for
// every bundle the lockfile records, produce an NFC-keyed "filename → hash"
// map by walking and hashing the directory.
//
// Per-bundle outcomes:
//   - dir exists and HashSealedRoot succeeds → key present with the files map
//     (Classify: Verified or Mismatch)
//   - dir doesn't exist → key absent (Classify: Removed)
//   - dir exists but HashSealedRoot fails (regular file at the path,
//     unsupported symlink, perms, etc.) → key present with EMPTY map
//     (Classify: Mismatch listing every recorded file as "missing"). One
//     busted bundle must not abort the whole verify.
//
// Pass 2 expands lf.Discovery and adds any matches not already covered by
// pass 1 - this is what surfaces directories on disk that were never pinned
// (Classify: Unverified). Pass 2 skips pass-1 keys so no directory is hashed
// twice.
//
// Empty-map convention rather than a sentinel string: Validate rejects
// lockfiles with empty files maps, so an empty on-disk map can never
// coincidentally equal a recorded one (Mismatch is guaranteed). It also
// composes with diffFileMaps, which lists every recorded file as "missing" -
// exactly what a user wants to see for "we couldn't read this bundle."
func buildOnDiskMap(cwd string, lf *seal.Lockfile) (map[string]map[string]string, error) {
	out := make(map[string]map[string]string, len(lf.Bundles))

	// Pass 1: every bundle the lockfile records.
	for key := range lf.Bundles {
		full, err := bundleKeyToSafePath(cwd, key)

		// Existence probe. ErrNotExist is the Removed case; any other stat error
		// (perms, I/O on cwd) is systemic and should abort rather than masquerade
		// as a bundle problem.
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if errors.Is(err, errUnsupportedBundlePath) {
				out[key] = map[string]string{}
				continue
			}
			return nil, err
		}

		// Don't propagate per-bundle hash errors - record the empty-map marker
		// so the bundle classifies as Mismatch instead of killing the run.
		files, _, err := seal.HashSealedRoot(full)
		if err != nil {
			out[key] = map[string]string{}
			continue
		}
		out[key] = files
	}

	// Pass 2: discovery-glob matches. ExpandPatterns handles a nil/empty
	// Discovery slice internally; we call unconditionally so the contract
	// ("pass 2 always runs, may add zero keys") stays uniform.
	matches, err := seal.ExpandPatterns(cwd, lf.Discovery)
	if err != nil {
		// A pattern error here means a malformed lockfile got past Validate -
		// surface as a bug rather than swallow.
		return nil, err
	}
	for _, key := range matches {
		// Skip pass-1 keys: avoids redundant hashing AND prevents pass 2 from
		// overwriting an empty-map "unreadable" marker pass 1 placed.
		if _, already := out[key]; already {
			continue
		}

		full, err := bundleKeyToSafePath(cwd, key)
		if err != nil {
			if errors.Is(err, errUnsupportedBundlePath) {
				out[key] = map[string]string{}
				continue
			}
			return nil, err
		}
		files, _, err := seal.HashSealedRoot(full)
		if err != nil {
			out[key] = map[string]string{}
			continue
		}
		out[key] = files
	}

	return out, nil
}
