package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// TestVerifyJSON_Shape pins the JSON output to the a single top-level object
// with "status" plus the four state arrays.
// Any tooling that parses `seal verify --json` will depend on these exact
// keys being present - pinning them in a test means breaking that contract
// requires updating this test, which is the right friction.
func TestVerifyJSON_Shape(t *testing.T) {
	// One bundle per Status keeps the test sensitive to grouping bugs (e.g.
	// accidentally tagging a Mismatch as Verified).
	states := []seal.State{
		{Key: "./a", Status: seal.Verified},
		{Key: "./b", Status: seal.Mismatch},
		{Key: "./c", Status: seal.Unverified},
		{Key: "./d", Status: seal.Removed},
	}

	var buf bytes.Buffer
	if err := WriteVerifyJSON(&buf, "Blocked", states); err != nil {
		t.Fatalf("WriteVerifyJSON: %v", err)
	}

	// json.Unmarshal verifies the output is *valid* JSON and lets us inspect it
	// as a map. Failing here means we're emitting garbage, not just the wrong
	// shape.
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}

	// The "status" field is the headline - it's what CI scripts grep.
	if got["status"] != "Blocked" {
		t.Errorf("status: got %v, want %q", got["status"], "Blocked")
	}

	// Every state array must be present even if empty. This is a stability
	// guarantee for consumers: they never have to write `if got["verified"] ==
	// nil`.
	for _, k := range []string{"verified", "unverified", "removed", "mismatch"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing top-level key %q", k)
		}
	}
}

// TestVerifyJSON_EmptyArrays verifies the "[]" - not "null" - invariant when
// no bundles populated a given status group. JSON's distinction between
// absent-key, null, and [] trips up shell scripts and `jq` pipelines all the
// time; we pick "[]" deliberately and pin it here.
func TestVerifyJSON_EmptyArrays(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteVerifyJSON(&buf, "Verified", nil); err != nil {
		t.Fatalf("WriteVerifyJSON: %v", err)
	}

	// Substring assertion (not full Unmarshal-and-compare) because
	// Unmarshal does NOT distinguish `[]` from a missing key - both produce a
	// nil/absent slice. We need the raw bytes here.
	out := buf.String()
	for _, want := range []string{
		`"verified": []`,
		`"unverified": []`,
		`"removed": []`,
		`"mismatch": []`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected substring %q in:\n%s", want, out)
		}
	}
}

// TestVerifyJSON_Sorted verifies the per-status arrays come out.
// lexicographic order even when the caller supplies an out-of-order slice.
// Determinism matters because `seal verify --json` output is diffed by CI
// systems - flapping order would generate noise commits.
func TestVerifyJSON_Sorted(t *testing.T) {
	// Same status, scrambled keys: the output array MUST be sorted.
	states := []seal.State{
		{Key: "./c", Status: seal.Verified},
		{Key: "./a", Status: seal.Verified},
		{Key: "./b", Status: seal.Verified},
	}

	var buf bytes.Buffer
	if err := WriteVerifyJSON(&buf, "Verified", states); err != nil {
		t.Fatalf("WriteVerifyJSON: %v", err)
	}

	// We assert ordering by checking the byte positions of each key in the
	// output string. Cheaper and clearer than re-parsing.
	out := buf.String()
	iA := strings.Index(out, `"./a"`)
	iB := strings.Index(out, `"./b"`)
	iC := strings.Index(out, `"./c"`)
	if !(iA != -1 && iA < iB && iB < iC) {
		t.Fatalf("expected ./a < ./b < ./c in output, got positions %d/%d/%d\n%s",
			iA, iB, iC, out)
	}
}