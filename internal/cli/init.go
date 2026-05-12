package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// InitOpts bundles the inputs RunInit needs. io.Reader/io.Writer fields make
// it test-drivable without touching real fds.
type InitOpts struct {
	// Cwd is where seal.json gets written and where heuristics scan.
	Cwd string

	// Stdin supplies the [y/N] answer. Tests pass strings.NewReader; main wires
	// os.Stdin. A nil Stdin in a non-TTY environment triggers the abort path.
	Stdin io.Reader

	// Stderr receives the summary, prompt, diagnostics, and success line. All
	// init output is on stderr so stdout stays clean for tooling.
	Stderr io.Writer

	// Warn switches the new lockfile's policy to "warn" (default "block").
	Warn bool

	// Verbose expands each bundle in the summary into a per-file listing.
	Verbose bool
}

// RunInit implements `seal init`. Returns the desired process exit code.
func RunInit(opts InitOpts) int {
	// Stat the canonical path (not FindLockfile) because here even an
	// unreadable file counts as "exists" - init must never clobber it just
	// because we can't parse it.
	lockPath := filepath.Join(opts.Cwd, seal.LockfileName)
	if _, err := os.Stat(lockPath); err == nil {
		fmt.Fprintf(opts.Stderr,
			"seal: init: %s already exists; init will not overwrite\n",
			lockPath)
		return 2
	} else if !errors.Is(err, os.ErrNotExist) {
		// Anything other than "file doesn't exist" (perms, I/O) surfaces as-is.
		fmt.Fprintf(opts.Stderr, "seal: init: %v\n", err)
		return 2
	}

	policy := "block"
	if opts.Warn {
		policy = "warn"
	}
	lf, err := buildInitLockfile(opts.Cwd, policy)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "seal: init: %v\n", err)
		return 2
	}

	writeInitSummary(opts.Stderr, lf, opts.Verbose)

	// Different wording when nothing was found, so the user understands
	// they're being asked to create an empty pin rather than apply a diff.
	prompt := "Apply these changes?"
	if len(lf.Discovery) == 0 && len(lf.Bundles) == 0 {
		prompt = "No agent bundles or known agent layouts detected in this project. Create empty seal.json with policy: " + policy + "?"
	}

	// If the caller supplied Stdin (test, scripted pipe), trust it. Otherwise
	// require a real TTY on both stdin AND stderr so the prompt is both
	// visible and answerable - fail closed to prevent a hung init on CI.
	if opts.Stdin == nil && !IsInteractive() {
		fmt.Fprintln(opts.Stderr,
			"seal: init: requires a TTY for confirmation; cannot bootstrap seal.json in a non-interactive environment")
		return 2
	}

	// Confirm appends the [y/N] suffix and applies the default-deny rule
	// (EOF / garbage / bare-Enter all count as "no").
	if !Confirm(opts.Stdin, opts.Stderr, prompt) {
		fmt.Fprintln(opts.Stderr, "seal: init aborted")
		return 2
	}

	// seal.WriteFile is atomic + flock-protected so a concurrent invocation
	// can't tear the file.
	if err := seal.WriteFile(lockPath, lf); err != nil {
		fmt.Fprintf(opts.Stderr, "seal: init: %v\n", err)
		return 2
	}

	// Surface the counts so a script piping init can confirm scope from
	// stderr alone without re-reading the lockfile.
	fmt.Fprintf(opts.Stderr,
		"Wrote seal.json (%d bundles, %d discovery patterns)\n",
		len(lf.Bundles), len(lf.Discovery))
	return 0
}