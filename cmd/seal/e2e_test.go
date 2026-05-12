package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// findModuleRoot walks UP from this test file's directory looking for go.mod.
// Using runtime.Caller(0) makes the lookup robust against where `go test` is
// invoked from - `go test ./...`.
// the repo root and `cd cmd/seal && go test` from inside both end up calling
// this with the same answer.
//
// The plan suggested cmd.Dir = ".." for the build helper; that breaks the
// moment anyone reorganises the tree or runs tests from a different CWD.
// Walking up to go.mod is one of those "do it right once" decisions whose
// ergonomics pay off later.
func findModuleRoot(t *testing.T) string {
	t.Helper()

	// Caller(0) gives us this very file's path.
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	// Walk upward until we hit go.mod. We cap the loop at a reasonable depth so
	// a misconfigured environment fails quickly rather than crawling all the way
	// to /.
	dir := filepath.Dir(here)
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// We've hit the filesystem root without finding go.mod.
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate go.mod from %s", here)
	return ""
}

// buildSealBinary builds the seal CLI binary into t.TempDir() and returns the
// absolute path. We build once per test that needs it (rather than once per
// test package) because t.TempDir is per- test-cleanup AND because parallel
// tests would race on a shared path. The build is cheap (~1s on first run,
// cached afterwards), so paying it per test is fine and keeps each test
// hermetic.
func buildSealBinary(t *testing.T) string {
	t.Helper()
	root := findModuleRoot(t)
	out := filepath.Join(t.TempDir(), "seal")
	// Windows refuses to exec a file without an executable extension; both
	// `go build -o` and the later exec.Command need the same path.
	if runtime.GOOS == "windows" {
		out += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", out, "./cmd/seal")
	cmd.Dir = root
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\noutput:\n%s", err, msg)
	}
	return out
}

// runSeal invokes the built binary in workdir with optional stdin and returns
// combined stdout+stderr plus the exit code. We use
// CombinedOutput because cobra writes the "[y/N] " prompt.
// stderr and the success line to stderr; reading them together keeps
// assertions simple.
//
// Exit code is read from cmd.ProcessState rather than relying.
// the error return - exec.Command treats any non-zero exit as an error, but
// for `seal verify` exit-1-on-Blocked is an expected outcome we want to
// assert on, not a test failure.
func runSeal(t *testing.T, bin, workdir, stdin string, args ...string) (string, int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = workdir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, _ := cmd.CombinedOutput()

	// ProcessState is non-nil as long as the process actually ran. If it didn't
	// (binary missing, exec failed), we'd have already returned via the build
	// helper, so this is safe.
	return string(out), cmd.ProcessState.ExitCode()
}

// runSealSplit invokes the binary like runSeal but captures stdout and stderr
// separately. Use this whenever a test cares about the stdout/stderr
// separation - e.g. `verify --json` (machine output on stdout, human summary
// suppressed on stderr) or `verify --quiet` (both streams empty).
func runSealSplit(t *testing.T, bin, workdir, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = workdir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	// Split-capture via strings.Builder. We swallow cmd.Run's error because
	// non-zero exit is an expected outcome here, not a test failure - the caller
	// asserts on exitCode directly.
	var sout, serr strings.Builder
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	_ = cmd.Run()
	return sout.String(), serr.String(), cmd.ProcessState.ExitCode()
}

