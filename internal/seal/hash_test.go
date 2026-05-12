package seal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestHashSealedRoot_HappyPath verifies the integration: walk the tree, hash
// each file with SHA-256, NFC-normalize keys, and confirm the returned
// contentHash equals what ContentHash(files) would compute independently.
//
// Cross-checking against ContentHash (rather than just asserting "looks
// hash-shaped") catches integration drift: if HashSealedRoot stored non-NFC
// keys, ContentHash's joinEntries lookup would miss and the returned
// aggregate wouldn't match.
func TestHashSealedRoot_HappyPath(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "SKILL.md"), "skill\n")
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(dir, "scripts", "run.sh"), "run\n")

	files, ch, err := HashSealedRoot(dir)
	if err != nil {
		t.Fatalf("HashSealedRoot: %v", err)
	}

	// Two regular files in, two map entries out.
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d (%v)", len(files), files)
	}

	// Each per-file hash is SHA-256 of the literal file bytes. We compute the
	// expected hash for "skill\n" independently and compare.
	wantSkillHash := "sha256:" + sha256Hex([]byte("skill\n"))
	if got := files["SKILL.md"]; got != wantSkillHash {
		t.Errorf("SKILL.md: want %q, got %q", wantSkillHash, got)
	}
	wantRunHash := "sha256:" + sha256Hex([]byte("run\n"))
	if got := files["scripts/run.sh"]; got != wantRunHash {
		t.Errorf("scripts/run.sh: want %q, got %q", wantRunHash, got)
	}

	// The aggregate contentHash must equal what ContentHash would compute over
	// the same map. This is the integration invariant.
	if want := ContentHash(files); ch != want {
		t.Errorf("contentHash mismatch:\n  got  %q\n  want %q", ch, want)
	}
}

func TestHashSealedRoot_FileRoot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "prompt.md")
	mustWriteFile(t, file, "prompt\n")

	files, ch, err := HashSealedRoot(file)
	if err != nil {
		t.Fatalf("HashSealedRoot: %v", err)
	}
	wantHash := "sha256:" + sha256Hex([]byte("prompt\n"))
	if got := files["."]; got != wantHash {
		t.Fatalf("file-root . hash: got %q, want %q (files=%v)", got, wantHash, files)
	}
	if len(files) != 1 {
		t.Fatalf("file-root should have one entry, got %v", files)
	}
	if want := ContentHash(files); ch != want {
		t.Errorf("contentHash mismatch: got %q want %q", ch, want)
	}
}

func TestHashSealedRoot_IncludesBundleLocalSealJSON(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "SKILL.md"), "skill\n")
	mustWriteFile(t, filepath.Join(dir, "seal.json"), "{\"local\":true}\n")

	files, _, err := HashSealedRoot(dir)
	if err != nil {
		t.Fatalf("HashSealedRoot: %v", err)
	}
	if _, ok := files["seal.json"]; !ok {
		t.Fatalf("bundle-local seal.json should be tracked; files=%v", files)
	}
}

func TestHashSealedRoot_RejectsRootSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}

	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "real.txt"), "real\n")
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink("real.txt", link); err != nil {
		t.Fatal(err)
	}

	_, _, err := HashSealedRoot(link)
	if err == nil {
		t.Fatalf("expected root symlink to be rejected")
	}
}

// TestHashSealedRoot_EmptyBundleErrors verifies enforced at the producer
// side: a directory with no trackable files (or one containing only excluded
// files) must error rather than returning an empty map. This prevents pin
// from creating a bundle that ReadFile would later reject.
func TestHashSealedRoot_EmptyBundleErrors(t *testing.T) {
	dir := t.TempDir()
	// .git is excluded by.git directory has zero trackable files - same error
	// path as a fully empty directory.
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(dir, ".git", "HEAD"), "head\n")

	_, _, err := HashSealedRoot(dir)
	if err == nil {
		t.Fatalf("expected error for bundle with no trackable files")
	}
}

// TestHashSealedRoot_NFCCollisionErrors verifies the defensive check:
// if the filesystem somehow contains two byte-distinct entries that
// NFC-normalize to the same path, HashSealedRoot must error rather than
// silently dropping one entry into the same map key.
//
// macOS APFS folds NFC and NFD to the same on-disk identity, so this case is
// unreachable there - the second create overwrites the first.
// We probe at runtime and skip if the host filesystem won't allow the
// scenario; the inline check itself is correct by inspection.
func TestHashSealedRoot_NFCCollisionErrors(t *testing.T) {
	dir := t.TempDir()

	// NFC and NFD forms of "café" - byte-distinct strings that collapse to the
	// same NFC representation.
	nfcName := "café.txt" // composed: caf + U+00E9 + .txt
	nfdName := "café.txt" // decomposed: cafe + U+0301 + .txt

	if err := os.WriteFile(filepath.Join(dir, nfcName), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, nfdName), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Probe whether the filesystem actually preserved both as distinct entries.
	// APFS folds them to one; ext4/NTFS preserve both. If the
	// FS folded, we cannot exercise the collision path here.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Skipf("filesystem folded NFC/NFD entries (got %d), can't reproduce collision", len(entries))
	}

	_, _, err = HashSealedRoot(dir)
	if err == nil {
		t.Fatalf("expected NFC-collision error, got nil")
	}
	if !strings.Contains(err.Error(), "NFC") && !strings.Contains(err.Error(), "normalize") {
		t.Errorf("error %q does not mention NFC/normalize", err)
	}
}
