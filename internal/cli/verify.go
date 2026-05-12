package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// VerifyOpts bundles the inputs RunVerify needs. Tests construct it inline
// with only the fields they care about; every zero value is meaningful.
type VerifyOpts struct {
	// Cwd: where seal.json is read from and where bundle keys resolve. Empty
	// Cwd is a caller bug - we don't default to "." so it's always visible
	// which side picked the value.
	Cwd string

	// Stderr receives human output and diagnostics.
	Stderr io.Writer

	// Stdout receives JSON output only when JSON==true. Exit-code-2 paths
	// must not write here.
	Stdout io.Writer

	// JSON emits machine-readable JSON to Stdout and suppresses the human
	// summary on Stderr.
	JSON bool

	// Quiet suppresses all output; the exit code is the only signal.
	Quiet bool

	// Verbose expands each Mismatch bundle into per-file modified/added/missing
	// lines in human output. No effect under JSON.
	Verbose bool
}

// RunVerify implements `seal verify`. Returns the desired process exit code.
//
// Exit code is an int (not error) because outcome=Blocked → exit 1 is a
// correct run, not a Go-style failure; encoding it as an int keeps error
// reserved for "something actually went wrong" (exit 2).
func RunVerify(opts VerifyOpts) int {
	lockPath, err := seal.FindLockfile(opts.Cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// User-actionable message for the common first-run case.
			fmt.Fprintln(opts.Stderr,
				"seal: verify: seal.json not found in current directory; run 'seal init' first")
		} else {
			fmt.Fprintf(opts.Stderr, "seal: verify: %v\n", err)
		}
		return 2
	}

	// ReadFile wraps decode + validate.
	lf, err := seal.ReadFile(lockPath)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "seal: verify: %v\n", err)
		return 2
	}

	onDisk, err := buildOnDiskMap(opts.Cwd, lf)
	if err != nil {
		// buildOnDiskMap only errors on systemic problems (unexpected stat
		// error, malformed discovery pattern past Validate). Surface and abort.
		fmt.Fprintf(opts.Stderr, "seal: verify: %v\n", err)
		return 2
	}

	states := seal.Classify(lf, onDisk)
	outcome := derivedOutcome(states, lf.Policy)

	// Render priority: Quiet > JSON > human. Never both JSON and human, or
	// tee'd logs double-count the result.
	switch {
	case opts.Quiet:
		// Exit code is the entire signal.
	case opts.JSON:
		if err := WriteVerifyJSON(opts.Stdout, outcome, states); err != nil {
			fmt.Fprintf(opts.Stderr, "seal: verify: %v\n", err)
			return 2
		}
	default:
		writeVerifyHuman(opts.Stderr, outcome, lf.Policy, states, opts.Verbose)
	}

	if outcome == "Blocked" {
		return 1
	}
	return 0
}