// writeFile is a tiny helper to populate fixture trees inside an
// E2E temp directory.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestE2E_InitThenVerify is the round-trip happy path: bootstrap a fresh
// project with `seal init`, then `seal verify` to confirm the lockfile and
// disk agree. Exercises all three layers of the binary at once - cobra
// parsing, the cli package handlers, and the underlying seal domain code.
//
// We answer "y" at init's prompt; the answer is read from stdin, which we
// feed via runSeal's stdin parameter. The test wouldn't hang even if we
// forgot the "y" - Confirm treats EOF as "no".
// design (commit 2125db1), so a missing stdin would surface as "init aborted"
// / exit 2.
func TestE2E_InitThenVerify(t *testing.T) {
	bin := buildSealBinary(t)

	dir := t.TempDir()
	// A Claude skills layout - heuristics will propose `.claude/skills/*` and
	// pin our one bundle.
	writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "v1")

	// init: answer "y" so the prompt accepts.
	out, code := runSeal(t, bin, dir, "y\n", "init")
	if code != 0 {
		t.Fatalf("init exit %d; combined output:\n%s", code, out)
	}
	if !strings.Contains(out, "Wrote seal.json") {
		t.Fatalf("init: missing success line in:\n%s", out)
	}

	// Lockfile must now exist on disk.
	if _, err := os.Stat(filepath.Join(dir, "seal.json")); err != nil {
		t.Fatalf("seal.json not created: %v", err)
	}

	// verify: should print "Result: Verified" and exit 0.
	out, code = runSeal(t, bin, dir, "", "verify")
	if code != 0 {
		t.Fatalf("verify exit %d; combined output:\n%s", code, out)
	}
	if !strings.Contains(out, "Result: Verified") {
		t.Fatalf("verify: missing 'Result: Verified' in:\n%s", out)
	}
}

// TestE2E_MutationBlocksVerify is the "tampering detected" path:
// after a clean init+verify, mutate the tracked file on disk and confirm
// `seal verify` now exits 1 with "Blocked". This is the
// CI-integration claim - the whole point of the tool is to make this
// transition surface as a non-zero exit.
func TestE2E_MutationBlocksVerify(t *testing.T) {
	bin := buildSealBinary(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "original")

	// init → clean state.
	if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
		t.Fatalf("init: exit %d", code)
	}

	// Mutate the tracked file. From the lockfile's perspective the
	// SHA-256 has now drifted, so verify should report Mismatch and (under
	// default block policy) exit 1.
	writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "TAMPERED")

	out, code := runSeal(t, bin, dir, "", "verify")
	if code != 1 {
		t.Fatalf("expected exit 1 (Blocked), got %d; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Blocked") {
		t.Fatalf("expected 'Blocked' in output:\n%s", out)
	}
}

