package cli

import "testing"

// TestIsInteractive_UnderGoTest verifies the negative path: when running
// inside `go test`, neither os.Stdin nor os.Stderr is a terminal (they're
// pipes connected to the test runner). So
// IsInteractive must return false.
//
// This is a small but load-bearing assertion: it proves the function does not
// crash on non-TTY file descriptors (a regression risk if someone replaces
// term.IsTerminal with a less robust check) and pins the "default to
// non-interactive in CI" behavior the CLI relies on for its automation
// contract.
func TestIsInteractive_UnderGoTest(t *testing.T) {
	if IsInteractive() {
		// If this ever fires, something has rewired stdin/stderr.
		// real TTYs in the test harness - either a bug here or.
		// our test invocation environment.
		t.Fatal("IsInteractive() returned true inside go test; expected false")
	}
}