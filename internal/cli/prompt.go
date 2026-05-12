// Package cli holds per-subcommand implementations and shared UI helpers
// (TTY detection, prompts, output writers). Code here is allowed to know it
// is running inside a terminal; the internal/seal package is pure domain
// logic and must not touch stdin/stderr.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Confirm asks a y/N question on w, reads one line from r, and returns true
// only for "y" or "yes" (case-insensitive). The single primitive used by
// `seal pin` and `seal init`; there is deliberately no --yes flag, so the
// prompt cannot be bypassed interactively.
//
// Default-deny on bare Enter, EOF, or garbage: pin and init are
// destructive-by-default (they replace previously-trusted bytes), and the
// "[y/N]" with capital N follows the POSIX convention that the uppercase
// letter is the default. Failing closed also makes
// `seal pin < /dev/null` safe.
//
// r and w are parameters (not os.Stdin / os.Stderr) so tests can inject
// buffers without touching the real terminal.
func Confirm(r io.Reader, w io.Writer, prompt string) bool {
	fmt.Fprintf(w, "%s [y/N] ", prompt)

	// bufio.Scanner.Scan returns false on EOF or any read error; treating
	// both as "no" keeps the failure mode uniform.
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		return false
	}

	answer := strings.ToLower(strings.TrimSpace(sc.Text()))
	return answer == "y" || answer == "yes"
}