// TestE2E_VerifyJSONShape verifies the --json flag routes machine- readable
// output to stdout. The internal/cli JSON tests already pin the structure;
// this test confirms the cobra wiring honours stdout-vs-stderr separation
// under a real process so a downstream `seal verify --json | jq` pipeline
// works end-to-end.
//
// We capture stdout and stderr separately here (not via
// CombinedOutput) because the whole point of --json is that stdout stays
// clean for tooling. A regression where the human summary leaks into stdout
// would break shell pipelines silently.
func TestE2E_VerifyJSONShape(t *testing.T) {
	bin := buildSealBinary(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "v1")
	if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
		t.Fatalf("init: exit %d", code)
	}

	// Separate stdout / stderr capture.
	cmd := exec.Command(bin, "verify", "--json")
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("verify --json: %v\nstderr:\n%s", err, stderr.String())
	}

	// stdout must contain the expected top-level fields.
	for _, want := range []string{
		`"status": "Verified"`,
		`"verified"`,
		`"unverified"`,
		`"removed"`,
		`"mismatch"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q; full stdout:\n%s", want, stdout.String())
		}
	}

	// stderr must NOT contain the human summary trailer - that's the regression
	// we're guarding against.
	if strings.Contains(stderr.String(), "Result:") {
		t.Errorf("stderr leaked human summary under --json:\n%s", stderr.String())
	}
}

// TestE2E_InitRefusesOverwrite proves the refuse-overwrite contract is
// honoured at the binary level (not just the cli handler). A second `seal
// init` against a directory that already has seal.json must exit 2 with the
// existing file intact byte-for-byte.
func TestE2E_InitRefusesOverwrite(t *testing.T) {
	bin := buildSealBinary(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "v1")

	// First init: succeeds.
	if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
		t.Fatalf("first init exit %d", code)
	}

	// Snapshot the lockfile bytes so we can prove invariance.
	before, err := os.ReadFile(filepath.Join(dir, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Second init: should exit 2 and leave the file untouched.
	out, code := runSeal(t, bin, dir, "y\n", "init")
	if code != 2 {
		t.Fatalf("second init: exit %d, want 2; output:\n%s", code, out)
	}

	after, err := os.ReadFile(filepath.Join(dir, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("seal.json was modified by refused-overwrite init")
	}
}

// TestE2E_InitDetectsAllHeuristicLayouts covers the heuristics
// Registry beyond the Claude-skills entry that the other E2E tests happen to
// exercise. A user with multiple agent toolchains in one repo should see ALL
// of them surface in the proposal - `init` silently missing a layout would
// look like a feature bug even though the registry technically knows about
// it.
//
// We populate every entry from internal/seal.Registry with a single fixture
// tree and assert that init's output mentions each pattern AND that the
// resulting seal.json contains both the discovery patterns and the matched
// bundle keys.
//
// Why one combined test instead of four subtests: the realistic user has
// multiple toolchains coexisting; one combined run proves they don't
// interfere. The error messages name which specific pattern is missing if a
// regression hits one entry, which is just as actionable as a per-subtest
// split.
func TestE2E_InitDetectsAllHeuristicLayouts(t *testing.T) {
	bin := buildSealBinary(t)

	dir := t.TempDir()

	// Each known agent-layout entry gets one bundle on disk.
	// SKILL.md is just placeholder content - the heuristics check looks at the
	// parent directory's existence, not at filenames.
	writeFile(t, filepath.Join(dir, ".claude/plugins/p1/SKILL.md"), "p1")
	writeFile(t, filepath.Join(dir, ".claude/skills/s1/SKILL.md"), "s1")
	writeFile(t, filepath.Join(dir, ".codex/skills/c1/SKILL.md"), "c1")
	writeFile(t, filepath.Join(dir, ".agents/skills/a1/SKILL.md"), "a1")

	// Run init. The preview should mention every proposed pattern;
	// then we accept with "y" and inspect the on-disk lockfile.
	out, code := runSeal(t, bin, dir, "y\n", "init")
	if code != 0 {
		t.Fatalf("init: exit %d; combined output:\n%s", code, out)
	}

	// The init preview surfaces patterns in Registry order. We assert each is
	// named so a regression that drops one.
	// the proposal is caught even if init still writes a valid (but incomplete)
	// seal.json.
	expectedPatterns := []string{
		".claude/plugins/*",
		".claude/skills/*",
		".codex/skills/*",
		".agents/skills/*",
	}
	for _, p := range expectedPatterns {
		if !strings.Contains(out, p) {
			t.Errorf("init output should mention pattern %q; got:\n%s", p, out)
		}
	}

	// Round-trip the produced seal.json and confirm every pattern landed in
	// discovery AND every bundle landed in bundles.
	raw, err := os.ReadFile(filepath.Join(dir, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}
	rawStr := string(raw)
	for _, p := range expectedPatterns {
		if !strings.Contains(rawStr, p) {
			t.Errorf("seal.json should include discovery pattern %q:\n%s",
				p, rawStr)
		}
	}
	// Each bundle directory must have been hashed and recorded.
	expectedBundles := []string{
		"./.claude/plugins/p1",
		"./.claude/skills/s1",
		"./.codex/skills/c1",
		"./.agents/skills/a1",
	}
	for _, b := range expectedBundles {
		if !strings.Contains(rawStr, b) {
			t.Errorf("seal.json should include bundle key %q:\n%s",
				b, rawStr)
		}
	}

	// And a sanity check: after init, the project is in a clean state - verify
	// must report Verified. This catches the case where init writes a lockfile
	// that doesn't actually agree with disk (e.g. a hash mismatch on one of the
	// rarer layouts).
	out, code = runSeal(t, bin, dir, "", "verify")
	if code != 0 {
		t.Fatalf("verify after multi-layout init: exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "Result: Verified") {
		t.Errorf("expected 'Result: Verified' after multi-layout init:\n%s", out)
	}
}

// TestE2E_VerifyOutputFlags pins contracts for the three verify output modes
// at the binary boundary: --quiet (truly silent.
// both fds, regardless of outcome), and the --json/--quiet mutual exclusion
// (caught at the cobra layer with exit 2 BEFORE any I/O).
//
// internal/cli covers the underlying handlers; cmd/seal/main_test covers the
// cobra mutex check via the in-process run() entry point.
// This test proves both paths behave correctly when invoked as a real process
// - which is what `seal verify --quiet | wc -c` users will actually.
func TestE2E_VerifyOutputFlags(t *testing.T) {
	bin := buildSealBinary(t)

	t.Run("--quiet emits nothing on success", func(t *testing.T) {
		// Pin a clean bundle so verify's outcome is Verified.
		// Under --quiet both stdout AND stderr must be empty - the exit code is the
		// only signal. This is the load- bearing CI contract: `seal verify --quiet;
		// echo $?` must produce a one-line output.
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "v1")
		if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
			t.Fatalf("init: exit %d", code)
		}

		stdout, stderr, code := runSealSplit(t, bin, dir, "", "verify", "--quiet")
		if code != 0 {
			t.Errorf("expected exit 0 on Verified+quiet, got %d", code)
		}
		if stdout != "" {
			t.Errorf("--quiet should produce zero stdout; got %q", stdout)
		}
		if stderr != "" {
			t.Errorf("--quiet should produce zero stderr; got %q", stderr)
		}
	})

	t.Run("--quiet stays silent on Blocked", func(t *testing.T) {
		// The other half of the --quiet contract: even when the outcome is
		// non-success, no output is produced. Without this test, a regression that
		// printed "Result: Blocked" to stderr under --quiet would be undetected -
		// the exit code would still be right.
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "v1")
		if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
			t.Fatalf("init: exit %d", code)
		}
		writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "TAMPERED")

		stdout, stderr, code := runSealSplit(t, bin, dir, "", "verify", "--quiet")
		if code != 1 {
			t.Errorf("expected exit 1 on Blocked+quiet, got %d", code)
		}
		if stdout != "" || stderr != "" {
			t.Errorf("--quiet should be silent even on Blocked; stdout=%q stderr=%q",
				stdout, stderr)
		}
	})

	t.Run("--json --quiet errors before any I/O", func(t *testing.T) {
		// The check lives in cobra's RunE (cmd/seal/main.go) - BEFORE FindLockfile
		// so misusing both flags must fail fast even in a directory with no
		// seal.json. The empty cwd here doubles as the proof.
		dir := t.TempDir() // no seal.json

		stdout, stderr, code := runSealSplit(t, bin, dir, "", "verify", "--json", "--quiet")
		if code != 2 {
			t.Errorf("expected exit 2 on --json+--quiet, got %d", code)
		}
		// The error message must name BOTH flags so the user knows what to remove.
		// We don't pin the exact wording - just that the two flag names appear.
		combined := stdout + stderr
		if !strings.Contains(combined, "--json") || !strings.Contains(combined, "--quiet") {
			t.Errorf("mutex error should mention both --json and --quiet; got:\n%s", combined)
		}
		// And critically: the error must NOT mention seal.json, which would prove
		// we touched the filesystem before failing the flag check.
		if strings.Contains(combined, "seal.json not found") {
			t.Errorf("mutex check should fail BEFORE filesystem access; got:\n%s", combined)
		}
	})
}

