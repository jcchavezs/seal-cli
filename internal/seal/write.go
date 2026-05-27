package seal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gofrs/flock"
)

// WriteFile encodes a Lockfile and writes it to path with two guarantees
// stacked together:
//
//  1. Concurrency: an exclusive flock on a cache-local .lock file serialises
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

	// Use a cache-local flock target, not path+".lock". The lock target file
	// is just a shared inode for the kernel lock; its existence is not the
	// lock. Removing it after unlock looks tidy, but is unsafe under contention:
	// a process can open the old inode and block in flock, then another process
	// can unlink the pathname after it finishes. A later process recreates the
	// same pathname as a new inode and locks that, while the earlier waiter may
	// still acquire the old unlinked inode. At that point two writers believe
	// they hold the same lock, but the kernel sees two different files.
	lockPath, err := writeLockPath(path)
	if err != nil {
		return err
	}
	lk := flock.New(lockPath)
	if err := lk.Lock(); err != nil {
		return fmt.Errorf("lock %s: %w", lk.Path(), err)
	}
	// Unlock errors are worth surfacing, but should not mask the actual write
	// result; the OS releases the lock on process exit anyway.
	defer unlockAndLog(lk, log.Printf)

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

type unlocker interface {
	Path() string
	Unlock() error
}

func unlockAndLog(lk unlocker, logf func(string, ...any)) {
	if err := lk.Unlock(); err != nil {
		logf("seal: unlock %s: %v", lk.Path(), err)
	}
}

// writeLockPath returns the stable flock rendezvous path for target.
//
// We use flock instead of "lock file exists" because existence-based locks need
// atomic O_EXCL creation, stale PID/host recovery, PID-reuse handling, and
// manual cleanup after crashes. Kernel locks give atomic acquisition and
// automatic release on process exit. The rendezvous file may remain, so it
// belongs in user cache/temp state rather than in arbitrary repositories.
func writeLockPath(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve lock path: %w", err)
	}
	sum := sha256.Sum256([]byte(abs))

	base, err := os.UserCacheDir()
	if err != nil {
		log.Printf("seal: UserCacheDir unavailable, falling back to TempDir: %v", err)
		base = os.TempDir()
	}
	dir := filepath.Join(base, "seal-cli", "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create lock dir: %w", err)
	}
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".lock"), nil
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
