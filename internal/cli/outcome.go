package cli

import "github.com/SamyGhannad/seal-cli/internal/seal"

// derivedOutcome reduces the per-bundle classifications to the single
// overall outcome string - the headline `seal verify` prints, what `--json`
// puts in its top-level "status" field, and what the exit-code logic reads.
//
// Note the two distinct vocabularies:
//
//	Per-bundle Status: Verified | Unverified | Removed | Mismatch
//	Overall Outcome:   "Verified" | "Drift" | "Warning" | "Blocked"
//
// Only "Verified" overlaps. Per-bundle Removed alone surfaces as "Drift"
// (informational - someone deliberately deleted a tracked dir). Per-bundle
// Unverified or Mismatch escalates to "Blocked" (exit 1) under block policy
// or "Warning" (exit 0) under warn policy.
//
// Drift is its own outcome rather than folding into Blocked because deletion
// is much lower-risk than a new unaudited dir (Unverified) or a tracked dir
// changing out from under us (Mismatch). It also lets `seal pin` later treat
// Drift specially - a bulk pin can prune vanished entries without prompting.
//
// `policy` is a plain string because the lockfile round-trips it that way
// and Validate already guarantees it's one of {"block","warn"}.
func derivedOutcome(states []seal.State, policy string) string {
	// Presence flags, not counts: one Mismatch and a hundred Mismatches reach
	// the same outcome.
	var hasUnverified, hasMismatch, hasRemoved bool
	for _, s := range states {
		switch s.Status {
		case seal.Unverified:
			hasUnverified = true
		case seal.Mismatch:
			hasMismatch = true
		case seal.Removed:
			hasRemoved = true
		}
	}

	if !hasUnverified && !hasMismatch && !hasRemoved {
		return "Verified"
	}

	// Only Removed → informational Drift.
	if !hasUnverified && !hasMismatch {
		return "Drift"
	}

	// Risky drift; policy decides whether CI gates.
	if policy == "warn" {
		return "Warning"
	}
	return "Blocked"
}