package cli

import (
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// TestDerivedOutcome encodes the table. Splitting this from RunVerify lets
// the rules be audited as pure data - no filesystem fixtures, no goroutines,
// no I/O - so a reviewer can read one short test and convince themselves the
// policy is right.
//
// Quick refresher on the four
//
// - "Verified" - every bundle classified Verified. The only zero- drift
// outcome; CI should treat as success.
// - "Drift" - only Removed bundles (lockfile entries with no matching
// directory on disk). Not blocking by itself: someone intentionally deleted a
// previously-trusted bundle, which is less dangerous than something appearing
// or changing silently.
// - "Warning" - there is risky drift (Unverified or Mismatch)
// AND the lockfile's policy is "warn". Caller prints the warning but exits 0
// so CI doesn't go red.
// - "Blocked" - risky drift AND policy is "block" (the default).
// Caller exits 1; CI goes red.
//
// Important: a Removed bundle never escalates to Blocked. Detecting a
// deletion is informational; only NEW or MODIFIED bundles trigger the policy
// gate.
func TestDerivedOutcome(t *testing.T) {
	cases := []struct {
		name   string
		states []seal.State
		policy string
		want   string
	}{
		// Empty lockfile + empty disk ⇒ everything verified by absence.
		{
			name:   "no states ⇒ Verified",
			states: nil,
			policy: "block",
			want:   "Verified",
		},
		// All four verified, single bundle. Sanity case.
		{
			name:   "all Verified ⇒ Verified",
			states: []seal.State{{Status: seal.Verified}},
			policy: "block",
			want:   "Verified",
		},
		// Only Removed entries ⇒ Drift, even under block policy.
		// This is the load-bearing case: deletion alone doesn't fail.
		{
			name:   "only Removed under block ⇒ Drift (not Blocked)",
			states: []seal.State{{Status: seal.Removed}},
			policy: "block",
			want:   "Drift",
		},
		// Removed + Verified mix: still Drift; the Verified entries don't change
		// the verdict.
		{
			name:   "Verified + Removed under block ⇒ Drift",
			states: []seal.State{{Status: seal.Verified}, {Status: seal.Removed}},
			policy: "block",
			want:   "Drift",
		},
		// Single Unverified under block ⇒ Blocked.
		{
			name:   "Unverified under block ⇒ Blocked",
			states: []seal.State{{Status: seal.Unverified}},
			policy: "block",
			want:   "Blocked",
		},
		// Same input under warn policy ⇒ Warning, not Blocked.
		{
			name:   "Unverified under warn ⇒ Warning",
			states: []seal.State{{Status: seal.Unverified}},
			policy: "warn",
			want:   "Warning",
		},
		// Mismatch is the other risky status.
		{
			name:   "Mismatch under block ⇒ Blocked",
			states: []seal.State{{Status: seal.Mismatch}},
			policy: "block",
			want:   "Blocked",
		},
		{
			name:   "Mismatch under warn ⇒ Warning",
			states: []seal.State{{Status: seal.Mismatch}},
			policy: "warn",
			want:   "Warning",
		},
		// Combo: Removed coexisting with Unverified should still hit the
		// risky-drift branch - the Removed alone wouldn't, but the Unverified does.
		{
			name: "Removed + Unverified under block ⇒ Blocked",
			states: []seal.State{
				{Status: seal.Removed},
				{Status: seal.Unverified},
			},
			policy: "block",
			want:   "Blocked",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := derivedOutcome(c.states, c.policy)
			if got != c.want {
				t.Fatalf("derivedOutcome(...) = %q, want %q", got, c.want)
			}
		})
	}
}