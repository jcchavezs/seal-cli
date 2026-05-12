#!/usr/bin/env bash

set -u

usage() {
  cat <<'EOF'
seal-labs.sh - build seal and generate interactive CLI labs

Usage:
  scripts/seal-labs.sh [--root DIR] [--all] [--lab NAME ...]
  scripts/seal-labs.sh --list
  scripts/seal-labs.sh --matrix

Options:
  --root DIR     Directory where labs are created. Default: /tmp/seal-labs-<timestamp>
  --lab NAME     Generate one lab. May be repeated.
  --all          Generate every lab.
  --list         Print lab names and titles.
  --matrix       Print the coverage matrix.
  -h, --help     Show this help.

Each lab is a standalone project directory with:
  README.md      Manual instructions and expected result.
  demo.sh        Runnable demonstration of the lab.

The script always compiles ./cmd/seal into <root>/bin/seal before creating labs.
EOF
}

LABS=(
  happy-path
  verify-json-output
  verify-quiet-output
  verify-json-quiet-conflict
  missing-lockfile-fatal
  init-existing-lockfile
  init-decline
  init-empty-project
  mismatch-modified-file
  mismatch-added-file
  mismatch-file-removed-from-lockfile
  mismatch-removed-file
  mismatch-unsupported-symlink
  unverified-new-discovery-dir
  unverified-new-file-root
  removed-bundle-drift
  warning-policy-mismatch
  warning-policy-unverified
  mixed-removed-mismatch-block
  discovery-whole-dir-added-file-mismatch
  discovery-hidden-child
  discovery-empty-dir-ignored
  discovery-question-is-literal
  regular-file-root-happy
  git-entry-excluded
  metadata-ignored
  invalid-json
  invalid-unknown-field
  invalid-unknown-bundle-field
  invalid-missing-policy
  invalid-bad-version
  invalid-bad-policy
  invalid-bad-hash
  invalid-contenthash
  invalid-empty-files
  invalid-dot-bundle-key
  invalid-traversal-bundle-key
  invalid-backslash-file-path
  invalid-discovery-trailing-slash
  invalid-discovery-recursive-glob
  invalid-discovery-leading-dot-slash
  invalid-discovery-leading-slash
  invalid-discovery-backslash
  invalid-discovery-repeated-separator
  invalid-discovery-dot-segment
  invalid-required-field-shapes
  invalid-bundle-key-shapes
  invalid-file-path-shapes
  invalid-duplicate-json-keys
  true-case-key-mismatch
  pin-bulk-noop
  pin-bulk-add-new
  pin-bulk-update-modified
  pin-bulk-removed-no-prune
  pin-bulk-removed-prune
  pin-targeted-dir
  pin-targeted-file-root
  pin-targeted-modified
  pin-targeted-noop
  pin-targeted-outside-project
  pin-targeted-project-root-rejected
  pin-targeted-symlink-rejected
  pin-decline-aborts
  pin-invalid-lockfile-fatal
  pin-bulk-unsupported-symlink-aborts
  subdir-no-lockfile-traversal
  invalid-static-coverage
  invalid-static-coverage-wrong-pattern
  invalid-static-coverage-removed-key
  invalid-narrowed-discovery
  removed-covered-bundle-drift
  discovery-file-sibling-unverified
  invalid-nested-bundle-keys
  invalid-overlapping-discovery
  invalid-overlapping-discovery-wildcard-literal
  pin-targeted-adds-discovery
  pin-targeted-nested-rejected
)

lab_title() {
  case "$1" in
    happy-path) echo "Init and verify clean project" ;;
    verify-json-output) echo "Verify JSON output on stdout" ;;
    verify-quiet-output) echo "Verify quiet mode is silent" ;;
    verify-json-quiet-conflict) echo "Verify rejects --json with --quiet" ;;
    missing-lockfile-fatal) echo "Verify without seal.json is fatal" ;;
    init-existing-lockfile) echo "Init refuses to overwrite seal.json" ;;
    init-decline) echo "Init prompt decline writes nothing" ;;
    init-empty-project) echo "Init can create an empty lockfile" ;;
    mismatch-modified-file) echo "Tracked file modification blocks" ;;
    mismatch-added-file) echo "Added file inside sealed root blocks" ;;
    mismatch-file-removed-from-lockfile) echo "File removed from lockfile blocks" ;;
    mismatch-removed-file) echo "Removed file inside sealed root blocks" ;;
    mismatch-unsupported-symlink) echo "Unsupported symlink inside sealed root blocks" ;;
    unverified-new-discovery-dir) echo "New discovered directory is unverified" ;;
    unverified-new-file-root) echo "New discovered regular file is unverified" ;;
    removed-bundle-drift) echo "Removed bundle reports drift" ;;
    warning-policy-mismatch) echo "Warn policy permits mismatch with warning" ;;
    warning-policy-unverified) echo "Warn policy permits unverified with warning" ;;
    mixed-removed-mismatch-block) echo "Removed plus mismatch still blocks" ;;
    discovery-whole-dir-added-file-mismatch) echo "Whole-directory discovery treats additions as mismatch" ;;
    discovery-hidden-child) echo "Wildcard discovery includes hidden children" ;;
    discovery-empty-dir-ignored) echo "Empty discovered directories are ignored" ;;
    discovery-question-is-literal) echo "Question mark is literal in discovery" ;;
    regular-file-root-happy) echo "Regular-file sealed root verifies" ;;
    git-entry-excluded) echo ".git entries inside bundles are excluded" ;;
    metadata-ignored) echo "Metadata-only chmod does not affect verification" ;;
    invalid-json) echo "Invalid JSON lockfile is fatal" ;;
    invalid-unknown-field) echo "Unknown top-level field is fatal" ;;
    invalid-unknown-bundle-field) echo "Unknown bundle field is fatal" ;;
    invalid-missing-policy) echo "Missing policy is fatal" ;;
    invalid-bad-version) echo "Unsupported version is fatal" ;;
    invalid-bad-policy) echo "Bad policy is fatal" ;;
    invalid-bad-hash) echo "Malformed hash is fatal" ;;
    invalid-contenthash) echo "Wrong contentHash is fatal" ;;
    invalid-empty-files) echo "Empty files map is fatal" ;;
    invalid-dot-bundle-key) echo "Exact dot bundle key is fatal" ;;
    invalid-traversal-bundle-key) echo "Traversal bundle key is fatal" ;;
    invalid-backslash-file-path) echo "Backslash file path is fatal" ;;
    invalid-discovery-trailing-slash) echo "Trailing-slash discovery is fatal" ;;
    invalid-discovery-recursive-glob) echo "Recursive discovery glob is fatal" ;;
    invalid-discovery-leading-dot-slash) echo "Leading ./ discovery is fatal" ;;
    invalid-discovery-leading-slash) echo "Leading / discovery is fatal" ;;
    invalid-discovery-backslash) echo "Backslash discovery is fatal" ;;
    invalid-discovery-repeated-separator) echo "Repeated-separator discovery is fatal" ;;
    invalid-discovery-dot-segment) echo "Dot/dot-dot discovery segment is fatal" ;;
    invalid-required-field-shapes) echo "Missing/null/wrong-type required fields are fatal" ;;
    invalid-bundle-key-shapes) echo "Bundle-key shape variants are fatal" ;;
    invalid-file-path-shapes) echo "File-path shape variants are fatal" ;;
    invalid-duplicate-json-keys) echo "Duplicate JSON object keys are fatal" ;;
    true-case-key-mismatch) echo "Wrong-case recorded key becomes removed plus unverified" ;;
    pin-bulk-noop) echo "Bulk pin no-op does not rewrite trust" ;;
    pin-bulk-add-new) echo "Bulk pin adds discovered bundle" ;;
    pin-bulk-update-modified) echo "Bulk pin updates modified bundle" ;;
    pin-bulk-removed-no-prune) echo "Bulk pin preserves removed bundle by default" ;;
    pin-bulk-removed-prune) echo "Bulk pin --prune removes missing bundle" ;;
    pin-targeted-dir) echo "Targeted pin adds a directory root" ;;
    pin-targeted-file-root) echo "Targeted pin adds a regular-file root" ;;
    pin-targeted-modified) echo "Targeted pin updates one modified bundle" ;;
    pin-targeted-noop) echo "Targeted pin no-op does not prompt" ;;
    pin-targeted-outside-project) echo "Targeted pin rejects outside path" ;;
    pin-targeted-project-root-rejected) echo "Targeted pin rejects project root" ;;
    pin-targeted-symlink-rejected) echo "Targeted pin rejects symlink roots" ;;
    pin-decline-aborts) echo "Pin prompt decline leaves lockfile unchanged" ;;
    pin-invalid-lockfile-fatal) echo "Pin refuses invalid existing lockfile" ;;
    pin-bulk-unsupported-symlink-aborts) echo "Bulk pin aborts on unsupported symlink" ;;
    subdir-no-lockfile-traversal) echo "Verify does not search parent directories" ;;
    invalid-static-coverage) echo "Uncovered bundle key is fatal" ;;
    invalid-static-coverage-wrong-pattern) echo "Existing discovery patterns must cover every bundle key" ;;
    invalid-static-coverage-removed-key) echo "Uncovered removed bundle key is fatal" ;;
    invalid-narrowed-discovery) echo "Narrowed discovery leaves invalid bundle key" ;;
    removed-covered-bundle-drift) echo "Covered removed bundle remains drift" ;;
    discovery-file-sibling-unverified) echo "Broad file-root discovery catches sibling insertions" ;;
    invalid-nested-bundle-keys) echo "Nested bundle keys are fatal" ;;
    invalid-overlapping-discovery) echo "Overlapping discovery roots are fatal" ;;
    invalid-overlapping-discovery-wildcard-literal) echo "Wildcard parent plus literal descendant discovery is fatal" ;;
    pin-targeted-adds-discovery) echo "Targeted pin adds exact discovery coverage" ;;
    pin-targeted-nested-rejected) echo "Targeted pin rejects nested sealed roots" ;;
    *) echo "$1" ;;
  esac
}

