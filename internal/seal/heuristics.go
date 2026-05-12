package seal

import (
	"os"
	"path/filepath"
	"strings"
)

// Entry describes one well-known agent layout. Adding {Name, Pattern} is
// the only change required to support a new layout at `seal init` time.
type Entry struct {
	// Name is human-readable, shown in init to explain why a pattern was
	// proposed (e.g. "Claude Code skills").
	Name string

	// Pattern is a discovery glob; MUST pass ValidatePattern.
	// TestRegistry_AllPatternsValid pins this for every shipped entry.
	Pattern string
}

// Registry is the set of known layouts. Consulted ONLY by `seal init` -
// verification is driven solely by the lockfile's discovery array so that
// CI stays deterministic across implementations. Do NOT import from
// RunVerify or RunPin.
// Registry entries fall into two pattern shapes:
//
//   - `dir/*` - each child directory is its own bundle (e.g. .claude/skills/*
//     where every skill is a folder containing SKILL.md plus extras).
//   - `dir`   - the directory itself is one bundle covering all files inside
//     (e.g. .cursor/rules - a flat collection of .mdc files).
var Registry = []Entry{
	{Name: "Agent Skills", Pattern: ".agents/skills/*"},
	{Name: "Amazon Q agents", Pattern: ".amazonq/agents/*"},
	{Name: "Amazon Q rules", Pattern: ".amazonq/rules"},
	{Name: "Amp agents", Pattern: ".amp/agents/*"},
	{Name: "Amp skills", Pattern: ".amp/skills/*"},
	{Name: "Claude Code agents", Pattern: ".claude/agents/*"},
	{Name: "Claude Code commands", Pattern: ".claude/commands/*"},
	{Name: "Claude Code hooks", Pattern: ".claude/hooks"},
	{Name: "Claude Code plugins", Pattern: ".claude/plugins/*"},
	{Name: "Claude Code skills", Pattern: ".claude/skills/*"},
	{Name: "Codex skills", Pattern: ".codex/skills/*"},
	{Name: "Cursor rules", Pattern: ".cursor/rules"},
	{Name: "Cursor skills", Pattern: ".cursor/skills/*"},
	{Name: "Gemini CLI skills", Pattern: ".gemini/skills/*"},
	{Name: "GitHub Copilot instructions", Pattern: ".github/instructions"},
	{Name: "GitHub Copilot prompts", Pattern: ".github/prompts"},
	{Name: "GitHub Copilot skills", Pattern: ".github/skills/*"},
	{Name: "Kiro agents", Pattern: ".kiro/agents/*"},
	{Name: "Kiro skills", Pattern: ".kiro/skills/*"},
	{Name: "Kiro specs", Pattern: ".kiro/specs/*"},
	{Name: "Kiro steering", Pattern: ".kiro/steering"},
	{Name: "OpenClaw skills", Pattern: ".openclaw/skills/*"},
	{Name: "OpenCode agents", Pattern: ".opencode/agents/*"},
	{Name: "OpenCode commands", Pattern: ".opencode/commands/*"},
	{Name: "OpenCode modes", Pattern: ".opencode/modes/*"},
	{Name: "OpenCode plugins", Pattern: ".opencode/plugins/*"},
	{Name: "OpenCode skills", Pattern: ".opencode/skills/*"},
	{Name: "OpenCode tools", Pattern: ".opencode/tools/*"},
	{Name: "Windsurf rules", Pattern: ".windsurf/rules"},
	{Name: "Windsurf skills", Pattern: ".windsurf/skills/*"},
}

// ProposeFor returns the subset of Registry patterns whose parent directory
// actually exists under projectRoot. Keeps init noise-free: a fresh Go
// project doesn't get proposed Python-tool layouts.
//
// Output order is Registry declaration order, not lexicographic, so init's
// UI is consistent across invocations.
func ProposeFor(projectRoot string) []string {
	var out []string
	for _, e := range Registry {
		probe := patternProbe(e.Pattern)
		if probe == "" {
			// A pattern with no usable filesystem probe has no layout to gate on.
			// Current registry has none such; skip conservatively rather than
			// auto-propose.
			continue
		}
		full := filepath.Join(projectRoot, probe)
		info, err := os.Stat(full)
		// Require directory: a regular file at the parent path isn't a layout
		// we should match.
		if err == nil && info.IsDir() {
			out = append(out, e.Pattern)
		}
	}
	return out
}

// patternProbe returns the directory whose existence should cause a Registry
// entry to be proposed. For wildcard-leaf patterns (`dir/*`), probe `dir`.
// For directory-as-bundle patterns (`dir`), probe `dir` itself rather than its
// parent; otherwise `.claude/skills` would accidentally propose `.claude/hooks`.
func patternProbe(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return ""
	}
	leaf := p[idx+1:]
	if strings.Contains(leaf, "*") {
		return p[:idx]
	}
	return p
}

// patternParent strips the trailing "/<segment>" from a pattern.
//
//	".claude/skills/*" → ".claude/skills"
//	"a/b/c"            → "a/b"
//	"top-level"        → ""
//
// Only used at init time; we deliberately don't handle `*` in non-leaf
// segments because no current Registry entry needs it.
func patternParent(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return ""
	}
	return p[:idx]
}
