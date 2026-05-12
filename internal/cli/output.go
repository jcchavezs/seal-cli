package cli

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/iancoleman/orderedmap"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// WriteVerifyJSON emits `seal verify --json`:
//
//	{ "status": "<headline>",
//	  "verified": ["./bundle/a"],
//	  "unverified": [], "removed": [], "mismatch": [] }
//
// All four arrays are always present (never null and never absent) so jq
// pipelines and CI scripts don't have to handle missing keys.
//
// orderedmap is used because Go's map iteration is randomised - we need
// byte-stable output for CI diff tooling. A tagged struct would also be
// ordered but every key addition would need a code edit; orderedmap keeps it
// data-driven.
func WriteVerifyJSON(w io.Writer, status string, states []seal.State) error {
	// Status-keyed grouping so adding a Status later is a one-line change.
	groups := map[seal.Status][]string{}
	for _, s := range states {
		groups[s.Status] = append(groups[s.Status], s.Key)
	}

	// Sort each group; output must be deterministic regardless of how the
	// caller assembled `states`.
	for k := range groups {
		sort.Strings(groups[k])
	}

	out := orderedmap.New()
	out.Set("status", status)
	out.Set("verified", nilToEmpty(groups[seal.Verified]))
	out.Set("unverified", nilToEmpty(groups[seal.Unverified]))
	out.Set("removed", nilToEmpty(groups[seal.Removed]))
	out.Set("mismatch", nilToEmpty(groups[seal.Mismatch]))

	enc := json.NewEncoder(w)
	// Two-space indent matches the lockfile encoder so all seal JSON looks
	// the same in editors.
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// nilToEmpty turns a nil slice into an empty slice so the JSON encoder emits
// `[]` rather than `null` - load-bearing for the "always an array" invariant
// documented on WriteVerifyJSON.
func nilToEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}