package seal

import (
	"strings"
	"testing"
)

// TestValidate_Version verifies the supported-versions check.
// Currently only version 1 is accepted.
// integer (or missing field) makes the lockfile invalid.
func TestValidate_Version(t *testing.T) {
	// Happy path: a minimal valid lockfile with version 1.
	if err := Validate(newValidLockfile()); err != nil {
		t.Fatalf("valid lockfile rejected: %v", err)
	}

	// Reject any other version. We test both "future" (2) and "missing" (0, the
	// Go zero value) to cover both directions.
	for _, v := range []int{0, 2, 99} {
		lf := newValidLockfile()
		lf.Version = v
		err := Validate(lf)
		if err == nil {
			t.Errorf("version %d should be rejected", v)
			continue
		}
		// Sanity-check the error message names the field so users have a chance to
		// act on the diagnostic.
		if !strings.Contains(err.Error(), "version") {
			t.Errorf("version %d: error %q does not mention 'version'", v, err)
		}
	}
}

// TestValidate_Policy verifies "block" or "warn". Any other value (including
// casing variants like "BLOCK" or empty string) is invalid.
//
// Why both casing and unknown values: the lockfile is reviewed in code review
// by humans who might fix-finger a value. Strict matching forces the user to
// use one of the two silently accepting a near-miss like "Block".
func TestValidate_Policy(t *testing.T) {
	// Happy path values.
	for _, ok := range []string{"block", "warn"} {
		lf := newValidLockfile()
		lf.Policy = ok
		if err := Validate(lf); err != nil {
			t.Errorf("policy %q: unexpected error %v", ok, err)
		}
	}
	// Rejected values: empty, mis-cased, near-miss synonyms.
	for _, bad := range []string{"", "BLOCK", "Block", "permit", "off", "deny"} {
		lf := newValidLockfile()
		lf.Policy = bad
		err := Validate(lf)
		if err == nil {
			t.Errorf("policy %q should be rejected", bad)
			continue
		}
		if !strings.Contains(err.Error(), "policy") {
			t.Errorf("policy %q: error %q does not mention 'policy'", bad, err)
		}
	}
}

// TestValidate_BundleKey covers.
// Bundle keys must be project-root-relative paths prefixed with "./", without
// any of the path-traversal or normalization hazards listed.
//
// The exact "." key is invalid because Seal v1 does not support the project
// root itself as a sealed root.
func TestValidate_BundleKey(t *testing.T) {
	// Each rejection case names the reader can map a failure back to a specific
	// line.
	for _, c := range []struct {
		name string
		key  string
	}{
		{"missing leading ./", "plugins/foo"},
		{"absolute path", "/plugins/foo"},
		{"trailing slash", "./plugins/foo/"},
		{"intermediate .. segment", "./plugins/../foo"},
		{"intermediate . segment", "./plugins/./foo"},
		{"backslash", "./plugins\\foo"},
		{"repeated separator", "./plugins//foo"},
		{"empty after ./", "./"},
		{"exact dot", "."},
	} {
		t.Run(c.name, func(t *testing.T) {
			lf := newValidLockfile()
			// Rehome the only bundle under the bad key so the rest.
			// the lockfile stays valid and only the key under test fails.
			entry := lf.Bundles["./.claude/skills/foo"]
			delete(lf.Bundles, "./.claude/skills/foo")
			lf.Bundles[c.key] = entry
			if err := Validate(lf); err == nil {
				t.Errorf("bundle key %q should be rejected", c.key)
			}
		})
	}
}

