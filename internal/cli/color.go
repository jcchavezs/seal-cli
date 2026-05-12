package cli

import (
	"os"

	"golang.org/x/term"
)

// UseColor reports whether the CLI should write ANSI color codes to stderr.
// The actual policy lives in shouldUseColor; this wrapper just reads the
// process-level inputs so the policy can be unit-tested without a real TTY.
func UseColor() bool {
	noColor := os.Getenv("NO_COLOR")
	termVar := os.Getenv("TERM")
	// term.IsTerminal takes an int fd; os.Stderr.Fd returns uintptr.
	stderrIsTTY := term.IsTerminal(int(os.Stderr.Fd()))
	return shouldUseColor(noColor, termVar, stderrIsTTY)
}

// shouldUseColor is the pure policy function.
//
// Rules in priority order:
//  1. NO_COLOR set to any non-empty value disables color (https://no-color.org).
//     Presence is the signal - `NO_COLOR=0` must still disable, per spec.
//  2. TERM=dumb disables color; ANSI on a dumb terminal renders as literal
//     "ESC[31m" garbage.
//  3. Otherwise paint only if stderr is a real terminal; never pollute piped
//     or redirected output with escape sequences.
func shouldUseColor(noColor, termVar string, stderrIsTTY bool) bool {
	if noColor != "" {
		return false
	}
	if termVar == "dumb" {
		return false
	}
	return stderrIsTTY
}