lab_category() {
  case "$1" in
    happy-path|removed-bundle-drift|removed-covered-bundle-drift|warning-*|mixed-*) echo "verification outcomes" ;;
    mismatch-*|unverified-*) echo "per-bundle states" ;;
    discovery-*|regular-file-root-happy|git-entry-excluded|metadata-ignored) echo "discovery and hashing" ;;
    invalid-*) echo "lockfile validity" ;;
    pin-*) echo "pin workflow" ;;
    verify-*|missing-lockfile-fatal|subdir-no-lockfile-traversal|init-*|true-case-*) echo "CLI behavior" ;;
    *) echo "misc" ;;
  esac
}

lab_covers() {
  case "$1" in
    happy-path) echo "init heuristic discovery; deterministic pin; verify outcome Verified; exit 0" ;;
    verify-json-output) echo "--json stdout contract; arrays for verified/unverified/removed/mismatch" ;;
    verify-quiet-output) echo "--quiet output suppression for success and blocked outcomes" ;;
    verify-json-quiet-conflict) echo "mutually exclusive verify output modes; fatal exit 2 before filesystem dependency" ;;
    missing-lockfile-fatal) echo "standalone tool requires seal.json in cwd; exit 2" ;;
    init-existing-lockfile) echo "init refuses overwrite" ;;
    init-decline) echo "interactive confirmation default-deny" ;;
    mismatch-modified-file) echo "Mismatch: modified tracked file; block policy => Blocked exit 1" ;;
    mismatch-added-file) echo "Mismatch: added file under recorded directory root; block policy => Blocked" ;;
    mismatch-file-removed-from-lockfile) echo "Mismatch: disk has a file that was removed from the lockfile files map" ;;
    mismatch-removed-file) echo "Mismatch: missing tracked file inside existing root; block policy => Blocked" ;;
    mismatch-unsupported-symlink) echo "unsupported non-regular entry during verify => Mismatch" ;;
    unverified-new-discovery-dir) echo "discovery match with no bundle entry => Unverified; block policy => Blocked" ;;
    unverified-new-file-root) echo "regular file matched by discovery as single-file root => Unverified" ;;
    removed-bundle-drift) echo "recorded root absent from disk => Removed; overall Drift exit 0" ;;
    warning-policy-mismatch) echo "policy warn converts Mismatch from Blocked to Warning exit 0" ;;
    warning-policy-unverified) echo "policy warn converts Unverified from Blocked to Warning exit 0" ;;
    mixed-removed-mismatch-block) echo "Removed does not block alone, but Mismatch still produces Blocked" ;;
    discovery-whole-dir-added-file-mismatch) echo "pattern 'dir' produces one recursive root; additions are Mismatch, not Unverified" ;;
    discovery-hidden-child) echo "'*' matches dot-prefixed names" ;;
    discovery-empty-dir-ignored) echo "empty/all-excluded dirs are not materialized as bundles" ;;
    discovery-question-is-literal) echo "?, [, ], {, } are literals; only * is wildcard" ;;
    regular-file-root-happy) echo "file-root bundle representation uses files['.']" ;;
    git-entry-excluded) echo ".git file/dir under bundle root is excluded" ;;
    metadata-ignored) echo "permissions/executable-bit metadata is out of scope" ;;
    invalid-json) echo "decode failure => fatal exit 2" ;;
    invalid-unknown-field) echo "unknown top-level fields are rejected" ;;
    invalid-unknown-bundle-field) echo "unknown bundle fields are rejected" ;;
    invalid-missing-policy) echo "required top-level field policy" ;;
    invalid-bad-version) echo "version must be supported integer 1" ;;
    invalid-bad-policy) echo "policy must be block or warn" ;;
    invalid-bad-hash) echo "hashes must be sha256:<64 lowercase hex>" ;;
    invalid-contenthash) echo "contentHash must match aggregate files map" ;;
    invalid-empty-files) echo "bundle files map must be non-empty" ;;
    invalid-dot-bundle-key) echo "Seal v1 rejects exact '.' bundle key" ;;
    invalid-traversal-bundle-key) echo "bundle keys reject .. segments" ;;
    invalid-backslash-file-path) echo "files paths must use forward slashes" ;;
    invalid-discovery-trailing-slash) echo "discovery patterns reject trailing slash" ;;
    invalid-discovery-recursive-glob) echo "discovery patterns reject **" ;;
    invalid-discovery-leading-dot-slash) echo "discovery patterns reject leading ./" ;;
    invalid-discovery-leading-slash) echo "discovery patterns reject leading /" ;;
    invalid-discovery-backslash) echo "discovery patterns reject backslash" ;;
    invalid-discovery-repeated-separator) echo "discovery patterns reject repeated separators" ;;
    invalid-discovery-dot-segment) echo "discovery patterns reject . and .. segments" ;;
    invalid-required-field-shapes) echo "required fields missing, null, or wrong type are rejected before verification" ;;
    invalid-bundle-key-shapes) echo "bundle keys reject missing prefix, absolute-looking paths, backslashes, repeated separators, dot segments, and trailing slash" ;;
    invalid-file-path-shapes) echo "files map paths reject empty, dot-as-directory, leading ./, absolute, repeated separators, dot segments, dot-dot segments, and trailing slash" ;;
    invalid-duplicate-json-keys) echo "decoder rejects duplicate object keys instead of taking last value" ;;
    true-case-key-mismatch) echo "strict case-sensitive path resolution reports wrong-case key as Removed and true-case discovery as Unverified" ;;
    pin-bulk-noop) echo "pin with no args reports No changes when all discovery matches are pinned" ;;
    pin-bulk-add-new) echo "bulk pin materializes a new discovery match after confirmation" ;;
    pin-bulk-update-modified) echo "bulk pin recalculates changed hashes after confirmation" ;;
    pin-bulk-removed-no-prune) echo "bulk pin reports Removed but preserves it without --prune" ;;
    pin-bulk-removed-prune) echo "bulk pin --prune deletes missing bundle entries" ;;
    pin-targeted-dir) echo "pin <path> adds a directory sealed root" ;;
    pin-targeted-file-root) echo "pin <path> adds a regular-file sealed root" ;;
    pin-targeted-modified) echo "pin <path> updates only the named modified bundle" ;;
    pin-targeted-noop) echo "pin <path> prints No changes and does not prompt if unchanged" ;;
    pin-targeted-outside-project) echo "path args outside cwd are rejected" ;;
    pin-targeted-project-root-rejected) echo "project root cannot be a bundle root" ;;
    pin-targeted-symlink-rejected) echo "symlink roots/intermediate segments are rejected during pin" ;;
    pin-decline-aborts) echo "pin confirmation decline aborts mutation and preserves old hashes" ;;
    pin-invalid-lockfile-fatal) echo "pin validates existing seal.json and exits 2 before writing if invalid" ;;
    pin-bulk-unsupported-symlink-aborts) echo "pin aborts on unsupported non-regular files instead of writing a partial lockfile" ;;
    subdir-no-lockfile-traversal) echo "verify run below project root must not walk upward to find seal.json" ;;
    invalid-static-coverage) echo "lockfile validation rejects bundle keys not statically produced by discovery" ;;
    invalid-static-coverage-wrong-pattern) echo "existing discovery patterns are named when they do not statically produce a recorded key" ;;
    invalid-static-coverage-removed-key) echo "uncovered recorded keys exit 2 before they can classify as Removed/Drift" ;;
    invalid-narrowed-discovery) echo "narrowing discovery while leaving old bundle keys behind is a static coverage validation error" ;;
    removed-covered-bundle-drift) echo "a missing recorded key is allowed to classify as Removed when discovery still statically covers it" ;;
    discovery-file-sibling-unverified) echo "docs/*.md discovery catches a new sibling file root as Unverified" ;;
    invalid-nested-bundle-keys) echo "lockfile validation rejects nested recorded bundle keys before verification classification" ;;
    invalid-overlapping-discovery) echo "lockfile validation rejects discovery patterns that can produce nested sealed roots" ;;
    invalid-overlapping-discovery-wildcard-literal) echo "lockfile validation rejects wildcard parents such as skills/* paired with literal descendants such as skills/foo/*" ;;
    pin-targeted-adds-discovery) echo "targeted pin adds the narrowest exact discovery pattern when no existing pattern covers the new key" ;;
    pin-targeted-nested-rejected) echo "targeted pin rejects targets inside an existing recorded sealed root" ;;
    *) echo "" ;;
  esac
}