// TestValidate_HashFormat verifies hash recorded in the lockfile (per-file
// hashes and the bundle-level contentHash) must be exactly "sha256:" followed
// by 64 lowercase hex characters. Any deviation invalidates the lockfile.
//
// Why strict: cross-implementation byte-determinism depends on every
// implementation emitting and accepting the same canonical form. If uppercase
// hex were allowed, two implementations could produce byte-different but
// semantically-equal lockfiles, and Git diffs would churn for no reason.
func TestValidate_HashFormat(t *testing.T) {
	hex64 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, c := range []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"prefix only", "sha256:"},
		{"uppercase prefix", "SHA256:" + hex64},
		{"capitalized prefix", "Sha256:" + hex64},
		{"uppercase hex", "sha256:" + strings.ToUpper(hex64)},
		{"non-hex chars", "sha256:" + strings.Repeat("z", 64)},
		{"too short", "sha256:" + strings.Repeat("a", 63)},
		{"too long", "sha256:" + strings.Repeat("a", 65)},
		{"bare hex no prefix", hex64},
		{"sha1 algorithm", "sha1:" + strings.Repeat("a", 40)},
	} {
		t.Run(c.name, func(t *testing.T) {
			lf := newValidLockfile()
			// Mutate the only file hash to the bad value, then recompute contentHash
			// so we don't trip the contentHash-mismatch check (which lands in a later
			// commit) before the hash-format check runs.
			b := lf.Bundles["./.claude/skills/foo"]
			b.Files = map[string]string{"SKILL.md": c.hash}
			b.ContentHash = ContentHash(b.Files)
			lf.Bundles["./.claude/skills/foo"] = b
			if err := Validate(lf); err == nil {
				t.Errorf("file hash %q should be rejected", c.hash)
			}
		})
	}
}

// TestValidate_EmptyFilesMap verifies at least one trackable file, and a
// lockfile is invalid if any bundle's files map is empty.
//
// We test both representations of "no files" - a nil map and an explicitly
// empty map - because Go encodes them identically (len == 0) but a future
// refactor that switches on `b.Files == nil` would silently break the other
// case. Asserting both keeps the rule independent of representation.
func TestValidate_EmptyFilesMap(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
	}{
		{"nil files map", nil},
		{"empty files map", map[string]string{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			lf := newValidLockfile()
			// Replace the only bundle's files map with the under-test value.
			// We zero out contentHash too because ContentHash({}) of an empty map
			// would otherwise be the only possible "valid" contentHash, and we want
			// the rejection to come from the empty-files rule rather than from a
			// (later-landing) contentHash mismatch check.
			b := lf.Bundles["./.claude/skills/foo"]
			b.Files = c.files
			b.ContentHash = ContentHash(c.files)
			lf.Bundles["./.claude/skills/foo"] = b
			err := Validate(lf)
			if err == nil {
				t.Fatalf("empty files map should be rejected")
			}
			// The error message should name "files" so the user can map the diagnostic
			// back to the offending field.
			if !strings.Contains(err.Error(), "files") {
				t.Errorf("error %q does not mention 'files'", err)
			}
		})
	}
}

// TestValidate_Discovery wires ValidatePattern (already unit-tested.
// pattern_test.go) into the Validate path. The point of this test is not to
// re-prove ValidatePattern - it's to confirm Validate actually CALLS it.
// Without this integration check, ValidatePattern could be silently dead code
// from the lockfile-validation viewpoint.
//
// We pick one representative bad pattern ("**" - explicitly forbidden.
// v1 the wiring itself.
func TestValidate_Discovery(t *testing.T) {
	// Good pattern should not affect the otherwise-valid lockfile.
	good := newValidLockfile()
	good.Discovery = []string{".claude/skills/*"}
	if err := Validate(good); err != nil {
		t.Errorf("valid discovery pattern rejected: %v", err)
	}

	// Bad pattern (recursive glob) should fail and the error should identify the
	// offending pattern by index so the user can fix the right line in
	// seal.json.
	bad := newValidLockfile()
	bad.Discovery = []string{".claude/skills/*", "plugins/**"}
	err := Validate(bad)
	if err == nil {
		t.Fatalf("recursive glob ** in discovery should be rejected")
	}
	if !strings.Contains(err.Error(), "discovery") {
		t.Errorf("error %q does not mention 'discovery'", err)
	}
}

