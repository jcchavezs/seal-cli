package seal

import "sort"

// Status is the per-bundle classification after `seal verify`.
//
// Int+iota rather than string: exhaustive-switch linters can warn when a
// new constant is added but a switch isn't updated, comparison is a single
// int compare, and the value is a fixed bit pattern.
//
// The zero value (statusUnset) is deliberately NOT Verified - a forgotten
// Status field on a State{} literal must never silently read as a successful
// verification. Any observation of statusUnset is a bug; String() renders it
// "unset" so the bug surfaces immediately.
type Status int

const (
	// statusUnset is the zero value. Unexported so external code can't build
	// one; exists only to detect uninitialized State{} fields.
	statusUnset Status = iota

	// Verified: present on disk and in lockfile, all per-file hashes match.
	// This is good - we love this
	Verified
	// Unverified: present on disk (via a discovery match) but no lockfile
	// entry.
	// This is bad - weo don't love this
	// Who added this file? and why?
	Unverified
	// Removed: recorded in lockfile but the directory is gone from disk.
	// This is meh - we had something, we don't have it anymore
	Removed
	// Mismatch: present in both, but at least one file is modified, added,
	// or missing, or present on disk but cannot be verified.
	// This is bad - we have something, but it's not what we expected.
	// What changed? Who changed it? Why? I'm scared. Mom! MOM!
	Mismatch
)

// String returns the lowercase human form (matches the JSON-key convention).
// statusUnset deliberately renders as "unset" so a bug-induced zero value
// produces obviously-wrong output rather than masquerading as "verified".
func (s Status) String() string {
	switch s {
	case statusUnset:
		return "unset"
	case Verified:
		return "verified"
	case Unverified:
		return "unverified"
	case Removed:
		return "removed"
	case Mismatch:
		return "mismatch"
	}
	return "unknown"
}

// State pairs a bundle key with its classification. Modified/Added/Missing
// are populated only for Mismatch; they are nil for the other statuses.
type State struct {
	// Key is the lockfile-shaped key: "./..." with forward slashes.
	Key string

	// Status; Classify always sets this on every returned element.
	Status Status

	// Modified: recorded hash differs from on-disk hash. Sorted.
	Modified []string
	// Added: on disk but not recorded. Sorted.
	Added []string
	// Missing: recorded but not on disk. Sorted. Named "Missing" rather than
	// "Removed" to avoid colliding with the Status constant of the same name
	// (file vs. whole-bundle absence).
	Missing []string
}

// Classify compares the lockfile's bundles against the on-disk map (key →
// per-file hashes) and returns one State per union member. Core of
// `seal verify`.
//
// Pure: no filesystem access, no hashing. The caller supplies the on-disk
// map; tests become two map literals, which is what makes the four-state
// matrix tractable to test exhaustively.
//
// Output order is undefined; callers that need determinism (e.g. JSON) sort
// themselves.
func Classify(lf *Lockfile, onDisk map[string]map[string]string) []State {
	out := make([]State, 0, len(lf.Bundles))

	// Pass 1: lockfile entries → Verified / Mismatch / Removed.
	for key, b := range lf.Bundles {
		files, present := onDisk[key]
		if !present {
			out = append(out, State{Key: key, Status: Removed})
			continue
		}
		if equalFileMaps(b.Files, files) {
			out = append(out, State{Key: key, Status: Verified})
			continue
		}
		mod, add, missing := diffFileMaps(b.Files, files)
		out = append(out, State{
			Key:      key,
			Status:   Mismatch,
			Modified: mod,
			Added:    add,
			Missing:  missing,
		})
	}

	// Pass 2: on-disk-only keys are usually Unverified. If the caller provided an
	// empty file map, it means discovery matched an entry that could not be
	// hashed, so report it as Mismatch.
	for key := range onDisk {
		if _, ok := lf.Bundles[key]; !ok {
			if len(onDisk[key]) == 0 {
				out = append(out, State{Key: key, Status: Mismatch})
				continue
			}
			out = append(out, State{Key: key, Status: Unverified})
		}
	}
	return out
}

// equalFileMaps reports byte-identical equality. Short-circuits on first
// difference, where diffFileMaps walks both maps fully.
func equalFileMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// diffFileMaps returns sorted slices for:
//   - modified: same key, different hash
//   - added:    in b only (on disk, not recorded)
//   - missing:  in a only (recorded, not on disk)
func diffFileMaps(a, b map[string]string) (modified, added, missing []string) {
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			missing = append(missing, k)
			continue
		}
		if va != vb {
			modified = append(modified, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			added = append(added, k)
		}
	}
	sort.Strings(modified)
	sort.Strings(added)
	sort.Strings(missing)
	return
}