// TestE2E_PinLifecycle covers the boundary: bulk-noop byte invariance,
// targeted leaves siblings alone, --prune drops removed entries, no-prune
// preserves them.
// Each is covered by an internal/cli unit test already; this proves the
// contracts hold once cobra wiring, real stdin, real file locking, and the
// atomic rename all run together.
//
// Subtests share one buildSealBinary (~500ms amortised) but each stands up
// its own fixture and lockfile so failures localise cleanly. The
// `byteSnapshot` closure factors out the "read seal.json into a string" idiom
// every phase needs.
func TestE2E_PinLifecycle(t *testing.T) {
	bin := buildSealBinary(t)

	// byteSnapshot returns the current seal.json contents as a string,
	// t.Fatal'ing on read errors. Used to assert byte- identity before and after
	// operations expected to be no-ops.
	byteSnapshot := func(t *testing.T, dir string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(dir, "seal.json"))
		if err != nil {
			t.Fatalf("read seal.json: %v", err)
		}
		return string(raw)
	}

	// initOnePinnedBundle is the common starting state: project with one Claude
	// skill bundle, freshly pinned via `seal init`.
	// Returns the cwd path so each subtest can layer changes on top.
	initOnePinnedBundle := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "v1")
		if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
			t.Fatalf("init: exit %d", code)
		}
		return dir
	}

	t.Run("bulk pin after init is a no-op", func(t *testing.T) {
		// Right after init, every bundle is unchanged ⇒ bulk pin must print "No
		// changes" and exit 0 without rewriting seal.json. Byte-equality is the
		// load-bearing assertion - even an identical rewrite would change the
		// file's mtime and a non-zero `diff` would catch the regression in CI.
		dir := initOnePinnedBundle(t)
		before := byteSnapshot(t, dir)

		// No stdin: if pin tries to prompt, the test would hang.
		out, code := runSeal(t, bin, dir, "", "pin")
		if code != 0 {
			t.Fatalf("bulk pin: exit %d:\n%s", code, out)
		}
		if !strings.Contains(out, "No changes") {
			t.Errorf("missing 'No changes':\n%s", out)
		}
		if after := byteSnapshot(t, dir); after != before {
			t.Errorf("seal.json rewritten on no-op bulk pin")
		}
	})

	t.Run("targeted pin replaces only the specified entry", func(t *testing.T) {
		// Add a SECOND bundle on disk that the lockfile doesn't know about yet.
		// Then `seal pin <one>` against the original (already-pinned) bundle. Two
		// outcomes we need to verify:
		// - The original bundle is unchanged content-wise, so targeting it should
		// be a no-op write-wise.
		// - The NEW bundle (bar) should NOT land in seal.json, because targeted
		// mode pins only what the user explicitly named.
		dir := initOnePinnedBundle(t)
		writeFile(t, filepath.Join(dir, ".claude/skills/bar/SKILL.md"), "untracked")
		before := byteSnapshot(t, dir)

		// Target foo (unchanged) → "No changes" + byte-identical lockfile + bar
		// remains UN-pinned.
		out, code := runSeal(t, bin, dir, "", "pin", ".claude/skills/foo")
		if code != 0 {
			t.Fatalf("targeted pin: exit %d:\n%s", code, out)
		}
		if !strings.Contains(out, "No changes") {
			t.Errorf("expected 'No changes' on idempotent targeted pin:\n%s", out)
		}
		after := byteSnapshot(t, dir)
		if after != before {
			t.Errorf("seal.json mutated on idempotent targeted pin\nbefore:\n%s\nafter:\n%s",
				before, after)
		}
		if strings.Contains(after, "./.claude/skills/bar") {
			t.Errorf("targeted pin leaked untracked sibling bar into lockfile:\n%s", after)
		}
	})

	t.Run("bulk pin without --prune preserves removed entry", func(t *testing.T) {
		// Delete the bundle dir entirely.
		// bulk pin without --prune MUST leave the lockfile entry in place (reported
		// but not applied). Byte equality.
		// seal.json before/after is the contract.
		dir := initOnePinnedBundle(t)
		if err := os.RemoveAll(filepath.Join(dir, ".claude/skills/foo")); err != nil {
			t.Fatal(err)
		}
		before := byteSnapshot(t, dir)

		_, code := runSeal(t, bin, dir, "", "pin")
		if code != 0 {
			t.Fatalf("bulk pin (no prune): exit %d", code)
		}
		if after := byteSnapshot(t, dir); after != before {
			t.Errorf("seal.json mutated by bulk pin without --prune")
		}
	})

	t.Run("bulk pin --prune drops removed entry", func(t *testing.T) {
		// Same setup but with --prune. The lockfile must end up without the removed
		// bundle key.
		dir := initOnePinnedBundle(t)
		if err := os.RemoveAll(filepath.Join(dir, ".claude/skills/foo")); err != nil {
			t.Fatal(err)
		}

		out, code := runSeal(t, bin, dir, "y\n", "pin", "--prune")
		if code != 0 {
			t.Fatalf("bulk pin --prune: exit %d:\n%s", code, out)
		}
		after := byteSnapshot(t, dir)
		if strings.Contains(after, "./.claude/skills/foo") {
			t.Errorf("--prune failed to drop removed bundle; lockfile still references it:\n%s", after)
		}
	})
}

