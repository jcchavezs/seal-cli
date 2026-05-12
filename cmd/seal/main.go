// Package main is the entry point for the seal CLI binary. It wires cobra
// subcommands and routes errors to stderr with exit codes.
//
// All real logic lives in run() so tests can drive the CLI without os.Exit.
// Subcommand RunE handlers return a typed exitCode error rather than calling
// os.Exit, preserving the single-site exit invariant.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/SamyGhannad/seal-cli/internal/cli"
)

// Version is overridden at release time via -ldflags="-X main.Version=$TAG".
var Version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

// exitCode carries a desired process exit code from a subcommand's RunE up to
// run(). Using a typed error fits cobra's RunE signature so we don't need a
// side-channel for "operation completed but exit non-zero" (e.g. `seal verify`
// returning 1 on Blocked).
type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit %d", int(e)) }

// run is the testable entry point; main() is the only os.Exit site.
//
// Exit code mapping:
//   - nil from Execute      → 0
//   - exitCode(n) returned  → n (handler already printed its diagnostic)
//   - any other error       → 2 with a "seal: <err>" line (cobra-internal:
//     unknown flag, missing subcommand, etc.)
func run(args []string) int {
	root := newRootCmd()
	root.SetArgs(args)

	// Cobra's default failure output (an "Error: ..." line plus the full usage
	// block) duplicates what our handlers already print. Silence both.
	root.SilenceUsage = true
	root.SilenceErrors = true

	err := root.Execute()
	if err == nil {
		return 0
	}

	var ec exitCode
	if errors.As(err, &ec) {
		return int(ec)
	}

	fmt.Fprintf(os.Stderr, "seal: %v\n", err)
	return 2
}

// newRootCmd assembles the top-level seal command and its subcommand tree.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "seal",
		Short:   "Seal v1 lockfile CLI",
		Long:    "seal manages the seal.json lockfile that records the trusted state of project-local agent bundles.",
		Version: Version,
	}
	root.AddCommand(newInitCmd(), newPinCmd(), newVerifyCmd())
	return root
}

func newInitCmd() *cobra.Command {
	var warn, verbose bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap seal.json in the current directory",
		Long:  "init proposes discovery patterns based on detected agent layouts, hashes the matched bundles, prints a summary, and on confirmation writes a fresh seal.json.",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				// Getwd can fail (deleted cwd, perms). Surface rather than letting
				// downstream code run with an empty Cwd.
				return fmt.Errorf("init: %w", err)
			}
			code := cli.RunInit(cli.InitOpts{
				Cwd:     cwd,
				Stdin:   os.Stdin,
				Stderr:  os.Stderr,
				Warn:    warn,
				Verbose: verbose,
			})
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&warn, "warn", false,
		"set policy to \"warn\" instead of the default \"block\"")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"show per-file detail in the summary")
	return cmd
}

// newPinCmd wires `seal pin`. Positional args pass through verbatim; RunPin
// chooses targeted vs bulk mode by checking len(Args).
func newPinCmd() *cobra.Command {
	var prune, verbose bool
	cmd := &cobra.Command{
		Use:   "pin [path...]",
		Short: "Pin or re-pin bundles into seal.json",
		Long:  "pin without args rebuilds bundle entries for every discovery match (bulk mode). With path args, pin replaces those entries specifically (targeted mode).",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("pin: %w", err)
			}
			code := cli.RunPin(cli.PinOpts{
				Cwd:     cwd,
				Stdin:   os.Stdin,
				Stderr:  os.Stderr,
				Args:    args,
				Prune:   prune,
				Verbose: verbose,
			})
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&prune, "prune", false,
		"(bulk only) drop lockfile entries no longer present on disk")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"show per-file detail in the summary")
	return cmd
}

// newVerifyCmd wires `seal verify`. --json and --quiet are mutually exclusive;
// we enforce that here (before any I/O) rather than inside cli.RunVerify.
func newVerifyCmd() *cobra.Command {
	var jsonOut, quiet, verbose bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify seal.json against the on-disk state",
		Long:  "verify reads seal.json, hashes every recorded bundle and every discovery match, and reports whether the project is Verified / Drift / Warning / Blocked.",
		RunE: func(c *cobra.Command, args []string) error {
			if jsonOut && quiet {
				fmt.Fprintln(os.Stderr,
					"seal: verify: --quiet and --json are mutually exclusive")
				return exitCode(2)
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("verify: %w", err)
			}
			code := cli.RunVerify(cli.VerifyOpts{
				Cwd:     cwd,
				Stderr:  os.Stderr,
				Stdout:  os.Stdout,
				JSON:    jsonOut,
				Quiet:   quiet,
				Verbose: verbose,
			})
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"emit machine-readable JSON to stdout")
	cmd.Flags().BoolVar(&quiet, "quiet", false,
		"no output; exit code is the only signal")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"show per-file detail in mismatches")
	return cmd
}
