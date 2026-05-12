package seal

import (
	"slices"
	"sort"
	"strings"
	"testing"
)

// TestClassify covers all four small synthetic project. We construct lockfile
// + on-disk maps.
// memory so the test exercises the classification logic without touching the
// filesystem - that's the whole point of taking onDisk as a parameter rather
// than walking inside Classify.
func TestClassify(t *testing.T) {
	hashA := "sha256:" + strings.Repeat("a", 64)
	hashB := "sha256:" + strings.Repeat("b", 64)

	// Lockfile has three bundles. "verified" + "removed" share an
	// internally-consistent files map; "mismatch" deliberately has a contentHash
	// that doesn't match its files (we only check Files equality in classify, so
	// this just makes the bundle structurally distinct).
	files := map[string]string{"x": hashA}
	lf := &Lockfile{
		Version: 1,
		Policy:  "block",
		Bundles: map[string]Bundle{
			"./verified": {ContentHash: ContentHash(files), Files: files},
			"./removed":  {ContentHash: ContentHash(files), Files: files},
			"./mismatch": {ContentHash: ContentHash(files), Files: files},
		},
	}

	// On-disk picture:
	// ./verified - matches lockfile exactly → Verified ./removed - absent (not
	// in onDisk) → Removed ./mismatch - present but x has different hash →
	// Mismatch ./new - present, NOT in lockfile → Unverified ./unsupported -
	// discovery matched it but hashing failed → Mismatch
	onDisk := map[string]map[string]string{
		"./verified":    {"x": hashA},
		"./mismatch":    {"x": hashB},
		"./new":         {"x": hashA},
		"./unsupported": {},
	}

	got := Classify(lf, onDisk)
	sort.Slice(got, func(i, j int) bool { return got[i].Key < got[j].Key })

	want := []State{
		{Key: "./mismatch", Status: Mismatch, Modified: []string{"x"}},
		{Key: "./new", Status: Unverified},
		{Key: "./removed", Status: Removed},
		{Key: "./unsupported", Status: Mismatch},
		{Key: "./verified", Status: Verified},
	}

	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Key != want[i].Key || got[i].Status != want[i].Status {
			t.Errorf("[%d] got {%s %s}, want {%s %s}", i, got[i].Key, got[i].Status, want[i].Key, want[i].Status)
		}
		if !slices.Equal(got[i].Modified, want[i].Modified) {
			t.Errorf("[%d] Modified: got %v, want %v", i, got[i].Modified, want[i].Modified)
		}
	}
}

// TestClassify_DiffPopulatesAllThreeFields verifies that a Mismatch bundle
// reports all three per-file change kinds: modified files (different hash),
// added files (only on disk), and missing files (only in lockfile). Without
// this test, a regression that silently dropped one diff category would still
// pass the basic four-state test above.
func TestClassify_DiffPopulatesAllThreeFields(t *testing.T) {
	hashA := "sha256:" + strings.Repeat("a", 64)
	hashB := "sha256:" + strings.Repeat("b", 64)
	hashC := "sha256:" + strings.Repeat("c", 64)

	lockedFiles := map[string]string{
		"unchanged.txt": hashA,
		"modified.txt":  hashA,
		"missing.txt":   hashA, // in lockfile, missing on disk
	}
	diskFiles := map[string]string{
		"unchanged.txt": hashA,
		"modified.txt":  hashB, // same key, different hash
		"added.txt":     hashC, // on disk, not in lockfile
	}

	lf := &Lockfile{
		Version: 1,
		Policy:  "block",
		Bundles: map[string]Bundle{
			"./b": {ContentHash: ContentHash(lockedFiles), Files: lockedFiles},
		},
	}
	onDisk := map[string]map[string]string{"./b": diskFiles}

	got := Classify(lf, onDisk)
	if len(got) != 1 {
		t.Fatalf("expected 1 state, got %d", len(got))
	}
	st := got[0]
	if st.Status != Mismatch {
		t.Fatalf("expected Mismatch, got %s", st.Status)
	}
	if !slices.Equal(st.Modified, []string{"modified.txt"}) {
		t.Errorf("Modified: got %v, want [modified.txt]", st.Modified)
	}
	if !slices.Equal(st.Added, []string{"added.txt"}) {
		t.Errorf("Added: got %v, want [added.txt]", st.Added)
	}
	if !slices.Equal(st.Missing, []string{"missing.txt"}) {
		t.Errorf("Missing: got %v, want [missing.txt]", st.Missing)
	}
}

// TestStatus_String pins the human-readable form of each Status so a future
// re-ordering of the iota constants doesn't silently rebrand an enum value
// (e.g. "Verified" suddenly becoming "Removed").
func TestStatus_String(t *testing.T) {
	for _, c := range []struct {
		s    Status
		want string
	}{
		{Verified, "verified"},
		{Unverified, "unverified"},
		{Removed, "removed"},
		{Mismatch, "mismatch"},
	} {
		if got := c.s.String(); got != c.want {
			t.Errorf("Status(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

// TestStatus_ZeroValueIsNotVerified pins the zero-value safety contract:
// a default-initialized State{} must NEVER look like a successful
// verification. Without this test, a future refactor that "tidies up" the
// iota ordering and accidentally puts Verified at iota=0 would silently make
// every uninitialized State render as verified.
func TestStatus_ZeroValueIsNotVerified(t *testing.T) {
	var zero Status
	if zero == Verified {
		t.Fatalf("zero-value Status must not equal Verified - security-sensitive default")
	}
	if got := zero.String(); got == "verified" {
		t.Fatalf("zero-value Status renders as %q; must not equal \"verified\"", got)
	}
}
