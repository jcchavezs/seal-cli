package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// resolvePathToBundleKey converts a user-supplied path (absolute or relative
// to cwd) into a canonical lockfile bundle key `./forward/slash/path`. Used
// by `seal pin` in targeted mode - every positional arg passes through here
// before any hashing or writing.
//
// The function enforces three invariants the lockfile depends on:
//
//  1. Inside-cwd. Paths escaping via ".." or pointing at sibling repos via
//     absolute paths are rejected.
//
//  2. Case-true byte equality. macOS APFS and Windows NTFS happily open
//     "Foo" when the on-disk entry is "foo"; that silent coercion would
//     corrupt the bundle key. The segment walk below uses os.ReadDir (which
//     reports the actual on-disk name) and requires a byte-exact match.
//     Case-sensitive filesystems (Linux ext4) already reject mismatches at
//     the syscall layer; our walk produces the same error there.
//
//  3. NFC-normalised forward-slash output. Normalisation happens after the
//     segment walk so whatever Unicode form the user typed (and that exists
//     byte-faithfully on disk) becomes canonical in the lockfile.
//
// Not the inverse of bundleKeyToPath: that helper trusts a well-formed
// lockfile key, this one has to ENFORCE the invariants on user input.
func resolvePathToBundleKey(cwd, userPath string) (string, error) {
	// Absolutise + clean so "skills/foo" and "./skills/foo" both normalise.
	absUser := userPath
	if !filepath.IsAbs(absUser) {
		absUser = filepath.Join(cwd, absUser)
	}
	absUser = filepath.Clean(absUser)
	absCwd := filepath.Clean(cwd)

	// Rel can fail only when the paths can't be expressed relatively (different
	// Windows volumes). We still check explicitly for ".." and "." below.
	rel, err := filepath.Rel(absCwd, absUser)
	if err != nil {
		return "", fmt.Errorf("compute relative path: %w", err)
	}

	if rel == "." {
		return "", fmt.Errorf("path %q is the project root, not a bundle", userPath)
	}

	// Match the ".." segment specifically (not just the prefix) so a real
	// filename like "..hidden" doesn't false-positive.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the project root %q", userPath, absCwd)
	}

	// Case-true segment walk: for each segment, ReadDir the accumulated parent
	// and require a byte-exact entry-name match. This is the only way to
	// detect "Foo" vs "foo" on case-insensitive filesystems.
	segments := strings.Split(rel, string(filepath.Separator))
	accumulated := absCwd
	for _, seg := range segments {
		// Shouldn't happen post-Clean, but reject rather than silently skip.
		if seg == "" {
			return "", fmt.Errorf("path %q has an empty component", userPath)
		}

		entries, err := os.ReadDir(accumulated)
		if err != nil {
			return "", fmt.Errorf("read directory %s: %w", accumulated, err)
		}

		// Linear scan: directory entry counts are small and we only walk as
		// many dirs as the path has segments.
		found := false
		for _, e := range entries {
			if e.Name() == seg {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf(
				"path component %q not found with that exact case under %s (case-true paths required)",
				seg, accumulated)
		}

		next := filepath.Join(accumulated, seg)
		info, err := os.Lstat(next)
		if err != nil {
			return "", fmt.Errorf("stat path component %s: %w", next, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path %q contains unsupported symlink component %q", userPath, seg)
		}
		accumulated = next
	}

	// Confirm the final target is a sealable root: directory or regular file.
	// Lstat deliberately rejects symlinks even when they resolve to a regular
	// file/dir; Seal v1 does not follow symlinks.
	info, err := os.Lstat(accumulated)
	if err != nil {
		return "", fmt.Errorf("stat resolved path: %w", err)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return "", fmt.Errorf("path %q is not a regular file or directory; bundles must be sealable roots", userPath)
	}

	// Forward-slash + NFC so the key matches what `seal init` and bulk pin
	// would produce on every platform.
	forwardSlash := filepath.ToSlash(rel)
	nfc := norm.NFC.String(forwardSlash)
	return "./" + nfc, nil
}
