package seal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadFile_RoundTrip verifies the happy path: a valid lockfile written by
// Encode is read back successfully by ReadFile.
//
// The composition under test is Decode + Validate, so a successful round-trip
// exercises both halves; either can fail and the test will catch it.
func TestReadFile_RoundTrip(t *testing.T) {
	// Use a temp dir so the test never touches the user's working tree;
	// t.TempDir cleans up automatically when the test exits.
	dir := t.TempDir()
	path := filepath.Join(dir, "seal.json")

	// Encode the canonical valid lockfile to bytes, then write to disk.
	// This is the same shape ReadFile will need to consume.
	lf := newValidLockfile()
	bytes, err := Encode(lf)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// ReadFile should accept the file and return a struct equivalent to what we
	// wrote.
	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("version: want 1, got %d", got.Version)
	}
	if len(got.Bundles) != 1 {
		t.Errorf("bundles: want 1, got %d", len(got.Bundles))
	}
}

// TestReadFile_Missing verifies that ReadFile preserves the standard
// fs.ErrNotExist sentinel from os.ReadFile so callers can distinguish
// "missing" from "invalid" with idiomatic errors.Is checks. This is the
// contract the CLI's "no lockfile here" diagnostic depends.
func TestReadFile_Missing(t *testing.T) {
	_, err := ReadFile(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected errors.Is(err, fs.ErrNotExist) to be true, got %v", err)
	}
}

// TestReadFile_InvalidJSON verifies that malformed bytes surface a decode
// error rather than crashing. The error wraps the path so the
// CLI can produce a useful diagnostic.
func TestReadFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seal.json")
	// Plausible-looking but malformed JSON: trailing brace mismatch.
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ReadFile(path)
	if err == nil {
		t.Fatalf("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not include path %q", err, path)
	}
}

// TestReadFile_FailsValidation verifies that ReadFile invokes Validate after
// Decode - a structurally-valid JSON but semantically-invalid lockfile (e.g.
// wrong version) must be rejected. Without this check, the Decode+Validate
// composition could silently degrade to "Decode only" and bad lockfiles would
// slip through.
func TestReadFile_FailsValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seal.json")
	// Build a lockfile with a.
	// The encoder doesn't validate; that's exactly what ReadFile is.
	lf := newValidLockfile()
	lf.Version = 2 //.
	bytes, err := Encode(lf)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ReadFile(path); err == nil {
		t.Fatalf("expected error for invalid version, got nil")
	}
}