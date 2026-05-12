package seal

import (
	"fmt"
	"strings"
)

// ValidatePattern enforces the discovery-pattern grammar. Returns nil on
// valid patterns; an error naming the violated rule otherwise.
//
// Called from three sites - lockfile validation (validate.go), glob
// expansion (expand.go), and init-time pattern proposal (heuristics.go) -
// so the grammar lives in exactly one place.
func ValidatePattern(p string) error {
	if p == "" {
		return fmt.Errorf("empty pattern")
	}
	// "**" recursive glob is explicitly forbidden in v1. Check first so the
	// error message is specific.
	if strings.Contains(p, "**") {
		return fmt.Errorf("** is not supported in v1")
	}
	// Patterns are project-root-relative without a leading "./" or "/".
	if strings.HasPrefix(p, "./") || strings.HasPrefix(p, "/") {
		return fmt.Errorf("must not start with \"./\" or \"/\"")
	}
	// Forbid backslash so patterns are unambiguous on Windows.
	if strings.Contains(p, "\\") {
		return fmt.Errorf("backslash not allowed")
	}
	// Each "/"-separated segment must be non-empty and not "." / "..".
	// Catches "a//b", "a/./b", "a/../b", and a trailing slash (Split yields
	// an empty trailing segment).
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid segment %q", seg)
		}
	}
	return nil
}