// TestValidate_ContentHashRecompute verifies a bundle's recorded contentHash
// MUST equal the cryptographic aggregate of its recorded files map. This is
// the rule that makes seal.json internally tamper-evident - without it, a
// hand-edited file hash could stay paired with the original (now-stale)
// contentHash and slip past validation.
//
// We test two flavors of mismatch to be sure the check isn't accidentally
// only catching a specific shape of corruption:
//
// - contentHash points to a totally unrelated digest (the most common
// hand-edit error: copy-paste from another bundle).
// - files map is mutated after contentHash was computed (the "swap a real
// file for a tampered one" attack the rule is designed to stop).
func TestValidate_ContentHashRecompute(t *testing.T) {
	t.Run("contentHash does not match files", func(t *testing.T) {
		lf := newValidLockfile()
		// Replace contentHash with a syntactically valid but wrong digest
		// (all-zeroes). The hash-format check will pass; the recompute check should
		// reject.
		b := lf.Bundles["./.claude/skills/foo"]
		b.ContentHash = "sha256:" + strings.Repeat("0", 64)
		lf.Bundles["./.claude/skills/foo"] = b
		err := Validate(lf)
		if err == nil {
			t.Fatalf("mismatched contentHash should be rejected")
		}
		if !strings.Contains(err.Error(), "contentHash") {
			t.Errorf("error %q does not mention 'contentHash'", err)
		}
	})

	t.Run("files map mutated after contentHash recorded", func(t *testing.T) {
		lf := newValidLockfile()
		// Original contentHash was computed over the {"SKILL.md": ...} map.
		// Replace one entry's hash with another valid-looking hash; the per-file
		// hash format check passes, but the aggregate now differs.
		b := lf.Bundles["./.claude/skills/foo"]
		b.Files = map[string]string{
			"SKILL.md": "sha256:" + strings.Repeat("a", 64),
		}
		// Deliberately do NOT recompute b.ContentHash here - that's the whole point
		// of the test. The lockfile is internally inconsistent.
		lf.Bundles["./.claude/skills/foo"] = b
		err := Validate(lf)
		if err == nil {
			t.Fatalf("files-map drift from contentHash should be rejected")
		}
	})

	// Sanity: the baseline lockfile (where ContentHash() was used.
	// compute the recorded contentHash) should still pass - i.e. the new rule
	// does not produce false positives.
	if err := Validate(newValidLockfile()); err != nil {
		t.Errorf("baseline valid lockfile rejected: %v", err)
	}
}

// TestValidate_FilesNFCDuplicates verifies a lockfile is internally
// inconsistent if any bundle's files map contains path keys that collapse to
// identical normalized paths.
//
// We construct two map keys that LOOK different at the byte level but
// NFC-normalize to the same string:
//
// - "café.txt" - NFC, 3 char "café" ("é" as a single codepoint)
// - "café.txt" - NFD, 4 char "café" ("e" + combining acute)
//
// Both render as "café.txt" but are byte-distinct map keys. The validator
// must reject the bundle; otherwise the encoder would later silently shadow
// one entry and produce a corrupted lockfile.
func TestValidate_FilesNFCDuplicates(t *testing.T) {
	// Two valid-looking but different per-file hashes so the per-file hash check
	// itself can't be what catches the issue.
	hashA := "sha256:" + strings.Repeat("a", 64)
	hashB := "sha256:" + strings.Repeat("b", 64)

	files := map[string]string{
		"café.txt":  hashA, // NFC
		"café.txt": hashB, // NFD - collides under NFC
	}
	lf := newValidLockfile()
	b := lf.Bundles["./.claude/skills/foo"]
	b.Files = files
	// Recompute contentHash so the (later-running) recompute check doesn't fire
	// first; we want the duplicate-detection check to be the source of the
	// rejection.
	b.ContentHash = ContentHash(files)
	lf.Bundles["./.claude/skills/foo"] = b

	err := Validate(lf)
	if err == nil {
		t.Fatalf("NFC-colliding file keys should be rejected")
	}
	// Error message should help the user diagnose the duplicate; we loosely
	// check for either "files" or "normalize" so the test isn't over-coupled to
	// the exact wording of validate.go.
	msg := err.Error()
	if !strings.Contains(msg, "files") && !strings.Contains(msg, "normalize") {
		t.Errorf("error %q does not mention files or normalization", err)
	}
}