lab_expected() {
  case "$1" in
    invalid-*|missing-lockfile-fatal|verify-json-quiet-conflict|pin-targeted-outside-project|pin-targeted-project-root-rejected|pin-targeted-symlink-rejected|pin-targeted-nested-rejected|pin-decline-aborts|pin-invalid-lockfile-fatal|pin-bulk-unsupported-symlink-aborts|init-existing-lockfile|init-decline|subdir-no-lockfile-traversal) echo "Fatal/Error exit 2" ;;
    discovery-file-sibling-unverified) echo "Blocked exit 1" ;;
    true-case-key-mismatch) echo "Blocked exit 1" ;;
    mismatch-*|unverified-*|mixed-removed-mismatch-block) echo "Blocked exit 1" ;;
    warning-*) echo "Warning exit 0" ;;
    removed-bundle-drift|removed-covered-bundle-drift|pin-bulk-removed-no-prune) echo "Drift exit 0" ;;
    *) echo "Success/Verified exit 0" ;;
  esac
}

list_labs() {
  local lab
  for lab in "${LABS[@]}"; do
    printf '%-48s %s\n' "$lab" "$(lab_title "$lab")"
  done
}

print_matrix() {
  printf '| Lab | Category | Covers | Expected |\n'
  printf '| --- | --- | --- | --- |\n'
  local lab
  for lab in "${LABS[@]}"; do
    printf '| `%s` | %s | %s | %s |\n' \
      "$lab" "$(lab_category "$lab")" "$(lab_covers "$lab")" "$(lab_expected "$lab")"
  done
}

has_lab() {
  local wanted=$1
  local lab
  for lab in "${LABS[@]}"; do
    if [ "$lab" = "$wanted" ]; then
      return 0
    fi
  done
  return 1
}

sha256_file() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

sha256_text() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
  else
    sha256sum | awk '{print $1}'
  fi
}

write_file() {
  local path=$1
  local content=$2
  mkdir -p "$(dirname "$path")"
  printf '%s' "$content" > "$path"
}

seed_skill() {
  local dir=$1
  local name=${2:-foo}
  local content=${3:-"v1
"}
  write_file "$dir/.claude/skills/$name/SKILL.md" "$content"
}

seed_two_skills() {
  local dir=$1
  seed_skill "$dir" foo "foo-v1
"
  seed_skill "$dir" bar "bar-v1
"
}

write_lockfile_single() {
  local dir=$1
  local policy=$2
  local discovery=$3
  local key=$4
  local file_key=$5
  local file_path=$6
  local file_hash content_input content_hash discovery_json
  file_hash=$(sha256_file "$file_path")
  content_input="$file_key:sha256:$file_hash"
  content_hash=$(printf '%s' "$content_input" | sha256_text)
  if [ -n "$discovery" ]; then
    discovery_json="  \"discovery\": [\"$discovery\"],
"
  else
    discovery_json=""
  fi
  cat > "$dir/seal.json" <<EOF
{
  "version": 1,
${discovery_json}  "policy": "$policy",
  "bundles": {
    "$key": {
      "contentHash": "sha256:$content_hash",
      "files": {
        "$file_key": "sha256:$file_hash"
      }
    }
  }
}
EOF
}

write_readme() {
  local dir=$1
  local lab=$2
  local title=$3
  local covers=$4
  local expected=$5
  {
    printf '# %s\n\n' "$title"
    printf '**Lab:** `%s`\n\n' "$lab"
    printf '**Covers:** %s\n\n' "$covers"
    printf '**Expected:** %s\n\n' "$expected"
    cat
    cat <<'EOF'

## Demo

Run:

```sh
./demo.sh
```

The demo resets this lab directory before running its commands, so it is safe
to rerun after manual experiments.
EOF
  } > "$dir/README.md"
}

write_demo() {
  local dir=$1
  {
    cat <<'EOF'
#!/usr/bin/env bash
set -u

LAB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SEAL="$LAB_DIR/../bin/seal"
cd "$LAB_DIR" || exit 1

LAST_OUTPUT=""
LAST_CODE=0

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'PASS: %s\n' "$*"
}

show_cmd() {
  printf '\n$ %s\n' "$*"
}

run_shell_expect() {
  local want=$1
  shift
  local cmd="$*"
  show_cmd "$cmd"
  LAST_OUTPUT="$(eval "$cmd" 2>&1)"
  LAST_CODE=$?
  printf '%s\n' "$LAST_OUTPUT"
  if [ "$LAST_CODE" -ne "$want" ]; then
    fail "expected exit $want, got $LAST_CODE"
  fi
}

run_shell_accept() {
  local allowed=$1
  shift
  local cmd="$*"
  show_cmd "$cmd"
  LAST_OUTPUT="$(eval "$cmd" 2>&1)"
  LAST_CODE=$?
  printf '%s\n' "$LAST_OUTPUT"
  case " $allowed " in
    *" $LAST_CODE "*) ;;
    *) fail "expected exit in {$allowed}, got $LAST_CODE" ;;
  esac
}

contains() {
  case "$LAST_OUTPUT" in
    *"$1"*) ;;
    *) fail "output did not contain: $1" ;;
  esac
}

not_contains() {
  case "$LAST_OUTPUT" in
    *"$1"*) fail "output unexpectedly contained: $1" ;;
    *) ;;
  esac
}

write_file() {
  mkdir -p "$(dirname "$1")"
  printf '%s' "$2" > "$1"
}

reset_all() {
  find . -mindepth 1 ! -name README.md ! -name demo.sh -exec rm -rf {} + 2>/dev/null || true
}

seed_skill() {
  local name=${1:-foo}
  local content=${2:-"v1
"}
  write_file ".claude/skills/$name/SKILL.md" "$content"
}

seed_two_skills() {
  seed_skill foo "foo-v1
"
  seed_skill bar "bar-v1
"
}

sha256_file() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

sha256_text() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
  else
    sha256sum | awk '{print $1}'
  fi
}

write_lockfile_single() {
  local policy=$1
  local discovery=$2
  local key=$3
  local file_key=$4
  local file_path=$5
  local file_hash content_input content_hash discovery_json
  file_hash=$(sha256_file "$file_path")
  content_input="$file_key:sha256:$file_hash"
  content_hash=$(printf '%s' "$content_input" | sha256_text)
  if [ -n "$discovery" ]; then
    discovery_json="  \"discovery\": [\"$discovery\"],
"
  else
    discovery_json=""
  fi
  cat > seal.json <<LOCK
{
  "version": 1,
${discovery_json}  "policy": "$policy",
  "bundles": {
    "$key": {
      "contentHash": "sha256:$content_hash",
      "files": {
        "$file_key": "sha256:$file_hash"
      }
    }
  }
}
LOCK
}

