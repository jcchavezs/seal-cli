package seal

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestWriteFile_RoundTrip verifies a file written by WriteFile is readable by
// ReadFile. This is the core happy path: encode, atomic rename, then full
// read+validate from disk.
func TestWriteFile_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seal.json")
	if err := WriteFile(path, newValidLockfile()); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
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

// TestWriteFile_AtomicCleanup verifies the temp file used during the
// atomic-rename step does not leak into the target directory after a
// successful write. A leak would suggest the rename step is missing or
// silently failing.
func TestWriteFile_AtomicCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seal.json")
	if err := WriteFile(path, newValidLockfile()); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		// The only file we expect is seal.json itself. Any sibling containing
		// ".tmp." in its name is a stranded atomic-rename staging file and means
		// the write didn't complete cleanly.
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file leaked: %s", e.Name())
		}
	}
}

// TestWriteFile_OverwriteExisting verifies WriteFile replaces an existing
// seal.json rather than failing with EEXIST. The atomic rename must
// overwrite, not balk; pin re-runs depend on this behavior.
func TestWriteFile_OverwriteExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seal.json")
	// First write establishes the file.
	if err := WriteFile(path, newValidLockfile()); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}
	// Second write must succeed even though the target already exists.
	// We don't change the contents because the atomic-rename behavior is what
	// matters here, not byte-level diff.
	if err := WriteFile(path, newValidLockfile()); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}
}

// TestWriteFile_Concurrent verifies the flock guard: many goroutines writing
// to the same path must serialize, and the final on-disk file must be a
// well-formed lockfile (not a torn write or a half-encoded stream). Without
// flock, concurrent renames could race and a reader might still see a
// coherent file by luck - the deterministic check is that ReadFile succeeds
// at the end.
//
// We run more goroutines than typical CPU count so the race detector (go test
// -race) has plenty of opportunity to flag any unguarded shared state inside
// WriteFile.
func TestWriteFile_Concurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seal.json")
	const writers = 8
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			if err := WriteFile(path, newValidLockfile()); err != nil {
				// t.Errorf is goroutine-safe; t.Fatalf is not.
				t.Errorf("WriteFile: %v", err)
			}
		}()
	}
	wg.Wait()
	// Read+validate the post-race file. If any of the renames produced a torn
	// write this read will fail with a decode or validate error.
	if _, err := ReadFile(path); err != nil {
		t.Fatalf("post-concurrent read: %v", err)
	}
}