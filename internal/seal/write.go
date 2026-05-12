package seal

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gofrs/flock"
)

// WriteFile encodes a Lockfile and writes it to path with two guarantees
// stacked together:
//
//  1. Concurrency: an exclusive flock on a sibling .lock file serialises
//     writers so two CLIs can't interleave encode-then-rename.
//  2. Crash atomicity: bytes go to a sibling .tmp.<pid>.<rand> file, then
//     os.Rename over the target. Readers always see either the old file or
//     the new one, never a torn write.
//
// Both are needed: flock alone doesn't help if a writer crashes mid-write
// (the partial file would be visible); atomic rename alone doesn't help if
// two writers race (both renames succeed and either could win).
func WriteFile(path string, lf *Lockfile) error {
	// Encode BEFORE locking: an encode failure leaves the on-disk file
	// untouched, and we don't block other writers during serialisation.
	bytes, err := Encode(lf)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	// Lock a sibling .lock file (not the target): avoids the chicken-and-egg
	// of needing to lock a file that may not exist yet (`seal init` creates
	// seal.json from scratch).
	lk := flock.New(path + ".lock")
	if err := lk.Lock(); err != nil {
		return fmt.Errorf("lock %s: %w", lk.Path(), err)
	}
	// Ignore Unlock error: it would mask the actual write error, and the
	// lock releases on process exit anyway.
	defer func() { _ = lk.Unlock() }()

	// pid + rand keeps temp names unique even if flock is bypassed and avoids
	// surprises from stale temps left by interrupted runs.
	tmp := tempPath(path)

	// 0o644 so users can inspect the lockfile in an editor without sudo.
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}

	// Best-effort directory fsync for POSIX durability. Many filesystems and
	// Windows don't meaningfully support dir-fsync; ignore the error.
	_ = syncDir(filepath.Dir(path))

	// On rename failure, leave the temp in place so the user can inspect it.
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// tempPath returns a same-directory temp filename. Same-directory matters:
// POSIX rename(2) is only atomic within a single filesystem, and a temp in
// /tmp would frequently cross filesystem boundaries.
func tempPath(target string) string {
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	suffix := ".tmp." + strconv.Itoa(os.Getpid()) + "." + strconv.Itoa(rand.Intn(1<<30))
	return filepath.Join(dir, base+suffix)
}

// syncDir fsyncs the directory so renames inside it become durable. Split
// out so a future platform shim has one place to land.
func syncDir(dir string) error {
	// Opening a directory and calling Sync is the standard POSIX idiom.
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}