EOF
    cat
  } > "$dir/demo.sh"
  chmod +x "$dir/demo.sh"
}

seed_initial_for_lab() {
  local dir=$1
  local lab=$2
  case "$lab" in
    missing-lockfile-fatal|init-empty-project|invalid-json|invalid-missing-policy|invalid-bad-version|invalid-bad-policy|invalid-empty-files|invalid-dot-bundle-key|invalid-traversal-bundle-key|invalid-backslash-file-path|invalid-discovery-*|invalid-overlapping-discovery|invalid-required-field-shapes|invalid-bundle-key-shapes|invalid-file-path-shapes|invalid-duplicate-json-keys)
      :
      ;;
    unverified-new-file-root|regular-file-root-happy|pin-targeted-file-root)
      write_file "$dir/prompts/a.md" "prompt-a
"
      ;;
    discovery-whole-dir-added-file-mismatch)
      write_file "$dir/.cursor/rules/rule-a.mdc" "rule-a
"
      ;;
    discovery-hidden-child)
      write_file "$dir/.agents/skills/.hidden/SKILL.md" "hidden
"
      ;;
    discovery-empty-dir-ignored)
      mkdir -p "$dir/.claude/skills/empty"
      ;;
    discovery-question-is-literal)
      write_file "$dir/literal?dir/item/SKILL.md" "literal
"
      ;;
    pin-targeted-dir|pin-targeted-adds-discovery)
      write_file "$dir/custom/skill/SKILL.md" "custom
"
      ;;
    pin-targeted-nested-rejected)
      write_file "$dir/skills/foo/SKILL.md" "root
"
      write_file "$dir/skills/foo/bar/SKILL.md" "nested
"
      ;;
    invalid-static-coverage)
      write_file "$dir/custom/skill/SKILL.md" "custom
"
      write_lockfile_single "$dir" block "" "./custom/skill" "SKILL.md" "$dir/custom/skill/SKILL.md"
      ;;
    invalid-nested-bundle-keys)
      write_file "$dir/skills/SKILL.md" "parent
"
      write_file "$dir/skills/foo/SKILL.md" "child
"
      ;;
    *)
      seed_skill "$dir" foo "v1
"
      ;;
  esac
}

create_lab() {
  local lab=$1
  local dir="$ROOT/$lab"
  rm -rf "$dir"
  mkdir -p "$dir"
  seed_initial_for_lab "$dir" "$lab"

  local title covers expected
  title=$(lab_title "$lab")
  covers=$(lab_covers "$lab")
  expected=$(lab_expected "$lab")

  case "$lab" in
    happy-path)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

```sh
../bin/seal init
../bin/seal verify
```

Answer `y` to the init prompt. Verify should print `Result: Verified` and exit 0.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init'
contains "Wrote seal.json"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "clean init + verify"
EOF
      ;;

    verify-json-output)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize, then run:

```sh
../bin/seal verify --json
```

JSON should be written to stdout and include `status`, `verified`,
`unverified`, `removed`, and `mismatch`.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 0 '"$SEAL" verify --json'
contains '"status": "Verified"'
contains '"verified"'
contains '"unverified"'
contains '"removed"'
contains '"mismatch"'
pass "verify --json shape"
EOF
      ;;

    verify-quiet-output)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Run `../bin/seal verify --quiet` in both clean and tampered states.
The command should communicate only by exit code.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 0 'out=$("$SEAL" verify --quiet 2>&1); code=$?; [ "$code" -eq 0 ] && [ -z "$out" ]'
write_file ".claude/skills/foo/SKILL.md" "tampered
"
run_shell_expect 0 'out=$("$SEAL" verify --quiet 2>&1); code=$?; [ "$code" -eq 1 ] && [ -z "$out" ]'
pass "quiet mode is silent on success and blocked outcomes"
EOF
      ;;

    verify-json-quiet-conflict)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

```sh
../bin/seal verify --json --quiet
```

This should fail before reading any lockfile.
EOF
      write_demo "$dir" <<'EOF'
reset_all
run_shell_expect 2 '"$SEAL" verify --json --quiet'
contains "--json"
contains "--quiet"
not_contains "seal.json not found"
pass "mutually exclusive verify flags reject early"
EOF
      ;;

    missing-lockfile-fatal)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

```sh
../bin/seal verify
```

With no `seal.json` in this directory, verify should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
run_shell_expect 2 '"$SEAL" verify'
contains "seal.json not found"
pass "missing lockfile is fatal"
EOF
      ;;

    init-existing-lockfile)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize once, then try `../bin/seal init` again. The second run should
exit 2 and leave the existing lockfile untouched.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
before=$(cat seal.json)
run_shell_expect 2 'printf "y\n" | "$SEAL" init'
contains "already exists"
after=$(cat seal.json)
[ "$before" = "$after" ] || fail "seal.json changed"
pass "init refuses overwrite"
EOF
      ;;

    init-decline)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Run `../bin/seal init` and answer `n`. No lockfile should be written.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 2 'printf "n\n" | "$SEAL" init'
contains "init aborted"
[ ! -e seal.json ] || fail "seal.json should not exist"
pass "init decline is default-deny"
EOF
      ;;

    init-empty-project)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Run `../bin/seal init` in an empty project and answer `y`. The resulting
lockfile should contain no discovery patterns and no bundles.
EOF
      write_demo "$dir" <<'EOF'
reset_all
run_shell_expect 0 'printf "y\n" | "$SEAL" init'
contains "0 bundles"
contains "0 discovery patterns"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "empty project init works"
EOF
      ;;

    mismatch-modified-file)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

```sh
../bin/seal init
../bin/seal verify
printf 'tampered\n' > .claude/skills/foo/SKILL.md
../bin/seal verify
```

The final verify should be `Blocked` with one mismatch.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
write_file ".claude/skills/foo/SKILL.md" "tampered
"
run_shell_expect 1 '"$SEAL" verify'
contains "Mismatch"
contains "Result: Blocked"
pass "modified file blocks"
EOF
      ;;

    mismatch-added-file)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

After init, add a new file inside the already pinned bundle directory.
Verify should report a mismatch, not an unverified bundle.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
write_file ".claude/skills/foo/extra.md" "new file
"
run_shell_expect 1 '"$SEAL" verify --verbose'
contains "Mismatch"
contains "added:"
contains "Result: Blocked"
pass "added file inside sealed root blocks"
EOF
      ;;

    mismatch-file-removed-from-lockfile)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

This lab simulates a reviewed file being removed from the lockfile while the
file remains on disk. The lockfile is internally consistent, but verify should
see the still-present disk file as `added:` and block.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file ".claude/skills/foo/SKILL.md" "skill
"
write_file ".claude/skills/foo/extra.md" "extra
"
write_lockfile_single block ".claude/skills/*" "./.claude/skills/foo" "SKILL.md" ".claude/skills/foo/SKILL.md"
run_shell_expect 1 '"$SEAL" verify --verbose'
contains "Mismatch"
contains "added:"
contains "extra.md"
contains "Result: Blocked"
pass "file removed from lockfile is detected as added on disk"
EOF
      ;;

    mismatch-removed-file)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

After init, delete a tracked file inside the bundle. Verify should block with
a mismatch and verbose `missing:` detail.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
rm .claude/skills/foo/SKILL.md
run_shell_expect 1 '"$SEAL" verify --verbose'
contains "Mismatch"
contains "missing:"
contains "Result: Blocked"
pass "removed tracked file blocks"
EOF
      ;;

    mismatch-unsupported-symlink)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

After init, create a symlink inside the pinned bundle. Verify should classify
the bundle as mismatch. On platforms that disallow symlink creation, this lab
is skipped by `demo.sh`.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
if ln -s SKILL.md .claude/skills/foo/link.md 2>/dev/null; then
  run_shell_expect 1 '"$SEAL" verify'
  contains "Mismatch"
  contains "Result: Blocked"
  pass "unsupported symlink blocks"
else
  pass "symlink unavailable; lab skipped on this platform"
fi
EOF
      ;;

    unverified-new-discovery-dir)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

After init, create a new direct child under `.claude/skills/`. Discovery
`.claude/skills/*` should surface it as `Unverified`.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill foo "foo
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
seed_skill bar "bar
"
run_shell_expect 1 '"$SEAL" verify'
contains "Unverified"
contains "Result: Blocked"
pass "new discovered directory is unverified"
EOF
      ;;

    unverified-new-file-root)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

