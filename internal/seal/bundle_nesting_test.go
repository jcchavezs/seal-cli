package seal

import "testing"

func TestBundleKeyIsStrictAncestorOneWay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ancestor   string
		descendant string
		want       bool
	}{
		// Direct and deep nesting (true).
		{name: "direct child", ancestor: "./skills", descendant: "./skills/foo", want: true},
		{name: "two-level descendant", ancestor: "./skills", descendant: "./skills/foo/bar", want: true},
		{name: "ancestor is intermediate directory", ancestor: "./skills/foo", descendant: "./skills/foo/bar/baz", want: true},
		{name: "dotted path segments", ancestor: "./.claude/skills", descendant: "./.claude/skills/foo", want: true},
		{name: "hidden leading segment", ancestor: "./.dirA", descendant: "./.dirA/2", want: true},
		{name: "file-like leaf segment", ancestor: "./docs", descendant: "./docs/contributor-prompt.md", want: true},
		{name: "deep tree", ancestor: "./a", descendant: "./a/b/c/d/e", want: true},

		// Equal keys and same path rest (false).
		{name: "identical keys", ancestor: "./skills/foo", descendant: "./skills/foo", want: false},
		{name: "same path different prefix form not used", ancestor: "./skills", descendant: "./skills", want: false},

		// Segment-boundary false positives (false) — string prefix without "/" boundary.
		{name: "prefix segment not ancestor", ancestor: "./skill", descendant: "./skills", want: false},
		{name: "prefix segment not ancestor reversed", ancestor: "./skill", descendant: "./skills/foo", want: false},
		{name: "concatenated segment name", ancestor: "./skills", descendant: "./skillsfoo", want: false},
		{name: "concatenated with further path", ancestor: "./skills", descendant: "./skillsfoo/bar", want: false},
		{name: "partial segment overlap mid-path", ancestor: "./skills/foo", descendant: "./skills/food", want: false},
		{name: "cousin paths share parent only", ancestor: "./skills/foo", descendant: "./skills/bar", want: false},

		// Wrong direction for one-way (false).
		{name: "child is not ancestor of parent", ancestor: "./skills/foo", descendant: "./skills", want: false},
		{name: "deeper key cannot ancestor shallow sibling branch", ancestor: "./a/b/c", descendant: "./a/b", want: false},

		// Exact "." bundle keys are invalid in Seal v1; nesting helper returns false.
		{name: "invalid dot ancestor", ancestor: ".", descendant: "./skills", want: false},
		{name: "invalid dot descendant", ancestor: "./skills", descendant: ".", want: false},
		{name: "both invalid dot keys", ancestor: ".", descendant: ".", want: false},

		// Missing or invalid "./" prefix (false).
		{name: "ancestor without dot slash", ancestor: "skills", descendant: "./skills/foo", want: false},
		{name: "descendant without dot slash", ancestor: "./skills", descendant: "skills/foo", want: false},
		{name: "both without dot slash", ancestor: "skills", descendant: "skills/foo", want: false},
		{name: "absolute-style ancestor", ancestor: "/skills", descendant: "./skills/foo", want: false},
		{name: "absolute-style descendant", ancestor: "./skills", descendant: "/skills/foo", want: false},
		{name: "empty ancestor", ancestor: "", descendant: "./skills", want: false},
		{name: "empty descendant", ancestor: "./skills", descendant: "", want: false},
		{name: "both empty", ancestor: "", descendant: "", want: false},

		// Empty path after "./" (false).
		{name: "ancestor is only dot slash", ancestor: "./", descendant: "./skills", want: false},
		{name: "descendant is only dot slash", ancestor: "./skills", descendant: "./", want: false},
		{name: "both only dot slash", ancestor: "./", descendant: "./", want: false},

		// Trailing-slash keys are not valid bundle keys but one-way must not
		// treat "./skills/" as ancestor of "./skills/foo" via sloppy prefix match.
		{name: "trailing slash on ancestor not segment-safe", ancestor: "./skills/", descendant: "./skills/foo", want: false},
		{name: "trailing slash on descendant", ancestor: "./skills", descendant: "./skills/foo/", want: false},

		// Single-segment roots.
		{name: "single segment ancestor", ancestor: "./x", descendant: "./x/y", want: true},
		{name: "single segment not matching", ancestor: "./x", descendant: "./y", want: false},

		// Unicode / NFC: comparison is raw bytes (document current behavior).
		{name: "byte-distinct NFC pair not treated as equal", ancestor: "./café", descendant: "./café/x", want: true},
		{name: "NFD ancestor vs NFC descendant same spelling", ancestor: "./cafe\u0301", descendant: "./caf\u00e9/x", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := bundleKeyIsStrictAncestorOneWay(tc.ancestor, tc.descendant)
			if got != tc.want {
				t.Fatalf("bundleKeyIsStrictAncestorOneWay(%q, %q) = %v, want %v",
					tc.ancestor, tc.descendant, got, tc.want)
			}
		})
	}
}

