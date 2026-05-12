package seal

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// hashRegex matches "sha256:" + 64 lowercase hex. Pre-compiled once.
var hashRegex = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Validate enforces every rule that JSON shape alone can't. The semantic
// half of ReadFile: Decode proves shape, Validate proves usability.
//
// Returns the FIRST error. Aggregating would be friendlier but adds
// order-dependent complexity; fix-first, re-run, fix-next is simple and
// deterministic.
//
// Check order matters: bundle key shape is validated before nesting and
// discovery coverage (so a key without "./" is not misread as nested roots),
// structural defects surface before content defects (so a malformed path
// doesn't look like a contentHash mismatch), and NFC duplicate detection runs
// before contentHash recompute (the recompute NFC-normalises internally, so
// colliding keys would produce a self-consistent but corrupted hash and slip
// past the check).
func Validate(lf *Lockfile) error {
	if err := validateVersion(lf.Version); err != nil {
		return err
	}
	if err := validatePolicy(lf.Policy); err != nil {
		return err
	}
	// Discovery validated at the top level; a malformed pattern is a
	// structural problem, not bundle-specific.
	for i, p := range lf.Discovery {
		if err := ValidatePattern(p); err != nil {
			return fmt.Errorf("discovery[%d] %q: %w", i, p, err)
		}
	}
	if err := validateDiscoveryPatternsNoNestedRoots(lf.Discovery); err != nil {
		return err
	}
	for k := range lf.Bundles {
		if err := validateBundleKey(k); err != nil {
			return fmt.Errorf("bundle key %q: %w", k, err)
		}
	}
	if err := validateBundleKeysNoNesting(lf.Bundles); err != nil {
		return err
	}
	if err := validateDiscoveryCoverage(lf); err != nil {
		return err
	}
	if err := validateBundleKeysUniqueAfterNFC(lf.Bundles); err != nil {
		return err
	}
	for k, b := range lf.Bundles {
		// Empty-files check first so the diagnostic points at the real defect
		// rather than at a downstream contentHash mismatch caused by hashing
		// an empty map.
		if err := validateBundleFilesNonEmpty(b); err != nil {
			return fmt.Errorf("bundle %q: %w", k, err)
		}
		// Path canonicalisation before content checks: a malformed path is
		// the more actionable diagnostic.
		if err := validateBundleFilesPaths(b); err != nil {
			return fmt.Errorf("bundle %q: %w", k, err)
		}
		// NFC dup-detect BEFORE contentHash recompute (see note above).
		if err := validateBundleFilesUniqueAfterNFC(b); err != nil {
			return fmt.Errorf("bundle %q: %w", k, err)
		}
		if err := validateBundleHashes(b); err != nil {
			return fmt.Errorf("bundle %q: %w", k, err)
		}
		// Recompute AFTER per-file hash format checks so malformed per-file
		// hashes surface as "invalid hash format" rather than the more
		// confusing "contentHash mismatch".
		if err := validateBundleContentHash(b); err != nil {
			return fmt.Errorf("bundle %q: %w", k, err)
		}
	}
	return nil
}

func validateBundleKeysNoNesting(bundles map[string]Bundle) error {
	keys := make([]string, 0, len(bundles))
	for k := range bundles {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if !BundleKeyIsStrictAncestor(keys[i], keys[j], true) {
				continue
			}
			containing := keys[i]
			if BundleKeyIsStrictAncestor(keys[j], keys[i], false) {
				containing = keys[j]
			}
			return fmt.Errorf(
				"bundle keys %q and %q are nested sealed roots; record only the containing root %q",
				keys[i], keys[j], containing,
			)
		}
	}
	return nil
}

func validateDiscoveryPatternsNoNestedRoots(patterns []string) error {
	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			if DiscoveryPatternsProduceNestedRoots(patterns[i], patterns[j]) {
				return fmt.Errorf(
					"discovery patterns %q and %q can produce nested sealed roots; choose one boundary model (whole root or per-child, not both)",
					patterns[i], patterns[j],
				)
			}
		}
	}
	return nil
}

func validateDiscoveryCoverage(lf *Lockfile) error {
	if len(lf.Bundles) == 0 {
		return nil
	}
	for key := range lf.Bundles {
		if BundleKeyCoveredByDiscovery(lf.Discovery, key) {
			continue
		}
		return DiscoveryCoverageError(lf.Discovery, key)
	}
	return nil
}

// validateBundleKeysUniqueAfterNFC rejects byte-distinct bundle keys that
// normalise to the same output key. Without this, Encode would have to choose
// one bundle and silently drop the other.
func validateBundleKeysUniqueAfterNFC(bundles map[string]Bundle) error {
	seen := make(map[string]string, len(bundles))
	for k := range bundles {
		nk := norm.NFC.String(k)
		if prev, ok := seen[nk]; ok {
			return fmt.Errorf("bundle keys %q and %q both NFC-normalize to %q", prev, k, nk)
		}
		seen[nk] = k
	}
	return nil
}

// validateBundleFilesPaths checks every key in a bundle's files map.
// Implementations are required to canonicalise paths BEFORE recording, so
// non-canonical input reaching the validator was hand-edited (or produced
// by a buggy implementation). Surfacing rather than silently re-normalising
// keeps the lockfile authoritative and makes tampering visible.
//
// File-map keys differ from bundle keys (see validateBundleKey): bundle
// keys MUST start with "./" or be exactly ".", file keys MUST NOT start
// with "./" - they're already sealed-root-relative by convention.
func validateBundleFilesPaths(b Bundle) error {
	if isFileRootFilesMap(b.Files) {
		return nil
	}
	for path := range b.Files {
		if err := validateFilePath(path); err != nil {
			return fmt.Errorf("file path %q: %w", path, err)
		}
	}
	return nil
}