This lab starts from a file-root discovery pattern (`prompts/*`). Pin one file,
then add another. The new file should be `Unverified`.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "prompts/a.md" "a
"
cat > seal.json <<'LOCK'
{
  "version": 1,
  "discovery": ["prompts/*"],
  "policy": "block",
  "bundles": {}
}
LOCK
run_shell_expect 0 'printf "y\n" | "$SEAL" pin >/dev/null'
write_file "prompts/b.md" "b
"
run_shell_expect 1 '"$SEAL" verify'
contains "Unverified"
contains "./prompts/b.md"
pass "new discovered regular file is unverified"
EOF
      ;;

    removed-bundle-drift)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

After init, delete the entire pinned bundle directory. Verify should report
`Drift`, not `Blocked`, and exit 0.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
rm -rf .claude/skills/foo
run_shell_expect 0 '"$SEAL" verify'
contains "Removed"
contains "Result: Drift"
pass "removed bundle is drift"
EOF
      ;;

    warning-policy-mismatch)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize with `--warn`, modify a tracked file, then verify. The mismatch is
reported, but exit code remains 0.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init --warn >/dev/null'
write_file ".claude/skills/foo/SKILL.md" "tampered
"
run_shell_expect 0 '"$SEAL" verify'
contains "Mismatch"
contains "Result: Warning"
pass "warn policy permits mismatch"
EOF
      ;;

    warning-policy-unverified)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize with `--warn`, add a new discovered bundle, then verify. The new
bundle is reported as unverified, but exit code remains 0.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill foo "foo
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init --warn >/dev/null'
seed_skill bar "bar
"
run_shell_expect 0 '"$SEAL" verify'
contains "Unverified"
contains "Result: Warning"
pass "warn policy permits unverified"
EOF
      ;;

    mixed-removed-mismatch-block)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Start with two bundles, delete one, modify the other. Removed alone would be
Drift, but the mismatch should make the outcome Blocked.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_two_skills
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
rm -rf .claude/skills/bar
write_file ".claude/skills/foo/SKILL.md" "tampered
"
run_shell_expect 1 '"$SEAL" verify'
contains "Removed"
contains "Mismatch"
contains "Result: Blocked"
pass "mismatch dominates removed drift under block policy"
EOF
      ;;

    discovery-whole-dir-added-file-mismatch)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

`.cursor/rules` is detected as one whole-directory bundle. Add a file under it
after init; verify should report one mismatch for `./.cursor/rules`.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file ".cursor/rules/rule-a.mdc" "a
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
write_file ".cursor/rules/rule-b.mdc" "b
"
run_shell_expect 1 '"$SEAL" verify --verbose'
contains "Mismatch"
contains "./.cursor/rules"
contains "added:"
pass "whole directory discovery addition is mismatch"
EOF
      ;;

    discovery-hidden-child)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Hidden children under a wildcard discovery directory should be pinned and
verified like any other child.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file ".agents/skills/.hidden/SKILL.md" "hidden
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 0 '"$SEAL" verify --json'
contains './.agents/skills/.hidden'
pass "wildcard discovery includes hidden child"
EOF
      ;;

    discovery-empty-dir-ignored)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Create an empty discovered child directory and initialize. It should not be
materialized as a bundle.
EOF
      write_demo "$dir" <<'EOF'
reset_all
mkdir -p .claude/skills/empty
run_shell_expect 0 'printf "y\n" | "$SEAL" init'
contains "0 bundles"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "empty discovered directory ignored"
EOF
      ;;

    discovery-question-is-literal)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The discovery pattern `literal?dir/*` should match a directory literally named
`literal?dir`, not any single-character wildcard variant.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "literal?dir/item/SKILL.md" "literal
"
cat > seal.json <<'LOCK'
{
  "version": 1,
  "discovery": ["literal?dir/*"],
  "policy": "block",
  "bundles": {}
}
LOCK
run_shell_expect 0 'printf "y\n" | "$SEAL" pin >/dev/null'
run_shell_expect 0 '"$SEAL" verify --json'
contains './literal?dir/item'
pass "question mark is literal"
EOF
      ;;

    regular-file-root-happy)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Use discovery `prompts/*`, bulk pin one regular file, then verify. The bundle
entry should use `files: { ".": ... }`.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "prompts/a.md" "a
"
cat > seal.json <<'LOCK'
{
  "version": 1,
  "discovery": ["prompts/*"],
  "policy": "block",
  "bundles": {}
}
LOCK
run_shell_expect 0 'printf "y\n" | "$SEAL" pin >/dev/null'
contains "Pinned"
grep -Fq '"."' seal.json || fail "file-root map did not contain dot entry"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "regular file root verifies"
EOF
      ;;

    git-entry-excluded)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

`.git` entries under a bundle root are excluded. Changing a `.git` file after
pinning should not alter verification.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
write_file ".claude/skills/foo/.git" "gitdir: elsewhere
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
write_file ".claude/skills/foo/.git" "changed
"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass ".git entry excluded"
EOF
      ;;

    metadata-ignored)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Change only file mode metadata after init. Seal hashes bytes, not metadata, so
verify should remain clean.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
chmod +x .claude/skills/foo/SKILL.md 2>/dev/null || true
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "metadata-only change ignored"
EOF
      ;;

    invalid-json)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Run verify against malformed JSON. It should exit 2 before classification.
EOF
      write_demo "$dir" <<'EOF'
reset_all
printf '{ bad json\n' > seal.json
run_shell_expect 2 '"$SEAL" verify'
contains "decode lockfile"
pass "invalid JSON fatal"
EOF
      ;;

    invalid-unknown-field)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The lockfile contains an unknown top-level field. Verify should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
cat > seal.json <<'LOCK'
{
  "version": 1,
  "policy": "block",
  "extra": true,
  "bundles": {}
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "unknown field"
pass "unknown top-level field fatal"
EOF
      ;;

    invalid-unknown-bundle-field)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The bundle entry contains an unknown field. Verify should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
write_lockfile_single block ".claude/skills/*" "./.claude/skills/foo" "SKILL.md" ".claude/skills/foo/SKILL.md"
perl -0pi -e 's/"files"/"unexpected": true,\n      "files"/' seal.json
run_shell_expect 2 '"$SEAL" verify'
contains "unknown field"
pass "unknown bundle field fatal"
EOF
      ;;

    invalid-missing-policy)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The lockfile omits required `policy`. Verify should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
cat > seal.json <<'LOCK'
{
  "version": 1,
  "bundles": {}
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "policy is required"
pass "missing policy fatal"
EOF
      ;;

    invalid-bad-version)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The lockfile uses unsupported schema version 2. Verify should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
cat > seal.json <<'LOCK'
{
  "version": 2,
  "policy": "block",
  "bundles": {}
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "version 2 not supported"
pass "bad version fatal"
EOF
      ;;

    invalid-bad-policy)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The lockfile uses an unsupported policy. Verify should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
cat > seal.json <<'LOCK'
{
  "version": 1,
  "policy": "allow",
  "bundles": {}
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "policy must be"
pass "bad policy fatal"
EOF
      ;;

    invalid-bad-hash)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

A recorded hash is not in canonical `sha256:<64 lowercase hex>` form. Verify
should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
write_lockfile_single block ".claude/skills/*" "./.claude/skills/foo" "SKILL.md" ".claude/skills/foo/SKILL.md"
perl -0pi -e 's/sha256:[0-9a-f]{64}/sha256:BAD/' seal.json
run_shell_expect 2 '"$SEAL" verify'
contains "not in sha256"
pass "bad hash fatal"
EOF
      ;;

    invalid-contenthash)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The aggregate contentHash does not match the files map. Verify should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
write_lockfile_single block ".claude/skills/*" "./.claude/skills/foo" "SKILL.md" ".claude/skills/foo/SKILL.md"
perl -0pi -e 's/"contentHash": "sha256:[0-9a-f]{64}"/"contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000"/' seal.json
run_shell_expect 2 '"$SEAL" verify'
contains "contentHash mismatch"
pass "bad contentHash fatal"
EOF
      ;;

    invalid-empty-files)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The bundle has an empty `files` object. Verify should exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