// TestE2E_VerifyJSONUnderNonVerifiedStates pins the JSON output shape for
// each non-Verified per-bundle Status: Mismatch, Removed,
// Unverified. The existing TestE2E_VerifyJSONShape only exercises the
// Verified path, so the failure-shape arrays - which are what downstream
// tooling (CI scripts, `jq` pipelines) actually parses - were unverified
// end-to-end.
//
// One outer function builds the binary once; three subtests each stand up a
// different fixture tree and assert that the corresponding bundle key appears
// in the matching JSON array
// AND that the top-level "status" headline + exit code line up with the.
//
// We unmarshal the JSON rather than substring-match because the arrays carry
// nested string values; substring assertions on raw
// JSON are fragile to formatting changes (indentation, key order), while
// Unmarshal hands us a Go map we can index directly.
func TestE2E_VerifyJSONUnderNonVerifiedStates(t *testing.T) {
	bin := buildSealBinary(t)

	// Helper to parse `verify --json` stdout into a map keyed.
	// "status" / "verified" / "unverified" / "removed" / "mismatch".
	// Local to this test because no other E2E test needs the parse.
	parseJSON := func(t *testing.T, raw string) map[string]any {
		t.Helper()
		var got map[string]any
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("unmarshal verify JSON: %v\nraw:\n%s", err, raw)
		}
		return got
	}

	// assertKeyInArray confirms the named JSON array contains the given bundle
	// key. We accept []any (Go's default.
	// JSON arrays) and do a typed comparison rather than rely on a stringy
	// contains check.
	assertKeyInArray := func(t *testing.T, got map[string]any, arrayName, wantKey string) {
		t.Helper()
		raw, ok := got[arrayName]
		if !ok {
			t.Fatalf("missing top-level array %q in: %+v", arrayName, got)
		}
		arr, ok := raw.([]any)
		if !ok {
			t.Fatalf("array %q is not a JSON array (type %T)", arrayName, raw)
		}
		for _, v := range arr {
			if s, ok := v.(string); ok && s == wantKey {
				return
			}
		}
		t.Errorf("expected %q in array %q; got %v", wantKey, arrayName, arr)
	}

	t.Run("Mismatch surfaces in mismatch array", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "original")

		if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
			t.Fatalf("init: exit %d", code)
		}
		// Mutate the file ⇒ Mismatch ⇒ Blocked under default policy.
		writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "TAMPERED")

		stdout, _, code := runSealSplit(t, bin, dir, "", "verify", "--json")
		if code != 1 {
			t.Errorf("exit %d, want 1 (Blocked):\n%s", code, stdout)
		}
		got := parseJSON(t, stdout)
		if got["status"] != "Blocked" {
			t.Errorf("status = %v, want Blocked", got["status"])
		}
		assertKeyInArray(t, got, "mismatch", "./.claude/skills/foo")
	})

	t.Run("Removed surfaces in removed array", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "original")

		if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
			t.Fatalf("init: exit %d", code)
		}
		// Delete the whole bundle directory ⇒ Removed.
		// Removed alone (no Mismatch/Unverified) is "Drift" ⇒ exit 0.
		if err := os.RemoveAll(filepath.Join(dir, ".claude/skills/foo")); err != nil {
			t.Fatal(err)
		}

		stdout, _, code := runSealSplit(t, bin, dir, "", "verify", "--json")
		if code != 0 {
			t.Errorf("exit %d, want 0 (Drift):\n%s", code, stdout)
		}
		got := parseJSON(t, stdout)
		if got["status"] != "Drift" {
			t.Errorf("status = %v, want Drift", got["status"])
		}
		assertKeyInArray(t, got, "removed", "./.claude/skills/foo")
	})

	t.Run("Unverified surfaces in unverified array", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "v1")

		// First init pins foo and writes the discovery glob into seal.json. The
		// glob is the bit that makes bar show up as Unverified later.
		if _, code := runSeal(t, bin, dir, "y\n", "init"); code != 0 {
			t.Fatalf("init: exit %d", code)
		}

		// Add a second dir that matches the same glob but is NOT in the lockfile ⇒
		// Unverified ⇒ Blocked under default.
		writeFile(t, filepath.Join(dir, ".claude/skills/bar/SKILL.md"), "v1")

		stdout, _, code := runSealSplit(t, bin, dir, "", "verify", "--json")
		if code != 1 {
			t.Errorf("exit %d, want 1 (Blocked):\n%s", code, stdout)
		}
		got := parseJSON(t, stdout)
		if got["status"] != "Blocked" {
			t.Errorf("status = %v, want Blocked", got["status"])
		}
		assertKeyInArray(t, got, "unverified", "./.claude/skills/bar")
	})
}

