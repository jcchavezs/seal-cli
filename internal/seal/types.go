// Package seal is the data layer: in-memory model of a seal.json lockfile,
// deterministic encoder and strict decoder, structural validation, content
// hashing, sealed-root hashing, discovery glob expansion, and init-time
// heuristics.
//
// Command orchestration (RunInit, RunPin, RunVerify) lives one level up in
// internal/cli; the cobra wrapper lives in cmd/seal.
package seal

// Lockfile is the parsed representation of a seal.json.
//
// Struct field order does NOT control serialisation order - encode.go is
// authoritative for that.
type Lockfile struct {
	// Version is the schema version. Only 1 is accepted in this
	// implementation.
	Version int `json:"version"`

	// Discovery holds the optional glob patterns standalone tools use to
	// find unpinned project-local bundles. The encoder drops the field
	// entirely when empty (cleaner initial diffs).
	Discovery []string `json:"discovery,omitempty"`

	// Policy is one of "block" or "warn".
	Policy string `json:"policy"`

	// Bundles maps bundle keys (e.g. "./.claude/skills/foo") to entries.
	// Keys are sorted at serialisation time, not at struct level.
	Bundles map[string]Bundle `json:"bundles"`
}

// Bundle is one entry under the bundles map. Struct order does not control
// serialisation order; encode.go emits revision (omitted when empty),
// contentHash, files.
type Bundle struct {
	// Revision is informational (e.g. a git SHA recorded at pin time). Empty
	// means "do not emit". MUST NEVER be used for integrity decisions.
	Revision string `json:"revision,omitempty"`

	// ContentHash is the aggregate SHA-256 over the files map. Always
	// present in a valid bundle.
	ContentHash string `json:"contentHash"`

	// Files maps sealed-root-relative paths (NFC, forward slashes) to
	// per-file hashes in canonical form ("sha256:" + 64 lowercase hex).
	// A bundle MUST contain at least one file.
	Files map[string]string `json:"files"`
}
