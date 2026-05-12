package cli

import (
	"errors"
	"path/filepath"
	"testing"
)

// TestBundleKeyToPath pins the translation.
// filesystem paths. The lockfile uses forward-slash, "./"-anchored keys
// everywhere; the OS wants absolute, native- separator paths. This helper is
// the single bridge.
//
// We deliberately use t.TempDir() as the cwd in every case so the expected
// path can be built with filepath.Join - that way the test is portable across
// Linux (/) and Windows (\).
func TestBundleKeyToPath(t *testing.T) {
	cwd := t.TempDir()

	cases := []struct {
		name string
		key  string
		want string
	}{
		// The canonical form. Every key the lockfile encoder emits looks like this.
		{
			name: "./-prefixed key",
			key:  "./skills/foo",
			want: filepath.Join(cwd, "skills", "foo"),
		},
		// Nested key with several path segments - verify we don't accidentally
		// collapse them.
		{
			name: "nested key",
			key:  "./a/b/c",
			want: filepath.Join(cwd, "a", "b", "c"),
		},
		// Defensive: a key WITHOUT the "./" prefix should still resolve. Validate
		// normally enforces the prefix, but we accept either form so this helper is
		// not a foot-gun.
		// internal callers that build keys themselves.
		{
			name: "key without ./ prefix",
			key:  "skills/foo",
			want: filepath.Join(cwd, "skills", "foo"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := bundleKeyToPath(cwd, c.key)
			if got != c.want {
				t.Fatalf("bundleKeyToPath(%q, %q) = %q, want %q",
					cwd, c.key, got, c.want)
			}
		})
	}
}

func TestBundleKeyToSafePathRejectsBareDot(t *testing.T) {
	cwd := t.TempDir()

	// The translator can still mechanically join "." to cwd, but safe
	// resolution is used for verify/pin hashing and must not allow project-root
	// bundle keys.
	_, err := bundleKeyToSafePath(cwd, ".")
	if err == nil {
		t.Fatal("bundleKeyToSafePath should reject exact dot bundle key")
	}
	if !errors.Is(err, errUnsupportedBundlePath) {
		t.Fatalf("bundleKeyToSafePath error = %v, want errUnsupportedBundlePath", err)
	}
}
