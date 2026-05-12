package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errUnsupportedBundlePath = errors.New("unsupported bundle path")

// bundleKeyToPath converts a bundle key into a filesystem path rooted at cwd.
//
// Bundle keys are forward-slash, "./"-anchored strings like "./skills/foo".
// The filesystem layer wants native separators and an absolute path; this
// helper is the single point of conversion so verify/init/pin don't each
// reinvent the trim-prefix/Join idiom.
//
// Accepts keys with OR without the "./" prefix. Validate enforces the prefix
// on lockfile-sourced keys, but internal callers (e.g. ExpandPatterns output)
// may pass either form.
//
// filepath.Join handles slash-to-platform-separator translation on Windows.
func bundleKeyToPath(cwd, key string) string {
	rel := strings.TrimPrefix(key, "./")
	return filepath.Join(cwd, rel)
}

func bundleKeyToSafePath(cwd, key string) (string, error) {
	rel := strings.TrimPrefix(key, "./")
	if rel == "." {
		return "", fmt.Errorf("%w: exact \".\" bundle key is invalid", errUnsupportedBundlePath)
	}

	accumulated := cwd
	for _, seg := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		if seg == "" {
			return "", fmt.Errorf("%w: bundle key %q contains an empty path segment", errUnsupportedBundlePath, key)
		}

		entries, err := os.ReadDir(accumulated)
		if err != nil {
			return "", err
		}
		found := false
		for _, entry := range entries {
			if entry.Name() == seg {
				found = true
				break
			}
		}
		if !found {
			return "", os.ErrNotExist
		}

		next := filepath.Join(accumulated, seg)
		info, err := os.Lstat(next)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: bundle key %q contains symlink segment %q", errUnsupportedBundlePath, key, seg)
		}
		accumulated = next
	}

	return accumulated, nil
}