// TestValidate_BundleKeyNFCDuplicates verifies byte-distinct bundle keys that
// collapse to the same NFC string are rejected. Without this, the deterministic
// encoder would shadow one bundle while normalizing keys for output.
func TestValidate_BundleKeyNFCDuplicates(t *testing.T) {
	entry := newValidLockfile().Bundles["./.claude/skills/foo"]
	lf := &Lockfile{
		Version: 1,
		Policy:  "block",
		Bundles: map[string]Bundle{
			"./skills/café":  entry, // NFC
			"./skills/café": entry, // NFD - collides under NFC
		},
	}

	err := Validate(lf)
	if err == nil {
		t.Fatalf("NFC-colliding bundle keys should be rejected")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bundle") && !strings.Contains(msg, "normalize") {
		t.Errorf("error %q does not mention bundle keys or normalization", err)
	}
}

// TestValidate_FilesPaths covers rules as applied to keys inside a bundle's
// files map.
//
// File-map keys are sealed-root-relative paths and MUST NOT contain any of
// the path-traversal or normalization hazards.
// Unlike bundle keys, they MUST NOT start with "./": file-map keys are
// already understood to be sealed-root-relative, and the prefix would be a
// redundant stutter that the encoder would have stripped on pin.
//
// Each rejection case names the can map a failure back to a specific line.
func TestValidate_FilesPaths(t *testing.T) {
	for _, c := range []struct {
		name string
		path string
	}{
		{"leading ./", "./SKILL.md"},
		{"absolute path", "/SKILL.md"},
		{"repeated separator", "docs//SKILL.md"},
		{"intermediate .. segment", "docs/../SKILL.md"},
		{"intermediate . segment", "docs/./SKILL.md"},
		{"leading .. segment", "../escape.md"},
		{"trailing slash", "docs/"},
		{"backslash", `docs\SKILL.md`},
		{"empty path", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			lf := newValidLockfile()
			// Replace the only files-map key with the under-test path so the failure
			// can only come from the path-validation rule.
			b := lf.Bundles["./.claude/skills/foo"]
			h := b.Files["SKILL.md"]
			b.Files = map[string]string{c.path: h}
			b.ContentHash = ContentHash(b.Files)
			lf.Bundles["./.claude/skills/foo"] = b
			if err := Validate(lf); err == nil {
				t.Errorf("file path %q should be rejected", c.path)
			}
		})
	}

	// Sanity: a plain bare filename and a multi-segment relative path should
	// both pass.
	for _, ok := range []string{"SKILL.md", "docs/intro.md", "a/b/c.txt"} {
		t.Run("ok_"+ok, func(t *testing.T) {
			lf := newValidLockfile()
			b := lf.Bundles["./.claude/skills/foo"]
			h := b.Files["SKILL.md"]
			b.Files = map[string]string{ok: h}
			b.ContentHash = ContentHash(b.Files)
			lf.Bundles["./.claude/skills/foo"] = b
			if err := Validate(lf); err != nil {
				t.Errorf("file path %q rejected unexpectedly: %v", ok, err)
			}
		})
	}
}

func TestValidate_DiscoveryCoverage(t *testing.T) {
	lf := newValidLockfile()
	lf.Discovery = nil
	err := Validate(lf)
	if err == nil {
		t.Fatal("bundle without discovery coverage should be rejected")
	}
	if !strings.Contains(err.Error(), "not statically produced") {
		t.Fatalf("error %q does not mention static discovery coverage", err)
	}
	if !strings.Contains(err.Error(), "./.claude/skills/foo") {
		t.Fatalf("error %q does not name uncovered bundle key", err)
	}
	if !strings.Contains(err.Error(), "discovery is empty") {
		t.Fatalf("error %q should mention empty discovery", err)
	}
}

func TestValidate_DiscoveryCoverage_NamesExistingPatterns(t *testing.T) {
	lf := newValidLockfile()
	lf.Discovery = []string{"skills/*", "docs/*"}
	err := Validate(lf)
	if err == nil {
		t.Fatal("uncovered bundle key should be rejected")
	}
	if !strings.Contains(err.Error(), `"skills/*"`) || !strings.Contains(err.Error(), `"docs/*"`) {
		t.Fatalf("error should list existing patterns: %v", err)
	}
	if !strings.Contains(err.Error(), "do not cover") {
		t.Fatalf("error should explain patterns do not cover the key: %v", err)
	}
}

