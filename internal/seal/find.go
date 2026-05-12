package seal

import (
	"os"
	"path/filepath"
)

// LockfileName is the canonical lockfile filename. Centralised so no caller
// hard-codes the string.
const LockfileName = "seal.json"

// FindLockfile returns the path to seal.json inside workingDir.
//
// MUST NOT traverse parent directories: a `seal verify` invoked from a
// random subdirectory of a monorepo must not accidentally pick up the
// repo-root lockfile and mask a missing per-project one.
//
// Returns the underlying os.Stat error unwrapped so callers can use
// errors.Is(err, fs.ErrNotExist) to distinguish "no lockfile here" from
// "present but broken" (the latter surfaces later from ReadFile).
func FindLockfile(workingDir string) (string, error) {
	candidate := filepath.Join(workingDir, LockfileName)
	if _, err := os.Stat(candidate); err != nil {
		return "", err
	}
	return candidate, nil
}