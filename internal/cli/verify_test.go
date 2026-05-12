package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/seal"
)

// writeBundleAndLockfile is a verify-test helper: writes a single bundle on
// disk under cwd/<key>, hashes it, and writes a.
// seal.json so a fresh `RunVerify(cwd)` will produce a clean "Verified"
// result. Lets every happy-path test be three lines.
//
// We keep this in verify_test.go (not testutil/) so the harness is visible
// right next to the tests that consume it.
func writeBundleAndLockfile(t *testing.T, cwd, key, file, content string) {
	t.Helper()

	// Drop the leading "./" so we can Join to a real path.
	rel := strings.TrimPrefix(key, "./")

	// Write the single tracked file under the bundle dir.
	full := filepath.Join(cwd, rel, file)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Compute the real hash so the lockfile actually matches disk.
	files, ch, err := seal.HashSealedRoot(filepath.Join(cwd, rel))
	if err != nil {
		t.Fatalf("HashSealedRoot: %v", err)
	}

	lf := &seal.Lockfile{
		Version:   1,
		Policy:    "block",
		Discovery: discoveryPatternsForBundleKey(key),
		Bundles: map[string]seal.Bundle{
			key: {ContentHash: ch, Files: files},
		},
	}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func discoveryPatternsForBundleKey(key string) []string {
	rel := strings.TrimPrefix(key, "./")
	if rel == "" {
		return nil
	}
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return []string{rel[:i] + "/*"}
	}
	return []string{rel}
}

func TestRunVerify_UncoveredRemovedKeyExitsBeforeClassification(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./.dirB/2", "SKILL.md", "gone\n")
	lf, err := seal.ReadFile(filepath.Join(cwd, seal.LockfileName))
	if err != nil {
		t.Fatal(err)
	}
	lf.Discovery = nil
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(cwd, ".dirB")); err != nil {
		t.Fatal(err)
	}

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "Result:") {
		t.Fatalf("verify should not classify when lockfile is invalid:\n%s", stderr.String())
	}
}

