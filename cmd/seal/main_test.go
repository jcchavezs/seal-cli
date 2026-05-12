package main

import "testing"

// TestRun_VersionFlag exercises the wiring end-to-end: cobra parses
// `--version`, prints to stdout, run() returns 0. We don't capture stdout
// here - cobra writes the version even when its
// SetOut isn't set, and asserting on stdout would require swapping os.Stdout
// which fights other goroutines in the test binary.
// Exit-code-only is enough for this smoke test; richer output coverage lives
// in the package-level cli tests.
func TestRun_VersionFlag(t *testing.T) {
	if got := run([]string{"--version"}); got != 0 {
		t.Errorf("run([--version]) = %d, want 0", got)
	}
}

// TestRun_HelpFlag covers cobra's --help path. Same rationale as above - exit
// code only.
func TestRun_HelpFlag(t *testing.T) {
	if got := run([]string{"--help"}); got != 0 {
		t.Errorf("run([--help]) = %d, want 0", got)
	}
}

// TestRun_UnknownCommandIsFatal verifies the cobra-error branch in run(): an
// unknown subcommand triggers cobra's RunE, which returns a non-nil error
// that does NOT carry an exitCode. Our code translates that into exit 2 and
// prints "seal: ...".
// stderr. We just check the exit code here; the behaviour of the actual cli
// handlers is covered by the internal/cli tests.
func TestRun_UnknownCommandIsFatal(t *testing.T) {
	if got := run([]string{"nope-not-a-real-subcommand"}); got != 2 {
		t.Errorf("run([nope]) = %d, want 2", got)
	}
}

// TestRun_VerifyMutuallyExclusiveFlags pins the mutual-exclusion guard we
// added in newVerifyCmd: --json and --quiet together should surface as exit 2
// BEFORE touching the filesystem (no seal.json needed for this test to be
// meaningful).
func TestRun_VerifyMutuallyExclusiveFlags(t *testing.T) {
	got := run([]string{"verify", "--json", "--quiet"})
	if got != 2 {
		t.Errorf("run([verify --json --quiet]) = %d, want 2", got)
	}
}
