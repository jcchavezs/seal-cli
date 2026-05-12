package seal

import (
	"fmt"
	"os"
)

// ReadFile loads seal.json: read + decode + validate, returning a Lockfile
// ready for use. Centralised so every subcommand has the same error
// contract.
//
// Error contract:
//   - File not found: underlying os.ReadFile error returned UNWRAPPED so
//     errors.Is(err, fs.ErrNotExist) keeps working at higher layers (the CLI
//     uses this to distinguish "no lockfile here" from "broken lockfile").
//   - Malformed or invalid: wrapped error includes the path so the CLI's
//     top-level formatter can name the offending file.
//   - Always returns nil *Lockfile on error.
func ReadFile(path string) (*Lockfile, error) {
	// Don't wrap a not-found error - callers rely on the unwrapped
	// fs.ErrNotExist sentinel.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Decode proves JSON shape and rejects unknown fields.
	lf, err := Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Validate proves every semantic rule (versions, hashes, NFC, ...).
	// Distinct "invalid lockfile:" prefix lets the user tell apart "JSON
	// malformed" from "semantically wrong".
	if err := Validate(lf); err != nil {
		return nil, fmt.Errorf("read %s: invalid lockfile: %w", path, err)
	}

	return lf, nil
}