func TestValidate_NarrowedDiscovery(t *testing.T) {
	lf := newValidLockfile()
	lf.Discovery = []string{".dirA/*"}
	lf.Bundles = map[string]Bundle{
		"./.dirB/2": lf.Bundles["./.claude/skills/foo"],
	}
	err := Validate(lf)
	if err == nil {
		t.Fatal("bundle key left behind after narrowing discovery should be rejected")
	}
	if !strings.Contains(err.Error(), "./.dirB/2") {
		t.Fatalf("error %q should name uncovered bundle key", err)
	}
}

func TestValidate_BundleKeysNoNesting(t *testing.T) {
	lf := newValidLockfile()
	lf.Discovery = []string{"skills", "skills/*"}
	child := lf.Bundles["./.claude/skills/foo"]
	lf.Bundles["./.claude/skills"] = child
	err := Validate(lf)
	if err == nil {
		t.Fatal("nested bundle keys should be rejected")
	}
	if !strings.Contains(err.Error(), "nested sealed roots") {
		t.Fatalf("error %q does not mention nested sealed roots", err)
	}
}

func TestValidate_DiscoveryPatternsNoNestedRoots(t *testing.T) {
	lf := newValidLockfile()
	lf.Bundles = map[string]Bundle{}
	lf.Discovery = []string{"skills", "skills/*"}
	err := Validate(lf)
	if err == nil {
		t.Fatal("overlapping discovery patterns should be rejected")
	}
	if !strings.Contains(err.Error(), "nested sealed roots") {
		t.Fatalf("error %q does not mention nested sealed roots", err)
	}
}

func TestValidate_DiscoveryPatternsNoNestedRoots_WildcardLiteralPrefix(t *testing.T) {
	lf := newValidLockfile()
	lf.Bundles = map[string]Bundle{}
	lf.Discovery = []string{"skills/*", "skills/foo/*"}
	err := Validate(lf)
	if err == nil {
		t.Fatal("wildcard parent with literal descendant pattern should be rejected")
	}
	if !strings.Contains(err.Error(), "nested sealed roots") {
		t.Fatalf("error %q does not mention nested sealed roots", err)
	}
}

func TestValidate_DiscoveryCoverage_RemovedKeyStillCovered(t *testing.T) {
	lf := newValidLockfile()
	lf.Discovery = []string{".dirB/*"}
	lf.Bundles = map[string]Bundle{
		"./.dirB/2": lf.Bundles["./.claude/skills/foo"],
	}
	if err := Validate(lf); err != nil {
		t.Fatalf("removed bundle key still covered by discovery should be valid: %v", err)
	}
}

func TestValidate_FileRootDotPath(t *testing.T) {
	h := "sha256:" + strings.Repeat("a", 64)
	files := map[string]string{".": h}
	lf := &Lockfile{
		Version:   1,
		Policy:    "block",
		Discovery: []string{"skills/*"},
		Bundles: map[string]Bundle{
			"./skills/m3.md": {
				ContentHash: ContentHash(files),
				Files:       files,
			},
		},
	}
	if err := Validate(lf); err != nil {
		t.Fatalf("single-file-root . path should be valid: %v", err)
	}

	mixed := map[string]string{".": h, "extra.md": h}
	lf.Bundles["./skills/m3.md"] = Bundle{
		ContentHash: ContentHash(mixed),
		Files:       mixed,
	}
	if err := Validate(lf); err == nil {
		t.Fatalf(". mixed with other file paths should be invalid")
	}
}

// newValidLockfile returns a minimal valid Lockfile for tests to mutate.
// Centralizing the baseline here keeps every TestValidate_* case.
// drifting into "well, my fixture happened to be valid anyway."
//
// The contentHash is the real cryptographic aggregate of the files map, so
// the contentHash-recomputation rule (added in a later commit) will also pass
// on this baseline.
func newValidLockfile() *Lockfile {
	files := map[string]string{
		"SKILL.md": "sha256:3bfc269594ef649228e9a74bab00f042efc91d5acc6fbee31a382e80d42388fe",
	}
	return &Lockfile{
		Version:   1,
		Policy:    "block",
		Discovery: []string{".claude/skills/*"},
		Bundles: map[string]Bundle{
			"./.claude/skills/foo": {
				ContentHash: ContentHash(files),
				Files:       files,
			},
		},
	}
}