cat > seal.json <<'LOCK'
{
  "version": 1,
  "discovery": [".claude/skills/*"],
  "policy": "block",
  "bundles": {
    "./.claude/skills/foo": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      "files": {}
    }
  }
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "files map is empty"
pass "empty files map fatal"
EOF
      ;;

    invalid-dot-bundle-key|invalid-traversal-bundle-key|invalid-backslash-file-path)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<EOF
## Manual Steps

This lab writes a structurally invalid bundle or file path. Verify should
exit 2 before any bundle classification.
EOF
      write_demo "$dir" <<EOF
reset_all
cat > seal.json <<'LOCK'
{
  "version": 1,
  "policy": "block",
  "bundles": {
EOF
      if [ "$lab" = "invalid-dot-bundle-key" ]; then
        cat >> "$dir/demo.sh" <<'EOF'
    ".": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      "files": {"SKILL.md": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
    }
EOF
      elif [ "$lab" = "invalid-traversal-bundle-key" ]; then
        cat >> "$dir/demo.sh" <<'EOF'
    "./skills/../foo": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      "files": {"SKILL.md": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
    }
EOF
      else
        cat >> "$dir/demo.sh" <<'EOF'
    "./skills/foo": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      "files": {"bad\\path": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
    }
EOF
      fi
      cat >> "$dir/demo.sh" <<'EOF'
  }
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "invalid"
pass "invalid path shape fatal"
EOF
      chmod +x "$dir/demo.sh"
      ;;

    invalid-discovery-*)
      local pattern
      case "$lab" in
        invalid-discovery-trailing-slash) pattern="skills/" ;;
        invalid-discovery-recursive-glob) pattern="skills/**" ;;
        invalid-discovery-leading-dot-slash) pattern="./skills/*" ;;
        invalid-discovery-leading-slash) pattern="/skills/*" ;;
        invalid-discovery-backslash) pattern='skills\*' ;;
        invalid-discovery-repeated-separator) pattern="skills//foo" ;;
        invalid-discovery-dot-segment) pattern="skills/./foo" ;;
      esac
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<EOF
## Manual Steps