// isFileRootFilesMap recognises the regular-file sealed-root representation:
// the sealed root itself is recorded as the sole files entry named ".".
// Mixed maps such as {".": ..., "extra.md": ...} are NOT file-root maps;
// they fall through to validateFilePath, which rejects "." as ambiguous.
func isFileRootFilesMap(files map[string]string) bool {
	_, ok := files["."]
	return ok && len(files) == 1
}

// validateFilePath is the single-path rule, split out so it can be reused
// (and unit-tested) without the bundle wrapper.
func validateFilePath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	// "." is reserved for bundle keys; you can't hash "the directory itself".
	if p == "." {
		return fmt.Errorf("\".\" is reserved for bundle keys, not file paths")
	}
	// Redundant stutter; the encoder strips it on pin.
	if strings.HasPrefix(p, "./") {
		return fmt.Errorf("must not start with \"./\"")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("absolute path not allowed")
	}
	if strings.Contains(p, "\\") {
		return fmt.Errorf("backslash not allowed")
	}
	// Explicit trailing-slash / "//" checks for clearer diagnostics - the
	// segment loop below would also catch them.
	if strings.HasSuffix(p, "/") {
		return fmt.Errorf("trailing slash not allowed")
	}
	if strings.Contains(p, "//") {
		return fmt.Errorf("repeated separator not allowed")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid segment %q", seg)
		}
	}
	return nil
}

// validateBundleFilesUniqueAfterNFC rejects two files-map keys that
// NFC-normalise to the same string. JSON parsing preserves byte-distinct
// keys, but the encoder later collapses them - silently dropping one entry.
//
// Practical relevance: macOS APFS historically reports filenames in NFD;
// Linux/Windows in NFC. Without this rule, the same file pinned on macOS
// vs Linux could produce two byte-distinct entries that the lockfile loses
// on every re-encode.
func validateBundleFilesUniqueAfterNFC(b Bundle) error {
	// seen maps NFC form → first raw key, so the error names BOTH offenders.
	seen := make(map[string]string, len(b.Files))
	for k := range b.Files {
		nk := norm.NFC.String(k)
		if prev, ok := seen[nk]; ok {
			return fmt.Errorf("files keys %q and %q both NFC-normalize to %q", prev, k, nk)
		}
		seen[nk] = k
	}
	return nil
}

// validateBundleFilesNonEmpty rejects an empty files map. Pre-flight for
// the per-file hash checks - nothing useful can be said about an empty map.
// len() handles nil and empty identically, covering both {"files": {}} and
// missing-files cases.
func validateBundleFilesNonEmpty(b Bundle) error {
	if len(b.Files) == 0 {
		return fmt.Errorf("files map is empty (a bundle must contain at least one file)")
	}
	return nil
}

// validateBundleContentHash recomputes the recorded contentHash through the
// same ContentHash() the encoder uses. A hand-edited or tampered file hash
// paired with a stale-but-well-formed contentHash would otherwise slip past
// validation, defeating the integrity field.
func validateBundleContentHash(b Bundle) error {
	got := ContentHash(b.Files)
	if got != b.ContentHash {
		return fmt.Errorf("contentHash mismatch: recorded %q, recomputed %q", b.ContentHash, got)
	}
	return nil
}

// validateBundleHashes enforces sha256:<64 lowercase hex> form on the
// bundle-level contentHash and every per-file hash. Recomputation lives in
// validateBundleContentHash so each rule is one reviewable concern.
func validateBundleHashes(b Bundle) error {
	if !hashRegex.MatchString(b.ContentHash) {
		return fmt.Errorf("contentHash %q is not in sha256:<64-lowercase-hex> form", b.ContentHash)
	}
	for path, h := range b.Files {
		if !hashRegex.MatchString(h) {
			return fmt.Errorf("file %q has invalid hash %q", path, h)
		}
	}
	return nil
}

// validateBundleKey enforces bundle-key shape: "./<relative-path>". The "./"
// prefix is what distinguishes bundle keys from absolute paths or accidental
// host paths. Seal v1 does not support the project root itself, so "." is
// invalid.
func validateBundleKey(k string) error {
	if k == "." {
		return fmt.Errorf("exact \".\" bundle key is invalid")
	}
	if !strings.HasPrefix(k, "./") {
		return fmt.Errorf("must start with \"./\"")
	}
	rest := strings.TrimPrefix(k, "./")
	if rest == "" {
		return fmt.Errorf("empty path after \"./\"")
	}
	if strings.Contains(rest, "\\") {
		return fmt.Errorf("backslash not allowed")
	}
	// Explicit trailing-slash / "//" checks for clearer diagnostics; the
	// segment loop below would also catch them.
	if strings.HasSuffix(rest, "/") {
		return fmt.Errorf("trailing slash not allowed")
	}
	if strings.Contains(rest, "//") {
		return fmt.Errorf("repeated separator not allowed")
	}
	for _, seg := range strings.Split(rest, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid segment %q", seg)
		}
	}
	return nil
}

// validatePolicy enforces exactly "block" or "warn". Case is significant -
// we don't lowercase, because a noisy human-edit would otherwise drift
// silently from byte-deterministic lockfile output.
func validatePolicy(p string) error {
	if p != "block" && p != "warn" {
		return fmt.Errorf("policy must be \"block\" or \"warn\", got %q", p)
	}
	return nil
}

// validateVersion accepts only 1. Rejects both 0 (the Go zero value of an
// unset field) and 2+ (future schemas this binary can't parse).
func validateVersion(v int) error {
	if v != 1 {
		return fmt.Errorf("version %d not supported (only 1)", v)
	}
	return nil
}