// TestE2E_InitWarnPolicyAcceptsMismatch is the mirror image.
// TestE2E_MutationBlocksVerify: same fixture, same mutation, but init uses
// --warn so the lockfile records policy: "warn".
// that policy.
// "Warning" with exit 0 - drift is reported but never gates CI.
//
// This is the only place the warn/block boundary is tested end- to-end.
// internal/cli has unit coverage of derivedOutcome's truth table; this test
// proves the policy actually round-trips through init's writer, the on-disk
// JSON, and verify's reader.
func TestE2E_InitWarnPolicyAcceptsMismatch(t *testing.T) {
	bin := buildSealBinary(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "original")

	// init --warn (the only difference from the default block path that
	// TestE2E_InitThenVerify already covers).
	if _, code := runSeal(t, bin, dir, "y\n", "init", "--warn"); code != 0 {
		t.Fatalf("init --warn: exit %d", code)
	}

	// Pin the policy field landed correctly in seal.json. We substring-match
	// rather than parse JSON because the parsing is already covered by
	// internal/seal tests - here we just want a smoke check that the --warn flag
	// actually mutated the produced file.
	raw, err := os.ReadFile(filepath.Join(dir, "seal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"policy": "warn"`) {
		t.Fatalf("seal.json should contain warn policy; got:\n%s", raw)
	}

	// Mutate the tracked file. Same drift as the block test.
	writeFile(t, filepath.Join(dir, ".claude/skills/foo/SKILL.md"), "TAMPERED")

	// Under warn policy, verify reports drift but exits 0. This is the
	// load-bearing assertion - a regression here would silently turn warn mode
	// into block mode, which CI users would catch only as unexplained pipeline
	// failures.
	out, code := runSeal(t, bin, dir, "", "verify")
	if code != 0 {
		t.Fatalf("expected exit 0 under warn policy, got %d; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Warning") {
		t.Errorf("expected 'Warning' outcome in output:\n%s", out)
	}
	// "(warn mode)" trailer makes the policy visible in the human output so an
	// operator can see WHY a mismatch wasn't a fail.
	if !strings.Contains(out, "(warn mode)") {
		t.Errorf("expected '(warn mode)' trailer in output:\n%s", out)
	}
}
