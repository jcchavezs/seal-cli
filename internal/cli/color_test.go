package cli

import "testing"

// TestShouldUseColor pins the truth table for the inner predicate.
// We test the pure helper (not UseColor itself) because the env-var reads and
// FD lookups are not interesting to verify - the policy is. Splitting them
// this way means a test failure points to the policy bug directly, not to a
// fixture issue.
func TestShouldUseColor(t *testing.T) {
	cases := []struct {
		name        string
		noColor     string // value of NO_COLOR env var
		term        string // value of TERM env var
		stderrIsTTY bool
		want        bool
	}{
		// Happy path: real terminal, no overrides. The one case that returns true.
		// Without this we'd never paint output.
		{name: "tty + clean env", noColor: "", term: "xterm-256color", stderrIsTTY: true, want: true},

		// NO_COLOR set to anything (even "0") disables color. This is the
		// no-color.org convention: presence is what matters.
		{name: "NO_COLOR=1 wins over TTY", noColor: "1", term: "xterm", stderrIsTTY: true, want: false},
		{name: "NO_COLOR=0 still wins (presence not value)", noColor: "0", term: "xterm", stderrIsTTY: true, want: false},

		// TERM=dumb is the POSIX way of saying "I cannot render ANSI";
		// honour it even when stderr happens to be a tty.
		{name: "TERM=dumb wins over TTY", noColor: "", term: "dumb", stderrIsTTY: true, want: false},

		// Non-tty stderr (pipe, redirect, file) ⇒ no color, regardless of env.
		// Pipes don't render ANSI; emitting it just pollutes log files with escape
		// sequences.
		{name: "no tty disables color", noColor: "", term: "xterm", stderrIsTTY: false, want: false},

		// All disabling signals at once ⇒ false. Belt-and-suspenders.
		{name: "everything off", noColor: "1", term: "dumb", stderrIsTTY: false, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldUseColor(c.noColor, c.term, c.stderrIsTTY)
			if got != c.want {
				t.Fatalf("shouldUseColor(noColor=%q, term=%q, tty=%v) = %v, want %v",
					c.noColor, c.term, c.stderrIsTTY, got, c.want)
			}
		})
	}
}

// TestUseColor_UnderGoTest verifies the public entry point returns false
// inside `go test` - stderr is a pipe, so even if NO_COLOR and
// TERM are unfavourable to disabling color the tty check still flips it off.
// This is a smoke test: it proves the wiring from env+fd into shouldUseColor
// is intact without needing a fake TTY.
func TestUseColor_UnderGoTest(t *testing.T) {
	if UseColor() {
		t.Fatal("UseColor() returned true under go test; stderr is not a TTY here")
	}
}