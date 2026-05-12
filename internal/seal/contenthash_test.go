package seal

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// realisticFiles is a small but.md at the bundle root and a script under a
// subdirectory. The per-file hashes are real SHA-256 outputs over the literal
// content shown next to each path in the comment, so any reader can recompute
// them externally.
//
//	SKILL.md: printf 'v1' | shasum -a 256 scripts/run.sh: printf 'echo hi' |
//	shasum -a 256
//
// Sorted by UTF-8 byte order, "SKILL.md" precedes "scripts/run.sh" because
// 'S' (0x53) < 's' (0x73).
var realisticFiles = map[string]string{
	"SKILL.md":       "sha256:3bfc269594ef649228e9a74bab00f042efc91d5acc6fbee31a382e80d42388fe",
	"scripts/run.sh": "sha256:56a79f3b115448072387c2480044bfa2cf8f90e4f5fddd8c943b4e051b81f80b",
}

// TestContentHash_KnownVector pins the contentHash output for a realistic
// input so any future change to the algorithm - or any drift from the
//
// The expected aggregate value was computed externally with:
//
//	printf 'SKILL.md:sha256:3bfc...88fe\nscripts/run.sh:sha256:56a7...f80b' |
//	shasum -a 256
//
// (Full hashes shown abbreviated here; see realisticFiles above.) If this
// test ever fails, recompute by hand FIRST (not by running ContentHash) to
// confirm whether the.
func TestContentHash_KnownVector(t *testing.T) {
	got := ContentHash(realisticFiles)
	want := "sha256:3356cd32fc0b6ad263626c6e9f3ea0d0bf82f2602e63ee7009339d322dd13c00"
	if got != want {
		t.Fatalf("ContentHash mismatch:\n got=%s\nwant=%s", got, want)
	}
}

// TestContentHash_NewlineSeparator verifies that the \n delimiter prevents
// the concatenation collision different inputs could happen to produce the
// same joined string.
func TestContentHash_NewlineSeparator(t *testing.T) {
	// Compute the proper, separator-bearing aggregate via ContentHash.
	withSep := ContentHash(realisticFiles)

	// Build the same per-entry strings WITHOUT the separator, hash that
	// directly, and assert the two hashes differ. We hash this separately (not
	// via ContentHash) so the test verifies the separator is doing real work
	// rather than being accidentally optimized away.
	collisionInput := "SKILL.md:" + realisticFiles["SKILL.md"] +
		"scripts/run.sh:" + realisticFiles["scripts/run.sh"]
	collision := "sha256:" + sha256Hex([]byte(collisionInput))

	if withSep == collision {
		t.Fatal("ContentHash must not collide with separator-less concatenation")
	}
}

// TestContentHash_OrderInsensitive verifies that two maps with the same
// (path, hash) pairs produce identical ContentHashes regardless of the order
// Go happened to iterate the input map.
//
// Go map iteration order is randomized, so this test inherently exercises
// the. Running with -count=N would also catch any latent non-determinism in
// the sort or the path normalization.
func TestContentHash_OrderInsensitive(t *testing.T) {
	// Build a second map with the same pairs but reversed insertion order in
	// source. The runtime iteration order is randomized either way, so what we
	// are really asserting is that the.
	reversed := map[string]string{
		"scripts/run.sh": realisticFiles["scripts/run.sh"],
		"SKILL.md":       realisticFiles["SKILL.md"],
	}
	if ContentHash(realisticFiles) != ContentHash(reversed) {
		t.Fatal("ContentHash must be insensitive to map iteration order")
	}
}

// sha256Hex is a test-only helper: returns lowercase hex of SHA-256(b).
// Lives in this _test.go so production code is not polluted.
// test-supporting utilities.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}