The lockfile uses invalid discovery pattern \`$pattern\`. Verify should exit 2.
EOF
      write_demo "$dir" <<EOF
reset_all
cat > seal.json <<'LOCK'
{
  "version": 1,
  "discovery": ["$pattern"],
  "policy": "block",
  "bundles": {}
}
LOCK
run_shell_expect 2 '"\$SEAL" verify'
contains "discovery"
pass "invalid discovery pattern fatal"
EOF
      ;;

    invalid-required-field-shapes)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

This lab runs several required-field variants: missing top-level fields, null
fields, wrong field types, and missing bundle fields. Every variant should exit
2 before verification classification.
EOF
      write_demo "$dir" <<'EOF'
reset_all

cat > seal.json <<'LOCK'
{"policy":"block","bundles":{}}
LOCK
run_shell_expect 2 '"$SEAL" verify'

cat > seal.json <<'LOCK'
{"version":1,"policy":"block"}
LOCK
run_shell_expect 2 '"$SEAL" verify'

cat > seal.json <<'LOCK'
{"version":"1","policy":"block","bundles":{}}
LOCK
run_shell_expect 2 '"$SEAL" verify'

cat > seal.json <<'LOCK'
{"version":1,"policy":null,"bundles":{}}
LOCK
run_shell_expect 2 '"$SEAL" verify'

cat > seal.json <<'LOCK'
{
  "version": 1,
  "policy": "block",
  "bundles": {
    "./skills/foo": {
      "files": {"SKILL.md": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
    }
  }
}
LOCK
run_shell_expect 2 '"$SEAL" verify'

cat > seal.json <<'LOCK'
{
  "version": 1,
  "policy": "block",
  "bundles": {
    "./skills/foo": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    }
  }
}
LOCK
run_shell_expect 2 '"$SEAL" verify'

pass "required-field invalid shapes are fatal"
EOF
      ;;

    invalid-bundle-key-shapes)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

This lab loops through invalid bundle-key shapes: no `./` prefix, absolute
path, backslash, repeated separator, dot segment, dot-dot segment, and trailing
slash.
EOF
      write_demo "$dir" <<'EOF'
reset_all
for key in \
  "skills/foo" \
  "/skills/foo" \
  "./skills\\foo" \
  "./skills//foo" \
  "./skills/./foo" \
  "./skills/../foo" \
  "./skills/foo/"
do
  cat > seal.json <<LOCK
{
  "version": 1,
  "policy": "block",
  "bundles": {
    "$key": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      "files": {"SKILL.md": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
    }
  }
}
LOCK
  run_shell_expect 2 '"$SEAL" verify'
done
pass "bundle-key invalid shapes are fatal"
EOF
      ;;

    invalid-file-path-shapes)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

This lab loops through invalid `files` map paths: empty path, mixed dot path,
leading `./`, absolute path, repeated separator, dot segment, dot-dot segment,
and trailing slash.
EOF
      write_demo "$dir" <<'EOF'
reset_all
for file_path in \
  "" \
  "./SKILL.md" \
  "/SKILL.md" \
  "dir//file.md" \
  "dir/./file.md" \
  "dir/../file.md" \
  "dir/file.md/"
do
  cat > seal.json <<LOCK
{
  "version": 1,
  "policy": "block",
  "bundles": {
    "./skills/foo": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      "files": {"$file_path": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
    }
  }
}
LOCK
  run_shell_expect 2 '"$SEAL" verify'
done

cat > seal.json <<'LOCK'
{
  "version": 1,
  "policy": "block",
  "bundles": {
    "./skills/foo": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      "files": {
        ".": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
        "extra.md": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
      }
    }
  }
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
pass "file-path invalid shapes are fatal"
EOF
      ;;

    invalid-duplicate-json-keys)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

This lab writes duplicate JSON object keys. The decoder should reject them
instead of accepting the last value.
EOF
      write_demo "$dir" <<'EOF'
reset_all
cat > seal.json <<'LOCK'
{
  "version": 1,
  "version": 1,
  "policy": "block",
  "bundles": {}
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "duplicate object key"

cat > seal.json <<'LOCK'
{
  "version": 1,
  "policy": "block",
  "bundles": {
    "./skills/foo": {
      "contentHash": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      "files": {
        "SKILL.md": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
        "SKILL.md": "sha256:1111111111111111111111111111111111111111111111111111111111111111"
      }
    }
  }
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "duplicate object key"
pass "duplicate JSON keys are fatal"
EOF
      ;;

    true-case-key-mismatch)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

The lockfile records `./.claude/skills/Foo`, while the filesystem reports
`.claude/skills/foo`. Verify should treat the recorded key as Removed and the
true-case discovery result as Unverified.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill foo "foo
"
write_lockfile_single block ".claude/skills/*" "./.claude/skills/Foo" "SKILL.md" ".claude/skills/foo/SKILL.md"
run_shell_expect 1 '"$SEAL" verify'
contains "Removed"
contains "Unverified"
contains "Result: Blocked"
pass "wrong-case recorded key is removed plus unverified"
EOF
      ;;

    pin-bulk-noop)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize, then run `../bin/seal pin`. It should print `No changes` and not
prompt.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
before=$(cat seal.json)
run_shell_expect 0 '"$SEAL" pin'
contains "No changes"
after=$(cat seal.json)
[ "$before" = "$after" ] || fail "bulk no-op rewrote seal.json"
pass "bulk pin no-op"
EOF
      ;;

    pin-bulk-add-new)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize one bundle, add another discovered bundle, then run bulk pin and
accept. Verify should become clean again.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill foo "foo
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
seed_skill bar "bar
"
run_shell_expect 0 'printf "y\n" | "$SEAL" pin'
contains "Pinned"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "bulk pin adds new discovery match"
EOF
      ;;

    pin-bulk-update-modified)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize, modify a tracked file, then run bulk pin and accept. Verify should
be clean afterward.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
write_file ".claude/skills/foo/SKILL.md" "modified
"
run_shell_expect 0 'printf "y\n" | "$SEAL" pin'
contains "modified"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "bulk pin updates modified hashes"
EOF
      ;;

    pin-bulk-removed-no-prune)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize, delete the bundle directory, then run `../bin/seal pin` without
`--prune`. It should preserve the entry and verify should remain Drift.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
rm -rf .claude/skills/foo
run_shell_expect 0 '"$SEAL" pin'
contains "removed"
contains "No changes"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Drift"
pass "bulk pin preserves removed without prune"
EOF
      ;;

    pin-bulk-removed-prune)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize, delete the bundle directory, then run `../bin/seal pin --prune`
and accept. Verify should become clean with zero bundles.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
rm -rf .claude/skills/foo
run_shell_expect 0 'printf "y\n" | "$SEAL" pin --prune'
contains "removed"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "bulk pin --prune removes missing entry"
EOF
      ;;

    pin-targeted-dir)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Create an empty lockfile with `seal init`, then targeted-pin `custom/skill`.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "custom/skill/SKILL.md" "custom
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 0 'printf "y\n" | "$SEAL" pin custom/skill'
contains "Pinned"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "targeted pin adds directory root"
EOF
      ;;

    pin-targeted-file-root)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Create an empty lockfile, then targeted-pin `prompts/a.md` as a regular-file
sealed root.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "prompts/a.md" "a
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 0 'printf "y\n" | "$SEAL" pin prompts/a.md'
contains "Pinned"
grep -Fq '"."' seal.json || fail "file-root map did not contain dot entry"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "targeted pin adds regular file root"
EOF
      ;;

    pin-targeted-modified)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize two bundles, modify one, then targeted-pin only that bundle.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_two_skills
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
write_file ".claude/skills/foo/SKILL.md" "foo-modified
"
run_shell_expect 0 'printf "y\n" | "$SEAL" pin .claude/skills/foo'
contains "Pinned"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "targeted pin updates named bundle"
EOF
      ;;

    pin-targeted-noop)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize, then targeted-pin the unchanged bundle. It should print
`No changes`.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
before=$(cat seal.json)
run_shell_expect 0 '"$SEAL" pin .claude/skills/foo'
contains "No changes"
after=$(cat seal.json)
[ "$before" = "$after" ] || fail "targeted no-op rewrote seal.json"
pass "targeted pin no-op"
EOF
      ;;

    pin-targeted-outside-project)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

After init, try to pin a path outside the project. It should fail.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
tmp=$(mktemp -d)
write_file "$tmp/outside.md" "outside
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 2 '"$SEAL" pin "$tmp/outside.md"'
contains "outside the project root"
rm -rf "$tmp"
pass "targeted pin rejects outside path"
EOF
      ;;

    pin-targeted-project-root-rejected)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

After init, try `../bin/seal pin .`. Seal v1 does not support the project root
as a sealed root.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 2 '"$SEAL" pin .'
contains "project root"
pass "targeted pin rejects project root"
EOF
      ;;

    pin-targeted-symlink-rejected)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Try to targeted-pin a symlink. On platforms that can create symlinks, pin
should reject it with exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "target/SKILL.md" "target
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
if ln -s target linked 2>/dev/null; then
  run_shell_expect 2 '"$SEAL" pin linked'
  contains "symlink"
  pass "targeted pin rejects symlink"
else
  pass "symlink unavailable; lab skipped on this platform"
fi
EOF
      ;;

    pin-decline-aborts)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize, modify a tracked file, run `seal pin`, and answer `n`. The command
should exit 2, preserve the old lockfile, and verify should still block.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
before=$(cat seal.json)
write_file ".claude/skills/foo/SKILL.md" "modified
"
run_shell_expect 2 'printf "n\n" | "$SEAL" pin'
contains "pin aborted"
after=$(cat seal.json)
[ "$before" = "$after" ] || fail "seal.json changed after declined pin"
run_shell_expect 1 '"$SEAL" verify'
contains "Result: Blocked"
pass "pin decline preserves lockfile"
EOF
      ;;

    pin-invalid-lockfile-fatal)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

`seal pin` should validate the existing lockfile before doing any write. This
lab uses an unknown field and expects exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
cat > seal.json <<'LOCK'
{
  "version": 1,
  "policy": "block",
  "extra": true,
  "bundles": {}
}
LOCK
run_shell_expect 2 '"$SEAL" pin'
contains "unknown field"
pass "pin refuses invalid existing lockfile"
EOF
      ;;

    pin-bulk-unsupported-symlink-aborts)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

`seal pin` should abort, not write, if hashing encounters an unsupported
non-regular file such as a symlink. On platforms without symlink support, the
demo reports a skip.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
before=$(cat seal.json)
if ln -s SKILL.md .claude/skills/foo/link.md 2>/dev/null; then
  run_shell_expect 2 'printf "y\n" | "$SEAL" pin'
  after=$(cat seal.json)
  [ "$before" = "$after" ] || fail "seal.json changed after unsupported symlink"
  pass "bulk pin aborts on unsupported symlink"
else
  pass "symlink unavailable; lab skipped on this platform"
fi
EOF
      ;;

    subdir-no-lockfile-traversal)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Initialize at the lab root, then `cd` into a subdirectory and run verify.
The CLI must not walk upward to find the root lockfile.
EOF
      write_demo "$dir" <<'EOF'
reset_all
seed_skill
mkdir -p nested
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 2 '(cd nested && "$SEAL" verify)'
contains "seal.json not found"
pass "verify does not traverse parent directories"
EOF
      ;;

    invalid-static-coverage)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

A non-empty `bundles` map without discovery coverage is an invalid lockfile.
Verify should fail before bundle classification with exit 2 and an actionable
static-coverage diagnostic.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "custom/skill/SKILL.md" "custom
"
write_lockfile_single block "" "./custom/skill" "SKILL.md" "custom/skill/SKILL.md"
run_shell_expect 2 '"$SEAL" verify'
contains "not statically produced"
contains "./custom/skill"
not_contains "Result:"
pass "uncovered bundle key is fatal"
EOF
      ;;

    invalid-static-coverage-wrong-pattern)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

This is the concrete bypass shape from the conformance issue: a file-root
bundle is recorded under `docs/`, but the existing discovery patterns do not
statically produce that key. Verify should reject the lockfile before reporting
the recorded file as verified or ignoring later siblings.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "docs/contributor-prompt.md" "prompt
"
write_file "docs/contributor-prompt-v2.md" "sibling
"
write_file "skills/foo/SKILL.md" "skill
"
write_lockfile_single block "skills/*" "./docs/contributor-prompt.md" "." "docs/contributor-prompt.md"
run_shell_expect 2 '"$SEAL" verify'
contains "not statically produced"
contains '"skills/*"'
contains "./docs/contributor-prompt.md"
not_contains "Result:"
pass "uncovered file-root key is fatal even when other discovery exists"
EOF
      ;;

    invalid-static-coverage-removed-key)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

An uncovered recorded key is invalid even if the sealed root is gone from disk.
Verify should exit 2 before it can classify that key as `Removed` or produce a
`Drift` result.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file ".dirB/two/SKILL.md" "two
"
write_lockfile_single block ".dirA/*" "./.dirB/two" "SKILL.md" ".dirB/two/SKILL.md"
rm -rf .dirB
run_shell_expect 2 '"$SEAL" verify'
contains "not statically produced"
contains "./.dirB/two"
not_contains "Removed"
not_contains "Result:"
pass "uncovered removed key exits before classification"
EOF
      ;;

    invalid-narrowed-discovery)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

If discovery is narrowed while old bundle keys remain, the lockfile is invalid
because recorded keys are no longer statically produced by discovery. Verify
should exit 2 instead of silently shrinking the insertion fence.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file ".dirA/one/SKILL.md" "one
"
write_file ".dirB/two/SKILL.md" "two
"
write_file ".dirB/three/SKILL.md" "three
"
a_hash=$(sha256_file ".dirA/one/SKILL.md")
b_hash=$(sha256_file ".dirB/two/SKILL.md")
a_content=$(printf '%s' "SKILL.md:sha256:$a_hash" | sha256_text)
b_content=$(printf '%s' "SKILL.md:sha256:$b_hash" | sha256_text)
cat > seal.json <<LOCK
{
  "version": 1,
  "discovery": [".dirA/*"],
  "policy": "block",
  "bundles": {
    "./.dirA/one": {
      "contentHash": "sha256:$a_content",
      "files": {"SKILL.md": "sha256:$a_hash"}
    },
    "./.dirB/two": {
      "contentHash": "sha256:$b_content",
      "files": {"SKILL.md": "sha256:$b_hash"}
    }
  }
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "not statically produced"
contains "./.dirB/two"
pass "narrowed discovery is fatal when old bundle keys remain"
EOF
      ;;

    removed-covered-bundle-drift)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

This is the control for static coverage: a missing recorded key should still
classify as `Removed` when discovery still statically covers it.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file ".dirB/two/SKILL.md" "two
"
write_lockfile_single block ".dirB/*" "./.dirB/two" "SKILL.md" ".dirB/two/SKILL.md"
rm -rf .dirB
run_shell_expect 0 '"$SEAL" verify'
contains "Removed"
contains "Result: Drift"
pass "covered removed key remains drift"
EOF
      ;;

    discovery-file-sibling-unverified)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

With a broad file-root discovery pattern such as `docs/*.md`, adding a sibling
file should be detected as a new `Unverified` bundle and block under the
default policy.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "docs/contributor-prompt.md" "prompt
"
write_lockfile_single block "docs/*.md" "./docs/contributor-prompt.md" "." "docs/contributor-prompt.md"
write_file "docs/contributor-prompt-v2.md" "sibling
"
run_shell_expect 1 '"$SEAL" verify'
contains "Unverified"
contains "./docs/contributor-prompt-v2.md"
contains "Result: Blocked"
pass "broad file-root discovery detects sibling insertion"
EOF
      ;;

    invalid-nested-bundle-keys)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Bundle keys may not be directory-segment ancestors of other bundle keys.
Verify should fail before bundle classification with exit 2.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "skills/SKILL.md" "parent
"
write_file "skills/foo/SKILL.md" "child
"
parent_hash=$(sha256_file "skills/SKILL.md")
child_hash=$(sha256_file "skills/foo/SKILL.md")
parent_content=$(printf '%s' "SKILL.md:sha256:$parent_hash
foo/SKILL.md:sha256:$child_hash" | sha256_text)
child_content=$(printf '%s' "SKILL.md:sha256:$child_hash" | sha256_text)
cat > seal.json <<LOCK
{
  "version": 1,
  "discovery": ["skills"],
  "policy": "block",
  "bundles": {
    "./skills": {
      "contentHash": "sha256:$parent_content",
      "files": {
        "SKILL.md": "sha256:$parent_hash",
        "foo/SKILL.md": "sha256:$child_hash"
      }
    },
    "./skills/foo": {
      "contentHash": "sha256:$child_content",
      "files": {
        "SKILL.md": "sha256:$child_hash"
      }
    }
  }
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "nested sealed roots"
contains "./skills"
contains "./skills/foo"
pass "nested bundle keys are fatal"
EOF
      ;;

    invalid-overlapping-discovery)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Discovery patterns `skills` and `skills/*` are invalid together because they
can produce nested sealed roots. Verify should fail before classifying any
discovered bundle.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "skills/foo/SKILL.md" "child
"
cat > seal.json <<'LOCK'
{
  "version": 1,
  "discovery": ["skills", "skills/*"],
  "policy": "block",
  "bundles": {}
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "discovery patterns"
contains "nested sealed roots"
pass "overlapping discovery roots are fatal"
EOF
      ;;

    invalid-overlapping-discovery-wildcard-literal)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Discovery patterns `skills/*` and `skills/foo/*` are invalid together:
`skills/*` can produce `./skills/foo`, while `skills/foo/*` can produce nested
sealed roots below it. Verify should reject this before classification.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "skills/foo/bar/SKILL.md" "nested
"
cat > seal.json <<'LOCK'
{
  "version": 1,
  "discovery": ["skills/*", "skills/foo/*"],
  "policy": "block",
  "bundles": {}
}
LOCK
run_shell_expect 2 '"$SEAL" verify'
contains "discovery patterns"
contains "nested sealed roots"
pass "wildcard parent plus literal descendant discovery is fatal"
EOF
      ;;

    pin-targeted-adds-discovery)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

When targeted pin adds a new key not covered by discovery, it should add the
narrowest exact discovery pattern in the same update so the new recorded
file-root key is valid on the next verify.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "docs/contributor-prompt.md" "prompt
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 0 'printf "y\n" | "$SEAL" pin docs/contributor-prompt.md >/dev/null'
grep -q '"docs/contributor-prompt.md"' seal.json || fail "targeted pin did not add exact discovery pattern"
grep -q '"./docs/contributor-prompt.md"' seal.json || fail "bundle key was not added"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "targeted pin adds exact discovery coverage for file root"
EOF
      ;;

    pin-targeted-nested-rejected)
      write_readme "$dir" "$lab" "$title" "$covers" "$expected" <<'EOF'
## Manual Steps

Pin a directory root, then try to pin a target inside that recorded sealed
root. Targeted pin should reject the nested boundary with exit 2 and leave the
existing lockfile valid.
EOF
      write_demo "$dir" <<'EOF'
reset_all
write_file "skills/foo/SKILL.md" "root
"
write_file "skills/foo/bar/SKILL.md" "nested
"
run_shell_expect 0 'printf "y\n" | "$SEAL" init >/dev/null'
run_shell_expect 0 'printf "y\n" | "$SEAL" pin skills/foo >/dev/null'
before=$(cat seal.json)
run_shell_expect 2 '"$SEAL" pin skills/foo/bar'
contains "inside existing sealed root"
contains "./skills/foo"
after=$(cat seal.json)
[ "$before" = "$after" ] || fail "seal.json changed after rejected nested pin"
run_shell_expect 0 '"$SEAL" verify'
contains "Result: Verified"
pass "targeted pin rejects nested sealed root"
EOF
      ;;

    *)
      fail "no creator for lab $lab"
      ;;
  esac
}

build_binary() {
  mkdir -p "$ROOT/bin"
  printf 'Compiling seal into %s/bin/seal...\n' "$ROOT"
  go build -o "$ROOT/bin/seal" ./cmd/seal || exit 1
}

choose_labs_interactively() {
  printf 'Available labs:\n'
  list_labs
  printf '\nEnter comma-separated lab names, or press Enter for all: '
  local answer
  IFS= read -r answer || answer=""
  if [ -z "$answer" ]; then
    SELECTED_LABS=("${LABS[@]}")
    return
  fi
  local old_ifs=$IFS
  IFS=,
  # shellcheck disable=SC2206
  SELECTED_LABS=($answer)
  IFS=$old_ifs
}

ROOT=""
SELECTED_LABS=()
GENERATE_ALL=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --root)
      [ "$#" -ge 2 ] || { printf 'missing value for --root\n' >&2; exit 2; }
      ROOT=$2
      shift 2
      ;;
    --lab)
      [ "$#" -ge 2 ] || { printf 'missing value for --lab\n' >&2; exit 2; }
      SELECTED_LABS+=("$2")
      shift 2
      ;;
    --all)
      GENERATE_ALL=1
      shift
      ;;
    --list)
      list_labs
      exit 0
      ;;
    --matrix)
      print_matrix
      exit 0
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [ -z "$ROOT" ]; then
  ROOT="${TMPDIR:-/tmp}/seal-labs-$(date +%Y%m%d-%H%M%S)"
fi

if [ "$GENERATE_ALL" -eq 1 ]; then
  SELECTED_LABS=("${LABS[@]}")
elif [ "${#SELECTED_LABS[@]}" -eq 0 ]; then
  if [ -t 0 ]; then
    choose_labs_interactively
  else
    SELECTED_LABS=("${LABS[@]}")
  fi
fi

for lab in "${SELECTED_LABS[@]}"; do
  if ! has_lab "$lab"; then
    printf 'unknown lab: %s\n\n' "$lab" >&2
    list_labs >&2
    exit 2
  fi
done

mkdir -p "$ROOT"
build_binary

for lab in "${SELECTED_LABS[@]}"; do
  printf 'Creating lab: %s\n' "$lab"
  create_lab "$lab"
done

cat > "$ROOT/README.md" <<EOF
# Seal CLI Labs

Generated by \`scripts/seal-labs.sh\`.

Binary:

\`\`\`sh
$ROOT/bin/seal
\`\`\`

List generated labs:

\`\`\`sh
find "$ROOT" -mindepth 1 -maxdepth 1 -type d | sort
\`\`\`

Each lab contains a README with manual steps and a runnable \`demo.sh\`.
EOF

printf '\nLabs created at: %s\n' "$ROOT"
printf 'Open %s/README.md or run a lab demo, for example:\n' "$ROOT"
printf '  cd %s/%s && ./demo.sh\n' "$ROOT" "${SELECTED_LABS[0]}"
