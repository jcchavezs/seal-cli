package cli

import (
	"os"

	"golang.org/x/term"
)

// IsInteractive reports whether it is safe to issue a Confirm prompt - i.e.
// whether stdin AND stderr are both TTYs.
//
// Both must be terminals:
//   - Stdin: we read the user's answer there. A pipe or /dev/null would hang
//     the prompt or silently default-deny in a context the operator never saw.
//   - Stderr: we WRITE the prompt there (stdout is reserved for JSON output,
//     e.g. `seal verify --json`). If stderr is redirected to a file, the user
//     never sees the question even if stdin is a real terminal.
//
// Returning false in either case is the safe default; callers can then exit
// with the non-interactive code (2) instead of pretending to prompt.
func IsInteractive() bool {
	// term.IsTerminal takes an int fd; os.Stdin.Fd / Stderr.Fd return uintptr.
	stdinIsTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stderrIsTTY := term.IsTerminal(int(os.Stderr.Fd()))
	return stdinIsTTY && stderrIsTTY
}
