package seal

import (
	"strings"
	"testing"
)

// TestDecode_Valid verifies a well-formed lockfile decodes without error and
// produces the expected struct shape. This is the happy-path smoke test;
// deeper structural checks live in validate_test.go.
func TestDecode_Valid(t *testing.T) {
	in := `{
  "version": 1,
  "policy": "block",
  "bundles": {
    "./.claude/skills/foo": {
      "contentHash": "sha256:4bc6ee3c79cf31fe7f32fb3fbcd0f96027a23dea307e5d3fcc7afa7b292e5989",
      "files": {
        "SKILL.md": "sha256:3bfc269594ef649228e9a74bab00f042efc91d5acc6fbee31a382e80d42388fe"
      }
    }
  }
}`
	lf, err := Decode([]byte(in))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Spot-check: version, policy, one bundle with one file.
	if lf.Version != 1 {
		t.Errorf("Version: got %d, want 1", lf.Version)
	}
	if lf.Policy != "block" {
		t.Errorf("Policy: got %q, want %q", lf.Policy, "block")
	}
	if got := len(lf.Bundles); got != 1 {
		t.Errorf("Bundles count: got %d, want 1", got)
	}
}

// TestDecode_UnknownField verifies at any level invalidates the lockfile. The
// decoder must reject silently added fields rather than ignoring them, since
// attackers could otherwise hide data inside a lockfile that older parsers
// would not flag.
func TestDecode_UnknownField(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{
			name: "unknown top-level field",
			in:   `{"version":1,"policy":"block","bundles":{},"surprise":true}`,
		},
		{
			name: "unknown bundle field",
			in: `{"version":1,"policy":"block","bundles":{"./a":{` +
				`"contentHash":"sha256:0000000000000000000000000000000000000000000000000000000000000000",` +
				`"files":{"x":"sha256:0000000000000000000000000000000000000000000000000000000000000000"},` +
				`"extra":"nope"}}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode([]byte(c.in))
			if err == nil {
				t.Fatal("expected unknown-field error, got nil")
			}
			// Match on the substring stdlib's DisallowUnknownFields uses so a future
			// change to the error wording surfaces in CI.
			if !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("error %q does not mention unknown field", err)
			}
		})
	}
}

// TestDecode_RequiredAndNullableFields verifies Decode enforces the JSON
// Schema constraints that Go struct zero values cannot preserve. Missing and
// explicit-null structured fields must fail at the decode boundary, before
// semantic validation gets a chance to reinterpret them as empty values.
func TestDecode_RequiredAndNullableFields(t *testing.T) {
	validHash := "sha256:" + strings.Repeat("0", 64)
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "missing version",
			in:   `{"policy":"block","bundles":{}}`,
			want: "version",
		},
		{
			name: "missing policy",
			in:   `{"version":1,"bundles":{}}`,
			want: "policy",
		},
		{
			name: "missing bundles",
			in:   `{"version":1,"policy":"block"}`,
			want: "bundles",
		},
		{
			name: "null discovery",
			in:   `{"version":1,"discovery":null,"policy":"block","bundles":{}}`,
			want: "discovery",
		},
		{
			name: "null bundles",
			in:   `{"version":1,"policy":"block","bundles":null}`,
			want: "bundles",
		},
		{
			name: "missing bundle contentHash",
			in:   `{"version":1,"policy":"block","bundles":{"./a":{"files":{"x":"` + validHash + `"}}}}`,
			want: "contentHash",
		},
		{
			name: "null bundle contentHash",
			in:   `{"version":1,"policy":"block","bundles":{"./a":{"contentHash":null,"files":{"x":"` + validHash + `"}}}}`,
			want: "contentHash",
		},
		{
			name: "missing bundle files",
			in:   `{"version":1,"policy":"block","bundles":{"./a":{"contentHash":"` + validHash + `"}}}`,
			want: "files",
		},
		{
			name: "null bundle files",
			in:   `{"version":1,"policy":"block","bundles":{"./a":{"contentHash":"` + validHash + `","files":null}}}`,
			want: "files",
		},
		{
			name: "empty revision",
			in:   `{"version":1,"policy":"block","bundles":{"./a":{"revision":"","contentHash":"` + validHash + `","files":{"x":"` + validHash + `"}}}}`,
			want: "revision",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode([]byte(c.in))
			if err == nil {
				t.Fatal("expected decode error, got nil")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not mention %q", err, c.want)
			}
		})
	}
}

// TestDecode_DuplicateObjectNames verifies JSON duplicate object names are
// rejected before decoding into Go maps can silently apply last-writer-wins
// semantics. The bundle and files maps are integrity-bearing, so duplicates
// there must never collapse without a diagnostic.
func TestDecode_DuplicateObjectNames(t *testing.T) {
	validHash := "sha256:" + strings.Repeat("0", 64)
	validBundle := `"contentHash":"` + validHash + `","files":{"x":"` + validHash + `"}`
	cases := []struct {
		name string
		in   string
	}{
		{
			name: "duplicate bundle key",
			in: `{"version":1,"policy":"block","bundles":{` +
				`"./a":{` + validBundle + `},` +
				`"./a":{` + validBundle + `}` +
				`}}`,
		},
		{
			name: "duplicate files key",
			in: `{"version":1,"policy":"block","bundles":{"./a":{` +
				`"contentHash":"` + validHash + `",` +
				`"files":{"x":"` + validHash + `","x":"` + validHash + `"}}}}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode([]byte(c.in))
			if err == nil {
				t.Fatal("expected duplicate-key error, got nil")
			}
			if !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("error %q does not mention duplicate", err)
			}
		})
	}
}

// TestDecode_NotJSON verifies the decoder rejects malformed JSON cleanly
// rather than panicking. A panic at the lockfile boundary would crash the
// CLI before the user-facing 'invalid lockfile' diagnostic could appear.
//
// Note: "null" is intentionally NOT in this list - it is valid JSON but the
// top-level check has a dedicated test because accepting it would erase
// all required-field presence information.
func TestDecode_NotJSON(t *testing.T) {
	for _, in := range []string{"", "not json", "{", "[", "{\"version\":"} {
		t.Run(in, func(t *testing.T) {
			_, err := Decode([]byte(in))
			if err == nil {
				t.Fatalf("expected error for input %q, got nil", in)
			}
		})
	}
}

// TestDecode_TrailingGarbage verifies the decoder rejects content after the
// top-level JSON value. A lockfile with trailing data is suspicious (likely
// concatenation of two lockfiles, or smuggled bytes) and should fail closed
// rather than silently accept the first object.
func TestDecode_TrailingGarbage(t *testing.T) {
	in := `{"version":1,"policy":"block","bundles":{}} extra`
	_, err := Decode([]byte(in))
	if err == nil {
		t.Fatal("expected trailing-data error, got nil")
	}
}
