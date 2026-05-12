package seal

import (
	"fmt"
	"path"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// PatternCoversBundleKey reports whether pattern statically produces bundleKey
// as a discovery match. It uses the same segment grammar as ExpandPatterns
// but does not touch the filesystem.
func PatternCoversBundleKey(pattern, bundleKey string) bool {
	if err := ValidatePattern(pattern); err != nil {
		return false
	}
	if !strings.HasPrefix(bundleKey, "./") {
		return false
	}
	keyRel := strings.TrimPrefix(bundleKey, "./")
	if keyRel == "" {
		return false
	}
	keySegs := splitNFCSegments(keyRel)
	patSegs := splitNFCSegments(pattern)
	return patternCoversKeySegments(patSegs, keySegs)
}

// BundleKeyCoveredByDiscovery reports whether any discovery pattern statically
// produces bundleKey.
func BundleKeyCoveredByDiscovery(patterns []string, bundleKey string) bool {
	for _, p := range patterns {
		if PatternCoversBundleKey(p, bundleKey) {
			return true
		}
	}
	return false
}

// ExactDiscoveryPatternForBundleKey returns the narrowest discovery pattern
// that statically produces bundleKey: the bundle key without its "./" prefix.
func ExactDiscoveryPatternForBundleKey(bundleKey string) string {
	return strings.TrimPrefix(bundleKey, "./")
}

// DiscoveryCoverageError returns an actionable error when bundleKey is not
// statically produced by any pattern in discovery.
func DiscoveryCoverageError(discovery []string, bundleKey string) error {
	suggest := SuggestDiscoveryPattern(bundleKey)
	if len(discovery) == 0 {
		return fmt.Errorf(
			"bundle key %q is not statically produced by any discovery pattern (discovery is empty); add %q (or repin the containing directory as a single sealed root)",
			bundleKey, suggest,
		)
	}
	quoted := make([]string, len(discovery))
	for i, p := range discovery {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	return fmt.Errorf(
		"bundle key %q is not statically produced by any discovery pattern; existing patterns %s do not cover it; add %q (or repin the containing directory as a single sealed root)",
		bundleKey, strings.Join(quoted, ", "), suggest,
	)
}

// SuggestDiscoveryPattern returns an actionable discovery pattern for an
// uncovered bundle key. When the key names a file under a directory, it
// suggests a sibling-matching glob such as docs/*.md.
func SuggestDiscoveryPattern(bundleKey string) string {
	rel := ExactDiscoveryPatternForBundleKey(bundleKey)
	if rel == "" {
		return "<add a discovery pattern for this bundle>"
	}
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		parent := rel[:i]
		leaf := rel[i+1:]
		if j := strings.LastIndex(leaf, "."); j > 0 {
			return parent + "/*" + leaf[j:]
		}
		return parent + "/*"
	}
	return rel
}

func patternCoversKeySegments(patSegs, keySegs []string) bool {
	if len(patSegs) == 0 || len(keySegs) == 0 || len(patSegs) > len(keySegs) {
		return false
	}
	for i, pat := range patSegs {
		ok, err := matchPatternSegment(pat, keySegs[i])
		if err != nil || !ok {
			return false
		}
		if i == len(patSegs)-1 {
			return i == len(keySegs)-1
		}
	}
	return false
}

func matchPatternSegment(patSeg, name string) (bool, error) {
	return path.Match(escapeSpecLiterals(patSeg), name)
}

// escapeSpecLiterals escapes ?, [, ] so path.Match treats them as literals.
// Seal's discovery grammar uses only `*` as a wildcard; the other metachars
// path.Match supports have no place here, and escaping at the boundary lets
// us keep using path.Match instead of writing a parallel matcher.
func escapeSpecLiterals(p string) string {
	var b strings.Builder
	b.Grow(len(p))
	for _, r := range p {
		// A backslash + next-char tells path.Match "this char, literal".
		switch r {
		case '?', '[', ']':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func splitNFCSegments(p string) []string {
	raw := strings.Split(p, "/")
	out := make([]string, len(raw))
	for i, seg := range raw {
		out[i] = norm.NFC.String(seg)
	}
	return out
}
