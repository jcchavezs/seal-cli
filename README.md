# seal

> Pin and verify the AI-agent bundles in your repo. Catch tampered or injected skills, prompts, and sub-agents before they ship.

`seal` records the trusted state of your project-local agent assets - Claude Code skills, Codex skills, sub-agent definitions, hooks, prompts, rules - into a single lockfile (`seal.json`). Run `seal verify` in CI to refuse builds when those assets drift from what was pinned.

Think `package-lock.json`, but for the context your agents read.

---

## Why this exists

Modern repos ship code *and* the agent context that runs on top of it: skill definitions, sub-agent system prompts, slash commands, hooks. Any of those can change how an agent behaves at runtime. Two failure modes seal defends against:

- **Tampering** - an existing skill or prompt is edited (accidentally or maliciously).
- **Insertion** - a new file lands inside an agent layout (`.claude/skills/`, `.codex/skills/`, `.github/skills/`, etc.) and gets loaded automatically.

Under the default `block` policy, `seal verify` fails CI (exit 1) for both cases. The lockfile is the allowlist; anything outside it trips the gate.

---

## Install

```sh
go install github.com/SamyGhannad/seal-cli/cmd/seal@latest
```

Or grab a prebuilt binary from the [releases page](https://github.com/SamyGhannad/seal-cli/releases) and drop it on your `PATH`.

```sh
seal --version
```

To upgrade later:

```sh
seal upgrade --check
seal upgrade
```

`seal upgrade` uses `go install ...@latest` for Go-installed binaries. For prebuilt release binaries, it downloads the matching GitHub Release asset, verifies the `.sha256` sidecar, and replaces the local executable where the OS permits it.

---

## Quickstart

```sh
# 1. Bootstrap the lockfile (auto-detects known agent layouts)
seal init

# 2. Commit seal.json alongside the assets it pins
git add seal.json && git commit -m "chore: seal agent bundles"

# 3. Verify in CI - exits non-zero on tampering or insertion
seal verify

# 4. After an intentional change, re-pin
seal pin && git commit -am "chore: re-pin bundles"
```

That's the whole loop.

---

## What it auto-detects

`seal init` scans for these layouts and proposes them as discovery patterns. Anything not detected can be added manually (see [Adding paths manually](#adding-paths-manually)).

**Claude Code**

| Asset | Pattern |
|---|---|
| Skills | `.claude/skills/*` |
| Agents | `.claude/agents/*` |
| Commands | `.claude/commands/*` |
| Plugins | `.claude/plugins/*` |
| Hooks | `.claude/hooks` |

**OpenAI Codex**

| Asset | Pattern |
|---|---|
| Skills | `.codex/skills/*` |

**Cursor**

| Asset | Pattern |
|---|---|
| Skills | `.cursor/skills/*` |
| Rules | `.cursor/rules` |

**Codeium Windsurf**

| Asset | Pattern |
|---|---|
| Skills | `.windsurf/skills/*` |
| Rules | `.windsurf/rules` |

**Gemini CLI**

| Asset | Pattern |
|---|---|
| Skills | `.gemini/skills/*` |

**GitHub Copilot**

| Asset | Pattern |
|---|---|
| Skills | `.github/skills/*` |
| Prompts | `.github/prompts` |
| Instructions | `.github/instructions` |

**OpenCode**

| Asset | Pattern |
|---|---|
| Skills | `.opencode/skills/*` |
| Agents | `.opencode/agents/*` |
| Commands | `.opencode/commands/*` |
| Plugins | `.opencode/plugins/*` |
| Modes | `.opencode/modes/*` |
| Tools | `.opencode/tools/*` |

**AWS Kiro**

| Asset | Pattern |
|---|---|
| Skills | `.kiro/skills/*` |
| Agents | `.kiro/agents/*` |
| Specs | `.kiro/specs/*` |
| Steering | `.kiro/steering` |

**Amazon Q Developer**

| Asset | Pattern |
|---|---|
| Agents | `.amazonq/agents/*` |
| Rules | `.amazonq/rules` |

**Sourcegraph Amp**

| Asset | Pattern |
|---|---|
| Skills | `.amp/skills/*` |
| Agents | `.amp/agents/*` |

**OpenClaw**

| Asset | Pattern |
|---|---|
| Skills | `.openclaw/skills/*` |

**Cross-tool (Codex, OpenCode, Gemini, Copilot all honour this)**

| Asset | Pattern |
|---|---|
| Agent Skills | `.agents/skills/*` |

A pattern is only proposed if the relevant directory actually exists in your project - a Go-only repo doesn't get every Python-tool pattern dumped into it.

---

## Discovery patterns

The `discovery` array in `seal.json` controls what `seal verify` looks at on disk. Two shapes are supported:

**`dir/*`** - each direct child of `dir` becomes its own bundle.

- A subdirectory child is sealed recursively as one bundle.
- A regular-file child is sealed as a single-file bundle.
- New children appearing later trigger `Unverified` at verify time → `Blocked` under default policy.
- Per-item diff locality: when one child changes, only that child's bundle reports Mismatch.

**`dir`** - `dir` itself is a single bundle covering everything in its subtree.

- Recursive: every file at any depth inside `dir` contributes to the bundle's hash.
- Captures flat files at the top of `dir` AND nested files at any depth.
- Coarser diffs: any change anywhere in the tree shows the single bundle as Mismatch.

**Pick `dir/*` when** items inside the directory are independent (each skill is its own thing) and you want per-item change reporting.

**Pick `dir` when** the directory is a "single artifact" - a set of rules, a flat scripts dir, anything where the natural unit is the whole tree.

Trailing slashes are not allowed: `.claude/agents` is valid, `.claude/agents/` is rejected.

**Static coverage.** Every recorded bundle key must be statically produced by at least one `discovery` pattern. `seal verify` rejects the lockfile (exit `2`) before classification when that invariant fails. Narrowing `discovery` while leaving old bundle keys in place is invalid, even if those directories are gone from disk.

**Targeted pin and insertion.** When `seal pin <path>` adds a bundle key that discovery does not yet cover, it also appends the narrowest exact discovery pattern for that key (for example `docs/contributor-prompt.md`). That makes the lockfile valid and pins the named path, but it does **not** fence sibling files in the same directory. To detect new siblings (for example `docs/contributor-prompt-v2.md`), use a per-child pattern such as `docs/*.md`, pin the containing directory with a whole-tree pattern (`docs`), or add the broader pattern by hand before relying on `seal verify` for insertion detection.

---

## Adding paths manually

`seal init` can't possibly know every project's layout. If you have agent context outside the auto-detected locations, edit `seal.json` and add patterns by hand:

```jsonc
{
  "version": 1,
  "policy": "block",
  "discovery": [
    ".claude/skills/*",
    "src/prompts",                 // whole-dir bundle
    "config/agent-rules/*",        // per-child bundles
    ".github/copilot-instructions.md"  // single file
  ],
  "bundles": { /* populated by seal pin */ }
}
```

Then run `seal pin` to incorporate the matched bundles:

```sh
seal pin
```

Pattern rules:

- Forward slashes only (no `\`)
- No leading `./` or `/`
- No trailing slash
- Single `*` matches one path segment; `**` is not supported in v1
- `?`, `[`, `]`, `{`, `}` are literal filename characters, not glob operators
- A pattern that matches nothing on disk simply produces no bundles - no error

You can also target individual paths directly:

```sh
seal pin docs/agent-readme.md      # pin one specific file
seal pin src/agents/my-agent       # pin one specific directory
```

---

## Commands

### `seal init`

Bootstrap a fresh `seal.json` in the current directory.

```sh
seal init           # default policy: block
seal init --warn    # policy: warn - drift reported, not blocking
seal init -v        # show per-file detail in the summary
```

Refuses to overwrite an existing lockfile.

### `seal pin [path...]`

Re-pin bundles into the lockfile.

```sh
seal pin                              # bulk re-pin every discovery match
seal pin --prune                      # also drop entries no longer on disk
seal pin .claude/skills/my-skill      # targeted: re-pin one bundle
seal pin docs/agent-readme.md         # targeted: pin a single file
```

Targeted mode is byte-faithful: `seal pin Foo` refuses if the on-disk entry is actually `foo`.

### `seal verify`

Compare on-disk state to the lockfile. Used in CI.

```sh
seal verify           # human output to stderr; exit code is the verdict
seal verify --json    # machine-readable output to stdout
seal verify --quiet   # exit code only, no output
```

---

## Outcomes & exit codes

`seal verify` collapses every per-bundle status (`Verified` / `Mismatch` / `Removed` / `Unverified`) into one of four overall outcomes:

| Outcome    | When                                                                                   | Exit |
|------------|----------------------------------------------------------------------------------------|------|
| `Verified` | Every bundle matches the lockfile                                                      | 0    |
| `Drift`    | Some bundles in the lockfile are gone from disk (only `Removed`, nothing else)         | 0    |
| `Warning`  | Any `Mismatch` or `Unverified`, AND `policy` is `warn`                                 | 0    |
| `Blocked`  | Any `Mismatch` or `Unverified`, AND `policy` is `block`                                | 1    |

Fatal errors (missing/invalid lockfile, I/O failure) always exit `2`.

**Key asymmetry:** a *new* file appearing on disk (`Unverified`) is treated as more risky than a tracked file *disappearing* (`Removed`). Deletion can't smuggle code in; insertion can. Only `Removed`-alone gets the soft `Drift` outcome.

---

## Policy

`seal.json` carries a single `policy` field:

- **`block`** *(default, recommended)* - `seal verify` exits `1` on any `Mismatch` or `Unverified`. This is the policy that actually defends against insertion attacks. Use in CI.
- **`warn`** - drift is reported on stderr but `seal verify` exits `0`. Useful during early development when bundles are churning. **Do not ship `warn` to a production CI lane** - it silently lets new files through.

Switch policy by hand-editing `seal.json` or re-running `seal init --warn`.

---

## CI example

GitHub Actions:

```yaml
- name: Install seal
  run: go install github.com/SamyGhannad/seal-cli/cmd/seal@latest

- name: Verify agent bundles
  run: seal verify
```

Exit code 1 (or 2) fails the job. Exit code 0 means the agent context shipping with this commit is exactly what was pinned, AND nothing new has appeared in the discovery paths.

---

## How it works

- **Per-file hashes.** Every bundle file is hashed (SHA-256), recorded under its NFC-normalised forward-slash path.
- **Aggregate content hash.** Per-bundle hash over the sorted `<path>:<hash>` entries - one value to verify the whole bundle.
- **File-roots.** A discovery match that points at a regular file (not a directory) becomes a single-file bundle whose `files` map is `{ ".": "sha256:..." }`. This catches loose agent assets like `.github/copilot-instructions.md`.
- **Deterministic encoding.** `seal.json` is byte-identical across machines: sorted keys, fixed indent, LF newlines, NFC text.
- **Atomic writes.** Lockfile updates use OS-level file locking and `tmp+rename` so concurrent runs can't corrupt the file.
- **No symlinks.** Symlinks and other non-regular files are unsupported in v1; encountering one during `pin` aborts, during `verify` produces `Mismatch`.

---

## License

MIT.
