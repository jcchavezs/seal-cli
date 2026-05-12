package seal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEncode_Golden walks testdata/golden, finds every <name>.input.json,
// decodes it as a "loose" Lockfile via stdlib json.Unmarshal, runs the
// production deterministic Encode on it, and compares the output bytes
// against <name>.json byte-for-byte.
//
// Why this shape:
// - Golden fixtures are checked-in JSON, easy to review on a PR.
// - Adding a new test case is "drop two files into testdata/golden", not "add
// another go test function and remember the assertion shape".
// - Byte-equality is the actual allow latent drift (different whitespace,
// different key order) while still passing.
func TestEncode_Golden(t *testing.T) {
	// Find every input fixture so adding a test case is just a matter.
	// dropping two files alongside the existing ones.
	matches, err := filepath.Glob("testdata/golden/*.input.json")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		// Defensive: a missing testdata directory would silently make this test
		// pass otherwise, hiding the absence of any encoder coverage.
		t.Fatal("no fixtures found in testdata/golden - encoder has no coverage")
	}
	for _, in := range matches {
		// Derive the golden output path: foo.input.json -> foo.json.
		out := strings.TrimSuffix(in, ".input.json") + ".json"
		// Each fixture gets its own subtest so failures point at the case.
		t.Run(filepath.Base(out), func(t *testing.T) {
			runGoldenCase(t, in, out)
		})
	}
}

func TestEncode_NFCCollisionErrors(t *testing.T) {
	t.Run("bundle keys", func(t *testing.T) {
		entry := newValidLockfile().Bundles["./.claude/skills/foo"]
		_, err := Encode(&Lockfile{
			Version: 1,
			Policy:  "block",
			Bundles: map[string]Bundle{
				"./skills/café":  entry,
				"./skills/café": entry,
			},
		})
		if err == nil {
			t.Fatal("expected NFC-colliding bundle keys to fail encoding")
		}
		if !strings.Contains(err.Error(), "normaliz") {
			t.Fatalf("error %q does not mention normalization", err)
		}
	})

	t.Run("files keys", func(t *testing.T) {
		h := "sha256:" + strings.Repeat("a", 64)
		files := map[string]string{
			"café.txt":  h,
			"café.txt": h,
		}
		_, err := Encode(&Lockfile{
			Version: 1,
			Policy:  "block",
			Bundles: map[string]Bundle{
				"./bundle": {
					ContentHash: ContentHash(files),
					Files:       files,
				},
			},
		})
		if err == nil {
			t.Fatal("expected NFC-colliding files keys to fail encoding")
		}
		if !strings.Contains(err.Error(), "normaliz") {
			t.Fatalf("error %q does not mention normalization", err)
		}
	})
}

// runGoldenCase decodes one input fixture, encodes it via Encode, and asserts
// byte-for-byte equality with the golden output file.
//
// Split out so the loop above stays focused on test discovery; the per-case
// logic is its own readable unit.
func runGoldenCase(t *testing.T, inputPath, goldenPath string) {
	t.Helper()

	// Load the input as a loose Lockfile. The input format is permissive (any
	// field order); the encoder is the canonicalizer that produces
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	var lf Lockfile
	if err := json.Unmarshal(raw, &lf); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}

	// Encode and compare against the golden bytes. We do NOT trim or otherwise
	// normalize either side - byte-equality is the contract.
	got, err := Encode(&lf)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("encoder output mismatch for %s:\n--- want (%d bytes) ---\n%s\n--- got (%d bytes) ---\n%s",
			filepath.Base(goldenPath), len(want), want, len(got), got)
	}
}