// TestBundleKeyIsStrictAncestorOneWay_SymmetryDocumentsDirection ensures the
// helper is genuinely directional: swapping arguments flips the result for
// nested pairs and stays false for unrelated pairs.
func TestBundleKeyIsStrictAncestorOneWay_SymmetryDocumentsDirection(t *testing.T) {
	t.Parallel()

	nested := []struct{ ancestor, descendant string }{
		{"./skills", "./skills/foo"},
		{"./a", "./a/b/c"},
	}
	for _, p := range nested {
		if !bundleKeyIsStrictAncestorOneWay(p.ancestor, p.descendant) {
			t.Fatalf("expected %q to ancestor %q", p.descendant, p.ancestor)
		}
		if bundleKeyIsStrictAncestorOneWay(p.descendant, p.ancestor) {
			t.Fatalf("expected %q not to ancestor %q", p.descendant, p.ancestor)
		}
	}

	unrelated := []struct{ a, b string }{
		{"./skills/foo", "./skills/bar"},
		{"./skill", "./skills"},
	}
	for _, p := range unrelated {
		if bundleKeyIsStrictAncestorOneWay(p.a, p.b) {
			t.Fatalf("expected no nesting between %q and %q", p.a, p.b)
		}
		if bundleKeyIsStrictAncestorOneWay(p.b, p.a) {
			t.Fatalf("expected no nesting between %q and %q", p.b, p.a)
		}
	}
}

func TestBundleKeyIsStrictAncestor(t *testing.T) {
	for _, c := range []struct {
		ancestor, descendant string
		bidirectional          bool
		want                 bool
	}{
		{"./skills", "./skills/foo", false, true},
		{"./skill", "./skills", false, false},
		{"./skills", "./skills", false, false},
		{"./skills", "./skillsfoo", false, false},
		{"./.claude/skills", "./.claude/skills/foo", false, true},
		{"./skills/foo", "./skills", true, true},
		{"./skill", "./skills", true, false},
	} {
		got := BundleKeyIsStrictAncestor(c.ancestor, c.descendant, c.bidirectional)
		if got != c.want {
			t.Fatalf("BundleKeyIsStrictAncestor(%q, %q, %v) = %v, want %v",
				c.ancestor, c.descendant, c.bidirectional, got, c.want)
		}
	}
}

func TestSampleNameForPatternSegment(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		patSeg, want string
	}{
		{"*", "probe"},
		{"*.md", "probe.md"},
		{"foo", "foo"},
		{"SKILL.md", "SKILL.md"},
		{"*skill", "probeskill"},
	} {
		got := sampleNameForPatternSegment(c.patSeg)
		if got != c.want {
			t.Fatalf("sampleNameForPatternSegment(%q) = %q, want %q", c.patSeg, got, c.want)
		}
	}
}

func TestStrictDescendantProbeKeysForPattern(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		pattern, ancestor string
		want            []string
	}{
		{
			pattern:  "skills/*",
			ancestor: "./skills",
			want:     []string{"./skills/probe"},
		},
		{
			pattern:  "docs/*.md",
			ancestor: "./docs",
			want:     []string{"./docs/probe.md"},
		},
		{
			pattern:  "skills/foo/*",
			ancestor: "./skills",
			want:     []string{"./skills/foo/probe"},
		},
		{
			pattern:  "docs/*",
			ancestor: "./skills",
			want:     nil,
		},
		{
			pattern:  "skills/*",
			ancestor: "./skills/foo",
			want:     nil,
		},
		{
			pattern:  "skills",
			ancestor: "./skills",
			want:     nil,
		},
	} {
		got := strictDescendantProbeKeysForPattern(c.pattern, c.ancestor)
		if len(got) != len(c.want) {
			t.Fatalf("strictDescendantProbeKeysForPattern(%q, %q) = %v, want %v",
				c.pattern, c.ancestor, got, c.want)
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Fatalf("strictDescendantProbeKeysForPattern(%q, %q) = %v, want %v",
					c.pattern, c.ancestor, got, c.want)
			}
		}
	}
}

func TestDiscoveryPatternsProduceNestedRoots(t *testing.T) {
	for _, c := range []struct {
		p1, p2 string
		want   bool
	}{
		{"skills", "skills/*", true},
		{"skills/*", "skills/foo/*", true},
		{"skills/foo/*", "skills/*", true},
		{"skills/*", "docs/*", false},
		{".claude/skills/*", ".dirB/*", false},
	} {
		got := DiscoveryPatternsProduceNestedRoots(c.p1, c.p2)
		if got != c.want {
			t.Fatalf("DiscoveryPatternsProduceNestedRoots(%q, %q) = %v, want %v", c.p1, c.p2, got, c.want)
		}
	}
}
