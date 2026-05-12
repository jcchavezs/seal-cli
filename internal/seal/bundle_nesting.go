package seal

import "strings"

// BundleKeyIsStrictAncestor reports whether ancestor is a directory-segment
// ancestor of descendant. The comparison is segment-aware: ./skills is an
// ancestor of ./skills/foo, but ./skill is not an ancestor of ./skills.
//
// When bidirectional is true, the check runs in both directions so either key
// may be the ancestor argument.
func BundleKeyIsStrictAncestor(ancestor, descendant string, bidirectional bool) bool {
	if bundleKeyIsStrictAncestorOneWay(ancestor, descendant) {
		return true
	}
	if bidirectional {
		return bundleKeyIsStrictAncestorOneWay(descendant, ancestor)
	}
	return false
}

// bundleKeyIsStrictAncestorOneWay compares valid bundle-key shapes only (./…).
// Invalid keys such as the exact "." project-root key return false; Validate
// rejects those before lockfile nesting checks run.
func bundleKeyIsStrictAncestorOneWay(ancestor, descendant string) bool {
	if ancestor == descendant {
		return false
	}
	if !strings.HasPrefix(ancestor, "./") || !strings.HasPrefix(descendant, "./") {
		return false
	}
	aRest := strings.TrimPrefix(ancestor, "./")
	dRest := strings.TrimPrefix(descendant, "./")
	if aRest == "" || dRest == "" {
		return false
	}
	if dRest == aRest {
		return false
	}
	if strings.HasSuffix(dRest, "/") || strings.Contains(dRest, "//") {
		return false
	}
	prefix := aRest + "/"
	if !strings.HasPrefix(dRest, prefix) {
		return false
	}
	return strings.TrimPrefix(dRest, prefix) != ""
}

// DiscoveryPatternsProduceNestedRoots reports whether two discovery patterns
// express incompatible nested sealed-root boundaries (for example skills and
// skills/*).
func DiscoveryPatternsProduceNestedRoots(p1, p2 string) bool {
	for _, k1 := range representativeBundleKeysForPattern(p1) {
		for _, k2 := range representativeBundleKeysForPattern(p2) {
			if BundleKeyIsStrictAncestor(k1, k2, true) {
				return true
			}
		}
		if patternCoversStrictDescendantOf(p2, k1) {
			return true
		}
	}
	for _, k2 := range representativeBundleKeysForPattern(p2) {
		if patternCoversStrictDescendantOf(p1, k2) {
			return true
		}
	}
	if discoveryPatternsOverlapViaLiteralPrefix(p1, p2) || discoveryPatternsOverlapViaLiteralPrefix(p2, p1) {
		return true
	}
	return false
}

// discoveryPatternsOverlapViaLiteralPrefix detects when a wildcard pattern can
// produce the literal path prefix of another pattern (for example skills/* can
// produce ./skills/foo while skills/foo/* matches below that root).
func discoveryPatternsOverlapViaLiteralPrefix(pWildcard, pLiteral string) bool {
	prefix := literalPrefixThroughFirstWildcard(pLiteral)
	if prefix == "" {
		return false
	}
	key := "./" + prefix
	if !PatternCoversBundleKey(pWildcard, key) {
		return false
	}
	return patternCoversStrictDescendantOf(pLiteral, key)
}

// literalPrefixThroughFirstWildcard returns the slash-separated path through
// the first segment that contains *, exclusive (for skills/foo/* → skills/foo).
// Patterns with no wildcard return the full pattern.
func literalPrefixThroughFirstWildcard(pattern string) string {
	if err := ValidatePattern(pattern); err != nil {
		return ""
	}
	segs := strings.Split(pattern, "/")
	var literal []string
	for _, seg := range segs {
		if strings.Contains(seg, "*") {
			break
		}
		literal = append(literal, seg)
	}
	return strings.Join(literal, "/")
}

func representativeBundleKeysForPattern(p string) []string {
	if err := ValidatePattern(p); err != nil {
		return nil
	}
	if !strings.Contains(p, "*") {
		return []string{"./" + p}
	}
	segs := strings.Split(p, "/")
	var literal []string
	for _, seg := range segs {
		if strings.Contains(seg, "*") {
			break
		}
		literal = append(literal, seg)
	}
	if len(literal) == 0 {
		return nil
	}
	base := "./" + strings.Join(literal, "/")
	var keys []string
	for _, seg := range segs[len(literal):] {
		if !strings.Contains(seg, "*") {
			continue
		}
		keys = append(keys, base+"/"+sampleNameForPatternSegment(seg))
	}
	if len(keys) == 0 {
		keys = []string{base + "/probe"}
	}
	return keys
}

// sampleNameForPatternSegment returns a concrete path segment that matches
// patSeg under Seal's discovery grammar (path.Match with * as the only wildcard).
func sampleNameForPatternSegment(patSeg string) string {
	if !strings.Contains(patSeg, "*") {
		return patSeg
	}
	if i := strings.Index(patSeg, "*"); i >= 0 {
		if suffix := patSeg[i+1:]; suffix != "" {
			return "probe" + suffix
		}
	}
	return "probe"
}

// strictDescendantProbeKeysForPattern builds bundle keys that are strict
// descendants of ancestorKey and are shaped to match pattern's trailing segments.
func strictDescendantProbeKeysForPattern(pattern, ancestorKey string) []string {
	if err := ValidatePattern(pattern); err != nil {
		return nil
	}
	if !strings.HasPrefix(ancestorKey, "./") {
		return nil
	}
	ancestorSegs := splitNFCSegments(strings.TrimPrefix(ancestorKey, "./"))
	patSegs := splitNFCSegments(pattern)
	if len(patSegs) <= len(ancestorSegs) {
		return nil
	}
	for i := range ancestorSegs {
		ok, err := matchPatternSegment(patSegs[i], ancestorSegs[i])
		if err != nil || !ok {
			return nil
		}
	}
	ext := make([]string, len(patSegs)-len(ancestorSegs))
	for i, patSeg := range patSegs[len(ancestorSegs):] {
		ext[i] = sampleNameForPatternSegment(patSeg)
	}
	rel := strings.Join(append(ancestorSegs, ext...), "/")
	// Confirm the sample extension actually matches the pattern tail.
	key := "./" + rel
	if !PatternCoversBundleKey(pattern, key) {
		return nil
	}
	return []string{key}
}

func patternCoversStrictDescendantOf(pattern, ancestorKey string) bool {
	for _, key := range strictDescendantProbeKeysForPattern(pattern, ancestorKey) {
		if bundleKeyIsStrictAncestorOneWay(ancestorKey, key) {
			return true
		}
	}
	return false
}
