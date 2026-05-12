package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// PinOpts groups the inputs RunPin needs.
type PinOpts struct {
	// Cwd is where seal.json lives and where path args resolve.
	Cwd string

	// Args are the positional path arguments. Empty ⇒ bulk mode (re-pin
	// everything Discovery matches). Non-empty ⇒ targeted mode.
	Args []string

	// Stdin supplies the [y/N] answer. Same TTY-or-Stdin contract as init.
	Stdin io.Reader

	// Stderr receives summary, prompt, diagnostics, success line.
	Stderr io.Writer

	// Prune (bulk only): drop lockfile entries whose directories have
	// vanished. Without it, removed entries appear in the summary but the
	// lockfile keeps them.
	Prune bool

	// Verbose surfaces per-file modified/added/missing detail in the summary.
	Verbose bool
}

// RunPin dispatches to bulk or targeted mode based on whether positional args
// were supplied. Returns the desired process exit code.
func RunPin(opts PinOpts) int {
	if len(opts.Args) == 0 {
		return runPinBulk(opts)
	}
	return runPinTargeted(opts)
}

// runPinTargeted implements `seal pin <path>...`.
//
// If every target classifies as unchanged we short-circuit with "No changes"
// - no prompt, no write. Any change requires Confirm; on "no" we exit 2
// without touching the lockfile.
func runPinTargeted(opts PinOpts) int {
	// Pin must not bootstrap a lockfile - that is init's job.
	lockPath, err := seal.FindLockfile(opts.Cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(opts.Stderr,
				"seal: pin: seal.json not found; run 'seal init' first")
		} else {
			fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
		}
		return 2
	}
	lf, err := seal.ReadFile(lockPath)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
		return 2
	}

	// Resolve every user path. Any failure (outside cwd, case mismatch,
	// missing, non-dir) aborts the whole operation - we never partial-apply,
	// so a typo doesn't half-write.
	keys := make([]string, 0, len(opts.Args))
	for _, arg := range opts.Args {
		key, err := resolvePathToBundleKey(opts.Cwd, arg)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
			return 2
		}
		keys = append(keys, key)
	}

	for _, key := range keys {
		if err := rejectNestedPinTarget(lf.Bundles, key); err != nil {
			fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
			return 2
		}
	}

	// Hash each resolved path before any classification so an unreadable
	// bundle aborts cleanly with no in-memory mutations to roll back.
	fresh := make(map[string]map[string]string, len(keys))
	for _, key := range keys {
		full, err := bundleKeyToSafePath(opts.Cwd, key)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
			return 2
		}
		files, _, err := seal.HashSealedRoot(full)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
			return 2
		}
		fresh[key] = files
	}

	// Classify each target and build the per-row data for the summary.
	targets := make([]pinTarget, 0, len(keys))
	var changes int
	for _, key := range keys {
		existing, has := lf.Bundles[key]
		kind := classifyPinTarget(existing, has, fresh[key])
		targets = append(targets, pinTarget{
			Key:       key,
			Kind:      kind,
			FileCount: len(fresh[key]),
		})
		if kind != pinUnchanged {
			changes++
		}
	}

	// No-op short-circuit - CI scripts grep "No changes".
	if changes == 0 {
		fmt.Fprintln(opts.Stderr, "No changes")
		return 0
	}

	writePinSummary(opts.Stderr, targets)

	// TTY guard mirrors init: if no Stdin was supplied AND we're not on a
	// TTY, refuse rather than hang.
	if opts.Stdin == nil && !IsInteractive() {
		fmt.Fprintln(opts.Stderr,
			"seal: pin: changes require confirmation but no TTY available")
		return 2
	}
	if !Confirm(opts.Stdin, opts.Stderr, "Apply these pin changes?") {
		fmt.Fprintln(opts.Stderr, "pin aborted")
		return 2
	}

	// Targeted mode mutates only the user-named keys; other recorded bundles
	// are left untouched.
	if lf.Bundles == nil {
		lf.Bundles = make(map[string]seal.Bundle, len(keys))
	}
	for _, key := range keys {
		files := fresh[key]
		// Recompute ContentHash unconditionally - even for "unchanged" entries
		// the user happened to include the bytes round-trip identically.
		ch := seal.ContentHash(files)
		lf.Bundles[key] = seal.Bundle{
			ContentHash: ch,
			Files:       files,
		}
		if !seal.BundleKeyCoveredByDiscovery(lf.Discovery, key) {
			lf.Discovery = appendDiscoveryPattern(lf.Discovery, seal.ExactDiscoveryPatternForBundleKey(key))
		}
	}

	// Atomic + flock-protected write; on failure the rename atomicity means
	// the worst case is "no change", never a torn file.
	if err := seal.WriteFile(lockPath, lf); err != nil {
		fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
		return 2
	}

	fmt.Fprintf(opts.Stderr, "Pinned %d bundle(s)\n", changes)
	return 0
}

