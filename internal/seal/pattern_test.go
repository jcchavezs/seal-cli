package seal

import "testing"

// TestValidatePattern_Valid covers patterns that must be accepted.
// Each case represents a real-world shape the discovery feature should
// support: simple "*"-leaf, multi-segment, "*"-mid-segment, and explicit rule
// that ?, [, ] are LITERAL inside a segment (not wildcards).
func TestValidatePattern_Valid(t *testing.T) {
	for _, p := range []string{
		".claude/skills/*",   // single "*" leaf (the common case)
		"plugins/*",          // top-level glob
		"a/b/c/*",            // deep nesting
		"a*b/c",              // "*" mid-segment matches zero+ chars
		"foo?bar/baz",        // "?" is literal
		"foo[bar]/baz",       // "[", "]" are literal
		"foo{a,b}/baz",       // "{", "}" are literal
		".hidden/*",          // hidden parent directory
	} {
		if err := ValidatePattern(p); err != nil {
			t.Errorf("ValidatePattern(%q): unexpected error %v", p, err)
		}
	}
}

// TestValidatePattern_Invalid covers each . Each case names the rule it
// violates so future readers can map a failure back to the.
func TestValidatePattern_Invalid(t *testing.T) {
	for _, c := range []struct {
		name    string
		pattern string
	}{
		{"empty", ""},
		{"** is unsupported in v1", "**"},
		{"** anywhere in pattern", "a/**"},
		{"** as suffix", "a/b/**"},
		{"leading ./", "./skills/*"},
		{"leading /", "/skills/*"},
		{"backslash", "skills\\foo"},
		{"intermediate . segment", "a/./b"},
		{"intermediate .. segment", "a/../b"},
		{"repeated slash", "a//b"},
		{"trailing slash", "a/"},
		{"only slash", "/"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidatePattern(c.pattern); err == nil {
				t.Errorf("ValidatePattern(%q): expected error, got nil", c.pattern)
			}
		})
	}
}