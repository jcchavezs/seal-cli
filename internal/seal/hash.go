package seal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"golang.org/x/text/unicode/norm"
)

// HashSealedRoot walks every regular file under root, hashes each, and
// aggregates them into the bundle's contentHash. Used by `pin` (record
// fresh hashes) and `verify` (recompute to compare against the lockfile).
//
// Returns NFC-normalised relative paths → "sha256:<hex>", the aggregate
// contentHash, and an error on walk failure, unsupported file type, empty
// bundle, or NFC collision.
//
// Defensive NFC-collision check: if two byte-distinct filesystem entries
// normalise to the same path (composed vs decomposed forms of the same
// logical filename), error rather than silently merging them into one map
// key.
func HashSealedRoot(root string) (map[string]string, string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, "", err
	}
	if info.Mode().IsRegular() {
		h, err := sha256File(root)
		if err != nil {
			return nil, "", fmt.Errorf("hash %s: %w", root, err)
		}
		files := map[string]string{".": "sha256:" + h}
		return files, ContentHash(files), nil
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("unsupported file type at %s: %s", root, info.Mode().Type())
	}

	files := make(map[string]string)
	// seen maps NFC form → original pre-NFC path so the collision error can
	// name BOTH offending raw filenames.
	seen := make(map[string]string)

	err = WalkRegularFiles(root, func(rel, full string) error {
		// NFC once here: ContentHash's joinEntries lookup also assumes NFC,
		// so normalising at this boundary fulfils both contracts.
		nfc := norm.NFC.String(rel)

		if prev, dup := seen[nfc]; dup {
			return fmt.Errorf("filesystem entries %q and %q both NFC-normalize to %q", prev, rel, nfc)
		}
		seen[nfc] = rel

		// Per-file hash is SHA-256 of the raw bytes - no transformation.
		h, err := sha256File(full)
		if err != nil {
			return fmt.Errorf("hash %s: %w", rel, err)
		}
		files[nfc] = "sha256:" + h
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	// Enforce non-empty here on the producer side so `pin` can never write
	// a lockfile that ReadFile would later reject.
	if len(files) == 0 {
		return nil, "", fmt.Errorf("no trackable files under %s", root)
	}

	return files, ContentHash(files), nil
}

// sha256File streams a file through the hasher so we never load the whole
// thing into memory - some bundles ship multi-megabyte fixtures.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	// Ignore close error: a successful Sum already produced the answer; a
	// deferred close failure on a read-only file is an OS quirk, not a
	// logic bug.
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