func TestRunVerify_SiblingInsertionDetected(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./docs/contributor-prompt.md", ".", "prompt\n")
	lf, err := seal.ReadFile(filepath.Join(cwd, seal.LockfileName))
	if err != nil {
		t.Fatal(err)
	}
	lf.Discovery = []string{"docs/*.md"}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "docs", "contributor-prompt-v2.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 1 {
		t.Fatalf("exit %d, want 1 (blocked); stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Unverified") {
		t.Fatalf("sibling file should be Unverified:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "./docs/contributor-prompt-v2.md") {
		t.Fatalf("stderr should name sibling key:\n%s", stderr.String())
	}
}

func TestRunVerify_UncoveredBundleKey(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./docs/contributor-prompt.md", ".", "prompt\n")
	lf, err := seal.ReadFile(filepath.Join(cwd, seal.LockfileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lf.Discovery = nil
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not statically produced") {
		t.Fatalf("stderr should mention discovery coverage:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "./docs/contributor-prompt.md") {
		t.Fatalf("stderr should name uncovered bundle key:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "Result:") {
		t.Fatalf("verify should not classify when lockfile is invalid:\n%s", stderr.String())
	}
}

func TestRunVerify_HappyPath(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./skills/foo", "SKILL.md", "hello")

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Result: Verified") {
		t.Fatalf("missing 'Result: Verified' in:\n%s", stderr.String())
	}
}

func TestRunVerify_FileRootHappyPath(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(cwd, "skills", "m3.md")
	if err := os.WriteFile(file, []byte("single file skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, ch, err := seal.HashSealedRoot(file)
	if err != nil {
		t.Fatalf("HashSealedRoot: %v", err)
	}
	lf := &seal.Lockfile{Version: 1, Policy: "block", Discovery: []string{"skills/*"}, Bundles: map[string]seal.Bundle{"./skills/m3.md": {ContentHash: ch, Files: files}}}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Result: Verified") {
		t.Fatalf("missing 'Result: Verified' in:\n%s", stderr.String())
	}
}

func TestRunVerify_MissingLockfile(t *testing.T) {
	cwd := t.TempDir()
	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "seal.json") {
		t.Errorf("stderr should mention seal.json:\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on exit 2; got:\n%s", stdout.String())
	}
}

func TestRunVerify_BlockedOnMismatch(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./skills/foo", "SKILL.md", "original")
	if err := os.WriteFile(filepath.Join(cwd, "skills/foo/SKILL.md"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &bytes.Buffer{}})
	if code != 1 {
		t.Fatalf("exit %d, want 1 (Blocked); stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Blocked") {
		t.Errorf("stderr should mention Blocked:\n%s", stderr.String())
	}
}

func TestRunVerify_JSONFlagWritesStdout(t *testing.T) {
	cwd := t.TempDir()
	writeBundleAndLockfile(t, cwd, "./skills/foo", "SKILL.md", "hello")

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout, JSON: true})
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "Verified"`) {
		t.Errorf("stdout missing JSON status field:\n%s", stdout.String())
	}
	if strings.Contains(stderr.String(), "Result:") {
		t.Errorf("stderr should be quiet under --json:\n%s", stderr.String())
	}
}

func TestRunVerify_BlocksSymlinkedIntermediateRecordedKey(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "SKILL.md")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cwd, "link")); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}

	files, ch, err := seal.HashSealedRoot(outsideFile)
	if err != nil {
		t.Fatalf("HashSealedRoot: %v", err)
	}
	lf := &seal.Lockfile{
		Version:   1,
		Policy:    "block",
		Discovery: []string{"link/SKILL.md"},
		Bundles: map[string]seal.Bundle{
			"./link/SKILL.md": {ContentHash: ch, Files: files},
		},
	}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatal(err)
	}

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 1 {
		t.Fatalf("exit %d, want 1 (Blocked); stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Blocked") {
		t.Errorf("stderr should mention Blocked:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "Verified") {
		t.Errorf("stderr should not report Verified:\n%s", stderr.String())
	}
}

func TestRunVerify_BlocksDiscoverySymlinkLeaf(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "target.md"), []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../target.md", filepath.Join(cwd, "skills", "link.md")); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}

	lf := &seal.Lockfile{
		Version:   1,
		Discovery: []string{"skills/*"},
		Policy:    "block",
		Bundles:   map[string]seal.Bundle{},
	}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatal(err)
	}

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 1 {
		t.Fatalf("exit %d, want 1 (Blocked); stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Mismatch") || !strings.Contains(stderr.String(), "./skills/link.md") {
		t.Errorf("stderr should report symlink leaf as mismatch:\n%s", stderr.String())
	}
}

func TestRunVerify_RejectsExactDotBundleKey(t *testing.T) {
	cwd := t.TempDir()
	lockfile := `{"version":1,"policy":"block","bundles":{".":{"contentHash":"sha256:668a11c74d3ad0fa0f4a0fb92abbb568e35e5c91bcfc4501fea406903d0c9a5b","files":{"SKILL.md":"sha256:3bfc269594ef649228e9a74bab00f042efc91d5acc6fbee31a382e80d42388fe"}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(cwd, seal.LockfileName), []byte(lockfile), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 2 {
		t.Fatalf("exit %d, want 2 (Fatal Error); stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "bundle key") || !strings.Contains(stderr.String(), ".") {
		t.Errorf("stderr should identify exact dot bundle key as invalid:\n%s", stderr.String())
	}
}

func TestRunVerify_TreatsWrongCaseRecordedKeyAsRemoved(t *testing.T) {
	cwd := t.TempDir()
	trueRoot := filepath.Join(cwd, "plugins", "myskill")
	if err := os.MkdirAll(trueRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trueRoot, "SKILL.md"), []byte("skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(cwd, "plugins", "MySkill")); err != nil {
		t.Skipf("filesystem is case-sensitive; cannot reproduce wrong-case fallback: %v", err)
	}

	files, ch, err := seal.HashSealedRoot(trueRoot)
	if err != nil {
		t.Fatalf("HashSealedRoot: %v", err)
	}
	lf := &seal.Lockfile{
		Version:   1,
		Discovery: []string{"plugins/*"},
		Policy:    "block",
		Bundles: map[string]seal.Bundle{
			"./plugins/MySkill": {ContentHash: ch, Files: files},
		},
	}
	if err := seal.WriteFile(filepath.Join(cwd, seal.LockfileName), lf); err != nil {
		t.Fatal(err)
	}

	var stderr, stdout bytes.Buffer
	code := RunVerify(VerifyOpts{Cwd: cwd, Stderr: &stderr, Stdout: &stdout})
	if code != 1 {
		t.Fatalf("exit %d, want 1 (Blocked); stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "0 verified, 1 unverified, 1 removed, 0 mismatch") {
		t.Errorf("stderr should count wrong-case key as removed and true-case key as unverified:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Removed    ./plugins/MySkill") {
		t.Errorf("stderr should list wrong-case recorded key as removed:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Unverified ./plugins/myskill") {
		t.Errorf("stderr should list true-case discovered key as unverified:\n%s", stderr.String())
	}
}
