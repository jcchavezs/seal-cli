package seal

import "testing"

func TestPatternCoversBundleKey(t *testing.T) {
	for _, c := range []struct {
		pattern string
		key     string
		want    bool
	}{
		{".claude/skills/*", "./.claude/skills/foo", true},
		{"skills/*", "./skills/foo", true},
		{"skills/*", "./skills/m3.md", true},
		{"skills", "./skills", true},
		{"skills", "./skills/foo", false},
		{"docs/*", "./docs/contributor-prompt.md", true},
		{"docs/*", "./docs/readme.txt", true},
		{".dirA/*", "./.dirB/2", false},
	} {
		t.Run(c.pattern+"_"+c.key, func(t *testing.T) {
			got := PatternCoversBundleKey(c.pattern, c.key)
			if got != c.want {
				t.Fatalf("PatternCoversBundleKey(%q, %q) = %v, want %v", c.pattern, c.key, got, c.want)
			}
		})
	}
}
