package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// writeTreeForOnDisk writes a flat map of "relative path → contents" rooted
// at dir, creating intermediate directories as needed. Local helper for
// ondisk_test only; verify_test will eventually grow its own variant for
// integration tests.
func writeTreeForOnDisk(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		// Ensure parent dirs exist. 0o755 is the standard "world-readable
		// directory" mode and matches what `git checkout` writes.
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
}

// TestBuildOnDiskMap_LockfileBundles covers pass 1: every bundle key recorded
// in the lockfile is hashed when present on disk, omitted when its directory
// has been deleted. Pass-2 behaviour (discovery matches adding NEW keys) is
// intentionally out of scope here - it lands in a follow-up commit with its
// own test.
func TestBuildOnDiskMap_LockfileBundles(t *testing.T) {
	cwd := t.TempDir()

	// Two bundles on disk; the third is recorded in the lockfile but not present
	// (simulating a user `rm -rf` after pinning).
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/foo/SKILL.md": "foo content",
		"skills/bar/SKILL.md": "bar content",
	})

	// Empty Discovery slice keeps pass-2 a no-op so this test stays focused on
	// the pass-1 contract.
	lf := &seal.Lockfile{
		Version:   1,
		Policy:    "block",
		Discovery: []string{"skills/*"},
		Bundles: map[string]seal.Bundle{
			"./skills/foo":  {ContentHash: "irrelevant"},
			"./skills/bar":  {ContentHash: "irrelevant"},
			"./skills/gone": {ContentHash: "irrelevant"},
		},
	}

	got, err := buildOnDiskMap(cwd, lf)
	if err != nil {
		t.Fatalf("buildOnDiskMap: %v", err)
	}

	// Present bundles must be hashed: each result must be a non-nil map
	// containing exactly the one file we wrote.
	for _, key := range []string{"./skills/foo", "./skills/bar"} {
		files, ok := got[key]
		if !ok {
			t.Fatalf("key %q missing from on-disk map", key)
		}
		if _, hasSkill := files["SKILL.md"]; !hasSkill {
			t.Errorf("key %q: expected SKILL.md in hashed files, got %v", key, files)
		}
	}

	// Absent bundle must be excluded entirely (not zero-valued).
	// Its absence is what Classify uses to derive Removed status.
	if _, ok := got["./skills/gone"]; ok {
		t.Errorf("key ./skills/gone should be absent from map; got %v", got["./skills/gone"])
	}
}

// TestBuildOnDiskMap_DiscoveryFindsUntrackedBundles covers pass 2:
// directories that match the lockfile's Discovery glob but are NOT recorded
// as bundles must show up in the on-disk map as well. Their presence is what
// eventually lets Classify produce Unverified states - "you have this dir on
// disk but never pinned it."
//
// Pass-1 keys already present in the map MUST NOT be re-hashed.
// pass 2 (would be wasteful); we don't strictly need to test that
// optimisation, but we cover it implicitly by including one bundle that's
// both in lockfile.Bundles AND matched by the glob.
func TestBuildOnDiskMap_DiscoveryFindsUntrackedBundles(t *testing.T) {
	cwd := t.TempDir()

	// Three dirs under skills/: one pinned, two not.
	writeTreeForOnDisk(t, cwd, map[string]string{
		"skills/pinned/SKILL.md":    "p",
		"skills/untracked/SKILL.md": "u",
		"skills/new/SKILL.md":       "n",
	})

	lf := &seal.Lockfile{
		Version: 1,
		Policy:  "block",
		Bundles: map[string]seal.Bundle{
			// Only "pinned" is in the lockfile; the other two are purely discovery
			// hits.
			"./skills/pinned": {ContentHash: "irrelevant"},
		},
		Discovery: []string{"skills/*"},
	}

	got, err := buildOnDiskMap(cwd, lf)
	if err != nil {
		t.Fatalf("buildOnDiskMap: %v", err)
	}

	// All three keys must be present in the resulting map. The pinned one came
	// via pass 1; the other two via pass 2.
	for _, key := range []string{
		"./skills/pinned",
		"./skills/untracked",
		"./skills/new",
	} {
		files, ok := got[key]
		if !ok {
			t.Errorf("key %q missing from on-disk map", key)
			continue
		}
		if _, hasSkill := files["SKILL.md"]; !hasSkill {
			t.Errorf("key %q: expected SKILL.md, got %v", key, files)
		}
	}
}

// TestBuildOnDiskMap_UnreadableBundleYieldsEmpty pins the failure semantics:
// if HashSealedRoot can't process a recorded bundle (e.g. unsupported symlink
// inside, empty dir, permissions error), we surface this by storing an EMPTY
// map under the key, not.
// returning an error. Classify then naturally produces a Mismatch state
// listing every file as "missing", which is the correct observable behaviour
// - "we have it in the lockfile but couldn't read it" is a mismatch, not a
// fatal abort.
//
// Empty-directory trigger.
// contain at least one trackable file, and HashSealedRoot enforces it
// (hash.go:70). An empty dir on disk therefore reliably fails to hash on
// every platform, with no symlink/permission tricks.
func TestBuildOnDiskMap_UnreadableBundleYieldsEmpty(t *testing.T) {
	cwd := t.TempDir()

	// "broken" is a directory with no files. HashSealedRoot refuses ;
	// buildOnDiskMap must catch that error and substitute an empty file map.
	if err := os.MkdirAll(filepath.Join(cwd, "broken"), 0o755); err != nil {
		t.Fatal(err)
	}

	lf := &seal.Lockfile{
		Version:   1,
		Policy:    "block",
		Discovery: []string{"broken"},
		Bundles: map[string]seal.Bundle{
			"./broken": {ContentHash: "irrelevant"},
		},
	}

	got, err := buildOnDiskMap(cwd, lf)
	if err != nil {
		t.Fatalf("buildOnDiskMap: unexpected error %v", err)
	}

	// The key must be present (so Classify won't call it Removed) and the map
	// must be empty (so Classify sees a Mismatch).
	files, ok := got["./broken"]
	if !ok {
		t.Fatal("./broken should be present with empty map; got absent")
	}
	if len(files) != 0 {
		t.Errorf("./broken should map to empty file map; got %v", files)
	}
}