// runPinBulk implements `seal pin` with no positional args: re-pin every
// candidate (discovery matches ∪ already-recorded keys), classify each as
// new / unchanged / modified / removed, and apply with --prune deciding
// whether removed entries are dropped or merely reported.
//
// Targeted and bulk share the same lockfile load, hashing, classification,
// summary, Confirm, and write helpers; only candidate-set construction and
// the prune branch differ.
func runPinBulk(opts PinOpts) int {
	lockPath, err := seal.FindLockfile(opts.Cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(opts.Stderr,
				"seal: pin: seal.json not found; run 'seal init' first")
		} else {
			fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
		}
		return 2
	}
	lf, err := seal.ReadFile(lockPath)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
		return 2
	}

	// Candidate set = union of discovered and recorded keys. A map dedups
	// without an explicit sort/merge.
	candidates := make(map[string]struct{})
	discovered, err := seal.ExpandPatterns(opts.Cwd, lf.Discovery)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
		return 2
	}
	for _, k := range discovered {
		candidates[k] = struct{}{}
	}
	for k := range lf.Bundles {
		candidates[k] = struct{}{}
	}

	// Hash and classify each candidate. We do this in its own pass so an
	// unreadable bundle aborts before any summary is emitted.
	type classified struct {
		key   string
		kind  pinKind
		files map[string]string // nil when kind == pinRemoved
	}
	classifiedAll := make([]classified, 0, len(candidates))

	for key := range candidates {
		full, statErr := bundleKeyToSafePath(opts.Cwd, key)
		if errors.Is(statErr, os.ErrNotExist) {
			// Gone → removed. Bulk's whole job is to surface (and optionally
			// prune) these, so don't fail here.
			classifiedAll = append(classifiedAll, classified{
				key:  key,
				kind: pinRemoved,
			})
			continue
		}
		if statErr != nil {
			fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", statErr)
			return 2
		}

		files, _, err := seal.HashSealedRoot(full)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
			return 2
		}
		existing, has := lf.Bundles[key]
		kind := classifyPinTarget(existing, has, files)
		classifiedAll = append(classifiedAll, classified{
			key:   key,
			kind:  kind,
			files: files,
		})
	}

	// Build summary rows and count pending writes. Removed only counts as
	// pending under --prune; this drives the no-op short-circuit below.
	targets := make([]pinTarget, 0, len(classifiedAll))
	var writeCount int
	for _, c := range classifiedAll {
		targets = append(targets, pinTarget{
			Key:       c.key,
			Kind:      c.kind,
			FileCount: len(c.files),
		})
		switch c.kind {
		case pinNew, pinModified:
			writeCount++
		case pinRemoved:
			if opts.Prune {
				writeCount++
			}
		}
	}

	// Print the summary even on the no-op path so reported-but-not-pruned
	// removals are visible (hints the user toward --prune).
	if writeCount == 0 {
		writePinSummary(opts.Stderr, targets)
		fmt.Fprintln(opts.Stderr, "No changes")
		return 0
	}

	writePinSummary(opts.Stderr, targets)
	if opts.Stdin == nil && !IsInteractive() {
		fmt.Fprintln(opts.Stderr,
			"seal: pin: changes require confirmation but no TTY available")
		return 2
	}
	if !Confirm(opts.Stdin, opts.Stderr, "Apply these pin changes?") {
		fmt.Fprintln(opts.Stderr, "pin aborted")
		return 2
	}

	// Apply: replace new/modified entries; drop removed iff --prune. Unchanged
	// entries are left alone - their bytes are already correct.
	if lf.Bundles == nil {
		lf.Bundles = make(map[string]seal.Bundle)
	}
	for _, c := range classifiedAll {
		switch c.kind {
		case pinNew, pinModified:
			ch := seal.ContentHash(c.files)
			lf.Bundles[c.key] = seal.Bundle{
				ContentHash: ch,
				Files:       c.files,
			}
		case pinRemoved:
			if opts.Prune {
				delete(lf.Bundles, c.key)
			}
		}
	}

	if err := seal.WriteFile(lockPath, lf); err != nil {
		fmt.Fprintf(opts.Stderr, "seal: pin: %v\n", err)
		return 2
	}

	fmt.Fprintf(opts.Stderr, "Pinned %d bundle(s)\n", writeCount)
	return 0
}

func rejectNestedPinTarget(bundles map[string]seal.Bundle, key string) error {
	for existing := range bundles {
		if seal.BundleKeyIsStrictAncestor(existing, key, false) {
			return fmt.Errorf(
				"cannot pin %s: target is inside existing sealed root %s; re-pin %s or change discovery first",
				key, existing, existing,
			)
		}
		if seal.BundleKeyIsStrictAncestor(key, existing, false) {
			return fmt.Errorf(
				"cannot pin %s: existing sealed root %s is inside target; remove %s or re-pin the containing root",
				key, existing, existing,
			)
		}
	}
	return nil
}

func appendDiscoveryPattern(patterns []string, pattern string) []string {
	for _, p := range patterns {
		if p == pattern {
			return patterns
		}
	}
	return append(patterns, pattern)
}
