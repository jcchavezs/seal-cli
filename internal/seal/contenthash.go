package seal

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// ContentHash computes the deterministic aggregate hash for a bundle's files
// map. The output is invariant to Go's randomised map iteration AND to the
// NFC/NFD path differences between macOS APFS and Linux/Windows filesystems.
//
// Pipeline:
//  1. NFC-normalise + sort paths
//  2. Build "<path>:<hash>" entries joined with "\n"
//  3. SHA-256 → "sha256:<lowercase-hex>"
func ContentHash(files map[string]string) string {
	paths := sortedNormalizedPaths(files)
	joined := joinEntries(paths, files)
	return digestPrefixed(joined)
}

// sortedNormalizedPaths returns the keys NFC-normalised and sorted by UTF-8
// byte order - needed because Go's map iteration is randomised.
//
// NFC matters cross-OS: macOS APFS historically reports paths in NFD (e.g.
// "コ" decomposed into base + combining marks); Linux and Windows report NFC.
// Without normalisation the same logical filename hashes differently across
// platforms.
func sortedNormalizedPaths(files map[string]string) []string {
	out := make([]string, 0, len(files))
	for p := range files {
		// NFC is a no-op on already-NFC input.
		out = append(out, norm.NFC.String(p))
	}
	sort.Strings(out)
	return out
}

// joinEntries pairs each (already-NFC) path with its hash via ":" and joins
// the entries with "\n". The newline delimiter is what prevents
// concatenation collisions across pairs.
//
// files[p] only succeeds because every caller applies NFC on the way in, so
// the normalised path still keys the original map.
func joinEntries(paths []string, files map[string]string) string {
	entries := make([]string, len(paths))
	for i, p := range paths {
		entries[i] = p + ":" + files[p]
	}
	return strings.Join(entries, "\n")
}

// digestPrefixed SHA-256s s and returns "sha256:<lowercase-hex>". Split out
// so prefix and casing live in exactly one place.
func digestPrefixed(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:])
}