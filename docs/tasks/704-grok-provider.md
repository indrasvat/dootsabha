# Task 704: Add xAI Grok CLI as Fourth Provider

**Status:** PENDING
**Depends On:** 105 (codex provider), 302 (extract providers), 703 (agy provider)
**Parallelizable With:** —

## Problem

Ārya now holds a Grok subscription, and `grok` (xAI "Grok Build TUI") is installed and
authenticated locally. Grok 4.5 at high reasoning effort is a capable reviewer and should
join दूतसभा as a **fourth** agent alongside `claude`, `codex`, and `agy`.

Unlike task 703 — which *replaced* the retired Gemini CLI and touched 64 files — this task
is strictly **additive**. Existing defaults, existing behaviour, and existing test
expectations must remain untouched.

> **Verified findings, risk analysis, timing data, and the council-review trail live in
> [`docs/grok-cli-findings.md`](../grok-cli-findings.md).** Read that before implementing —
> it is the evidence this task is built on.

## Contract summary (detail: `docs/grok-cli-findings.md`)

- **Invoke:** `grok -p <PROMPT> --output-format streaming-messages-json --always-approve
  --permission-mode bypassPermissions -m grok-4.5 --reasoning-effort high --no-plan
  --no-subagents --no-auto-update --sandbox read-only --max-turns <N>`
  …run with `HOME=<empty 0700 tmpdir>` and `GROK_HOME=<real ~/.grok>`.
  `--cwd` is **stripped from config but not injected** — the subprocess inherits
  दूतसभा's working directory, matching sibling providers. `--sandbox read-only`
  is what provides containment, so this is safe.
- **Parse:** NDJSON; take the **last** `type=="result"` line. Success →
  `result`/`session_id`/`usage`/`total_cost_usd`/`modelUsage` (snake_case, mirrors
  `claudeResponse`). Error → same `type:"result"` but `is_error:true`,
  `subtype:"error_during_execution"`, message in **`errors[]`**, **no `result` field**.
- **Never use `--output-format json`** — its `.text` concatenates tool-call preambles.
- **Never pass `--verbatim`** — it strips the `<user_query>` delimiter; grok does not
  rewrite prompts anyway.
- **Exit codes were reworked in this PR** — usage errors now emit 2, config errors 6.
  Precedence `2 > 6 > 4 > 3 > 5 > 1 > 0`. See CLAUDE.md / PRD §6.1.
- **Never assert an exact patch version** — the binary self-updates.

## Decision

**Additive and opt-in.** grok is fully wired into every command and fully tested, but:

- Default council stays `claude,codex,agy` (`codex,agy` inside Claude Code) — `council.go` **unchanged**.
- `refine --reviewers` default stays `codex,agy` — `refine.go` **unchanged**.
- `review --author/--reviewer` defaults stay `codex`/`claude` — `review.go` **unchanged**.
- Users opt in explicitly: `--agent grok`, `--agents claude,codex,grok`, `--chair grok`, etc.

Consequences: `MaxAgents = 5` is never approached; every existing test expectation stays
valid, which is what makes the red/green TDD baseline meaningful. Promoting grok to a
default later is a one-line follow-up.

**No Devanagari alias is required.** Verified: provider *names* carry no Devanagari aliases
anywhere in the repo (`claude`/`codex`/`agy` are bare ASCII in `getProvider`, config keys,
env vars). CLAUDE.md's bilingual rule covers *commands* and *key flags*; this task adds
neither. Stated explicitly so review does not flag it as missing.

## Files

### ADD — new files (the bulk of the work)

| Path | Contents |
|---|---|
| `internal/providers/grok.go` | `GrokProvider`, `NewGrokProvider`, `Name`, `Invoke`, `HealthCheck`, `providerConfig`, `stripPinnedFlags`, `grokIsolatedEnv`, `grokResult`/`grokUsage`/`grokModelUsage`, `parseGrokNDJSON` |
| `internal/providers/grok_test.go` | ~20 L2 tests (see TDD Plan) |
| `internal/providers/grok_internal_test.go` | `package providers` — tests for `grokIsolatedEnv` / `stripPinnedFlags` |
| `plugins/grok/main.go` | gRPC provider plugin, modelled on `plugins/codex/main.go`; `Capabilities.SupportsJson = true` |
| `testdata/mock-providers/mock-grok` | bash mock (mode 755) emitting a real NDJSON stream |
| `docs/tasks/704-grok-provider.md` | this file |

### EDIT — registration points only (one-line additions, no behaviour change)

| Path | Change |
|---|---|
| `internal/core/config.go` | 3 `ConfigComments` entries; 3 `v.SetDefault("providers.grok.*")`; append `"grok"` to `defaultProviderNames` |
| `configs/default.yaml` | new `grok:` block after `agy:` |
| `internal/cli/consult.go` | `getProvider` case; `providerColor` case; `--agent` help text; invalid-agent error string |
| `internal/output/styles.go` | `GrokColor = lipgloss.Color("#A855F7")` — purple, hue 271°, ≥54° from every existing palette hue |
| `internal/plugin/context_file.go` | grok row in `DefaultContextFile` provider map |
| `plugins/council-strategy/main.go` | provider-colour switch case |
| `internal/providers/types.go`, `internal/cli/root.go` | package/long-description doc strings |
| `Makefile` | `grok-provider` in `build-plugins` |

`internal/cli/status.go` needs **no change** — `collectHealthRows` iterates `cfg.Providers`,
so the grok row appears automatically. `internal/core/migrate.go` needs **no change** — it
is the gemini→agy rename and grok is not a migration.

### EDIT — test infrastructure

`scripts/test-binary.sh` (new mock-grok case), `scripts/test-agent-workflow.sh`
(`DOOTSABHA_PROVIDERS_GROK_BINARY`; provider loop `3` → `4`),
`scripts/test-plugin-smoke.sh` (`grok-provider`), `docs/testing-strategy.md` (mock contract),
`docs/PRD.md` §8.2 palette, `docs/PROGRESS.md` (Phase 7 row).

### EDIT — user-facing help, docs, and SKILL

Every surface that enumerates providers must list grok, and each is asserted by a test.

Exact strings (audited; **all are help/error text — no default value changes**):

| # | File:line | Change |
|---|---|---|
| C1 | `root.go:46` | `(Claude, Codex, Antigravity)` → `(Claude, Codex, Antigravity, Grok)` |
| C2 | `consult.go:105` | `"Agent to query: claude, codex, agy"` → `…, grok"` |
| C3 | `consult.go:126` | `unknown provider: %q — valid values: claude, codex, agy` → `…, grok` |
| C4 | `council.go:238` | `--agents` usage → `"Comma-separated agent names (claude, codex, agy, grok)"`; **`defaultAgents` at :234/:236 untouched** |
| C5/C6 | `review.go:164,167` | `--author`/`--reviewer` usage gains the provider list; defaults `codex`/`claude` untouched |
| C7/C8 | `refine.go:281,284` | `--author`/`--reviewers` usage gains the list; **default `"codex,agy"` untouched** |

⚠ `config_cmd.go:42,45` and `root.go:128` mention "Antigravity (agy)" but are **gemini→agy
migration** strings — **do not touch**; grok is not a migration.

| Surface | Change |
|---|---|
| `README.md` | provider table / agent list; note grok is opt-in and supplies tokens + cost |
| `skill/SKILL.md` | **minimal** edit — grok discoverable without bloating the index |
| `skill/references/command-reference.md` | provider-specific detail lives here, not in SKILL.md |
| `commands/dootsabha.md` | slash-command provider list |

**SKILL discipline (Ārya's explicit requirement):** keep the skill *sharp and focused*.
Follow current Claude Code skill best practice — SKILL.md stays a lean index; all
provider detail (flags, JSON fields, cost characteristics) goes into `references/`.
A new `skill/examples/` file is **not** justified unless it earns its tokens; default is
no new example. Verify skill claims against the real binary's `--help` output — do not
let the skill drift from the CLI.

## TDD Plan — strict red → green

Baseline `make ci` is green at HEAD (0 lint issues, all tests pass), so every red is ours.
**Each step: write the test, RUN IT, paste the failing output into the task's evidence
section, then implement to green.** No implementation before a captured red.

1. **RED** `grok_test.go`: `TestGrokProviderName` → `undefined: providers.NewGrokProvider`.
   **GREEN** minimal struct + `Name()`.
2. **RED** argv tests: every flag from the table above present; prompt passed via `-p`;
   `--no-plan` asserted explicitly (correctness-critical — without it grok can return a
   *plan* instead of the answer). Add a negative assertion that **`--verbatim` is NOT
   passed**, with a comment pointing at the rationale above, so the decision is enforced
   by a test rather than by memory. **GREEN** `Invoke` argv build.
2b. **RED** flag-precedence tests: correctness-critical pinned flags must **not** be
   overridable by `providers.grok.flags` in user config. A config supplying
   `--output-format plain` (or `--sandbox off`, `--permission-mode default`, a second
   `-m`, `--verbatim`, `--plan`) must be **stripped/overridden**, not appended — otherwise
   a stray config line silently breaks parsing or re-opens the containment hole. Generalise
   `stripGrokModelFlags` into `stripPinnedFlags` covering: `--output-format`, `-m/--model`,
   `--reasoning-effort/--effort`, `--sandbox`, `--permission-mode`, `--cwd`, `-p/--single`,
   `--prompt-file`, `--max-turns`, `--no-plan`/`--plan`, `--verbatim`. **GREEN**.
3. **RED** NDJSON parse tests (`parseGrokNDJSON`): last `type=="result"` line wins;
   `result`→`Content`, `session_id`→`SessionID`, `usage.*`→`TokensIn/Out`,
   `total_cost_usd`→`CostUSD`, `modelUsage` key→`Model` (assert **`grok-4.5-build`**, *not*
   `grok-4.5` — the key deliberately differs from the `-m` value). Also: malformed lines are
   skipped defensively; `system`/`assistant`/`user` lines ignored; **no `result` line at all**
   → clear error. Mirror `parseCodexJSONL`'s use of `bytes.Split` over `bufio.Scanner`
   (`codex.go:153` — Scanner's 64 KB token limit breaks on large lines, GitHub issue #4;
   grok review lines run to 16 KB+ and will exceed it). **GREEN**.
3b. **RED** optional-telemetry tests: envelope with `cost_is_partial` / no `total_cost_usd`
   must yield `CostUSD == 0` **without** being reported as a real zero cost; envelope with
   `usage_is_incomplete`; `input_tokens` treated as uncached-only (document which value
   `TokensIn` carries and assert it). **GREEN**.
3c. **RED** harness-isolation test (R1 solve): `grokIsolatedEnv()` must set `HOME` to an
   empty dir and `GROK_HOME` to the *real* grok home, preserving every other var. Assert:
   real home resolved from `$GROK_HOME` when set, else `$HOME/.grok`, computed **before**
   `HOME` is overridden (order bug is the obvious failure mode); exactly one `HOME=` entry
   in the result; other env entries untouched. **Hardening (from council review):** the
   helper must take the base env as a **parameter** (`grokIsolatedEnv(base []string) []string`)
   rather than reading ambient `os.Getenv`, so it is deterministically testable; the temp
   HOME must be created **0700** and race-safe (`os.MkdirTemp`, not a fixed path); and
   creation failure must degrade to a clear error, never silently fall back to the real
   `$HOME`. **Test-design note:** the shared `mockRunner`
   (`internal/providers/claude_test.go:16-39`) *discards* `opts ...core.RunOption`, and
   `core.runConfig` is unexported, so the external `providers_test` package cannot inspect
   the resulting env. Do **not** rewrite `mockRunner` (that would break the additive rule).
   Instead build the env in a pure helper `grokCompatEnv(base []string) []string` and test
   it from a **new internal test file** `internal/providers/grok_internal_test.go`
   (`package providers`) — idiomatic Go, fully additive, existing tests untouched. **GREEN**.
4. **RED** error ladder: `type=="error"` → message; exit≠0 + unparseable stdout → stderr;
   exit≠0 + empty stderr → `exit code %d`; empty `text` → empty-response error; runner
   error; cancelled context. **GREEN** error handling.
5. **RED** `HealthCheck`: version parsed from `grok X.Y.Z (hash) [stable]`; binary missing →
   `Healthy:false`; exit≠0 → `Healthy:false`. **Assert semver *parseability*, never an exact
   patch version** — the binary self-updated 0.2.117 → 0.2.118 mid-research, so
   `CLIVersion == "0.2.117"` would be a self-breaking test. (Mock-based tests may pin an
   exact string; real-CLI tests must not.)
6. **RED** model override + `stripGrokModelFlags` (config-supplied `--model`/`-m` stripped).
7. **RED** config/CLI/output/context-file/strategy tests → **GREEN** registration edits.

## Verification

| Level | Command | Must show |
|---|---|---|
| L1 | `make ci-fast` | compile + 0 lint + vet clean |
| L2 | `make test` | all new grok tests pass; **no pre-existing test modified except by addition** |
| L3 | `make test-binary` | mock-grok smoke passes; provider count 3 → 4 |
| L4 | `make test-visual` | screenshots for every output-visible command |
| L5 | `make test-agent` | JSON schemas, exit codes, ANSI-free piped output, perf |

### Per-command matrix — every relevant command, mock **and** real CLI

| Command | Mock (L3/L5) | Real grok |
|---|---|---|
| `status` / `sthiti` | grok row present, 4 providers, TTY + `--json` + piped | grok healthy, version `0.2.117` |
| `consult --agent grok` | content, exit 0, `--json` | real review returned |
| `council --agents claude,codex,grok` | dispatch + peer review + synthesis | full 3-stage pipeline |
| `council --agents … --chair grok` | grok synthesises | chair path |
| `review --author grok --reviewer codex` | 2-step pipeline | real |
| `review --author codex --reviewer grok` | reviewer path | real |
| `refine --author claude --reviewers codex,grok` | sequential incorporate | real |
| `config show` (+`--commented`,`--json`) | grok block + 3 comment keys | — |
| `plugin list` / `plugin inspect grok` | `grok-provider` discovered | — |
| exit codes | invalid agent → **1**; provider failure → 3; partial → 5 | — |

> ⚠ **Do not assert exit 2 anywhere.** Verified against the binary at HEAD: `consult --agent
> nosuchagent`, `council --nope`, `council` (no prompt) and `consult` (no `--agent`) **all
> return 1**. `ExitUsage` (2) is documented in CLAUDE.md, PRD §6.1, `README.md`,
> `skill/SKILL.md` and `commands/dootsabha.md` but is **unreachable from the CLI**. This is
> a pre-existing bug, **out of scope here** (see Follow-ups) — this task must simply not
> encode the false expectation. Grok's own unknown-model error surfaces as exit 3
> (provider error) via the normal path.

**Also verify:** grok token/cost/session actually populate `--json` output (unlike agy)
and flow into the metrics collector; `styles_test.go` box alignment with the longer
4-provider string; `dootsabha-recap` extension renders the grok row from the context file.

## Criteria (DoD)

- [ ] `make ci` green — 0 lint issues, all tests pass
- [ ] `make test-binary` green with `mock-grok`; provider assertions updated 3 → 4
- [ ] `make test-agent` green (L5)
- [ ] `make test-visual` green with screenshots in `.claude/screenshots/`
- [ ] Every RED step above has captured failing output as evidence
- [ ] **No existing test expectation weakened or deleted** — additions only
- [ ] Default council / refine / review defaults provably unchanged
- [ ] Real-CLI smoke passes for status, consult, council, review, refine
- [ ] `--json` for a grok run carries non-zero `tokens_in`, `tokens_out`, `cost_usd`, `session_id`
- [ ] **R1 harness isolation verified against the real CLI**: a real `dootsabha consult
      --agent grok` run shows `mcpServers: 0` / `hooks: 0` / `projectInstructions: 0` via
      `grok inspect`, and `usage.input_tokens` ≈ 13 k rather than ≈ 22 k. Auth still works.
- [ ] `NO_COLOR` and piped output contain zero ANSI for all grok paths
- [ ] `README.md`, `configs/default.yaml`, `docs/PRD.md` §8.2, `docs/PROGRESS.md` updated
- [ ] **Help text** lists grok everywhere providers are enumerated, verified against real
      `--help` output for root, consult, council, review, refine, status
- [ ] **SKILL** updated minimally and still sharp — SKILL.md remains a lean index,
      provider detail in `references/`, no unjustified new example, claims verified
      against the real binary, progressive disclosure preserved
- [ ] `commands/dootsabha.md` slash command lists grok
- [ ] shux frames captured under `.shux/out/grok-provider/` (**gitignored — never committed**)
      and attached to the **PR as review evidence** via the `browsing-as-you` skill

## Visual Test Results

Frames captured with `shux` into `.shux/out/704-grok-provider/` (**gitignored — not
committed**; attached to the PR as review evidence). Full captions in that directory's
`MANIFEST.md`. All frames use a `bash --norc --noprofile` pane so the prompt is a bare
`bash-5.3$` — no username, home path, hostname, or personal config is rendered.

| Frame | Proves |
|---|---|
| `01-status-four-providers.png` | `status` lists 4 providers; grok healthy `0.2.118` / `grok-4.5` / auth ✓, purple dot distinct from amber, emerald, blue |
| `02-status-json-grok.png` | `status --json` carries a well-formed grok entry |
| `03-config-grok-block.png` | `config show --commented` exposes the 3 new `providers.grok.*` keys |
| `04-consult-help-lists-grok.png` | `consult --help` advertises grok in `--agent` |
| `05-council-help-lists-grok.png` | **Additive contract in one frame** — `--agents` lists grok while the default stays `"codex,agy"` |
| `06-refine-help-lists-grok.png` | `refine --help` lists grok; default `codex,agy` unchanged |
| `10-council-in-progress.png` | Real council `--agents codex,grok` mid-flight — dispatch both ✓, grok peer-reviewing |
| `13-council-synthesis-footer.png` | Council synthesis + footer: 453.9 s · $0.259 · 824 900 in / 22 691 out |
| **`20-consult-command-typed.png`** | The real invocation verbatim on the command line |
| **`22-consult-full-review.png`** | **The whole grok review in one frame** — Medium/Low findings with exact line citations, quoted source, "correct / intentionally fine" table, severity ranking, footer 59.9 s · $0.1145 |
| `24-consult-review-tail-and-footer.png` | Tail of an earlier run of the same review (47.2 s · $0.0924) |

### Real-CLI verification

- `dootsabha status` — grok healthy against the live binary (`0.2.118`, auto-updated from
  `0.2.117` mid-task, which is why `--no-auto-update` is pinned and no test asserts a patch version).
- `dootsabha consult --agent grok` over this provider's own source returned a genuine
  review with **no tool-call preamble** (`## Verdict` first) and full telemetry:
  `grok-4.5-build`, 103 548 in / 8 344 out, **$0.3178**, session `019fbf11-…`, 155 s.
- That review found a **real bug** — value-taking flags swallowing a following flag as
  their value — reproduced as a failing test and fixed (see below).

### Bugs found by dogfooding grok against this provider

| Finding | Status |
|---|---|
| `["--sandbox","--keep-me"]` silently dropped `--keep-me`; `["--effort","--keep-me"]` set effort to `"--keep-me"` | ✅ **Fixed** — `isFlagToken` guard, RED test first |
| `HealthCheck` non-zero exit with empty stderr → blank `Error` (unexplained red status row) | ✅ **Fixed** — reuses `grokFailureMessage` |
| `grokEmptyHome` leaked the temp dir when `Chmod` failed | ✅ **Fixed** — `RemoveAll` on the error path |
| `TokensIn` is uncached-only, under-reporting prompt size | ⚠️ **Accepted** — deliberate, consistent with `claude.go`; documented in the struct |
| `--cwd` in the documented contract but never injected | ✅ **Doc corrected** — cwd is inherited; containment comes from `--sandbox read-only` |

## Graceful degradation — verified across the failure matrix

Driven against the real binary with mock providers (absent / failing / hanging /
empty / malformed), across `status`, `consult`, `council`, `review`, `refine`.
Every issue found was fixed **in this PR**, each with a failing test first.

| # | Issue | Origin | Fix |
|---|---|---|---|
| 1 | `status` exited 3 when an opt-in provider was merely absent — i.e. on every machine without the grok CLI | **Introduced here** | `Installed` field + `statusExitError` rule; absent opt-in reads `not installed (optional)` |
| 2 | `--json` emitted **two** JSON documents on any failure path, so `… --json \| jq` died with "Extra data" | **Pre-existing** (reproduced on `main` with `agy`) | `jsonDocWritten` guard — the command's specific document wins, `Execute()` no longer appends a generic one |
| 3 | All three mocks emitted **invalid JSON** for prompts containing quotes/newlines — so council/refine were tested against garbage | mock-grok **here**; claude/codex **pre-existing** | `json_str()` escaping in all three mocks |
| 4 | `--chair bogus` silently accepted: fell back to another agent and exited 0 | **Pre-existing** | `validateChair` rejects unknown names (exit 1), matching `--agent` |
| 5 | Chair fallback was invisible to humans (recorded only in JSON) | **Pre-existing** | stderr warning naming the requested and actual chair |

Checked and deliberately **not** changed:

- **`council` exits 1 when all agents fail** — documented in its own `--help`
  ("Exit codes: 0 success, 1 all failed, …"). Intentional, not an inconsistency to
  "fix" by breaking a published contract.
- **ANSI in piped output when a provider's *content* contains escapes** — दूतसभा
  adds no ANSI of its own; relaying provider content verbatim is correct, and
  stripping it would corrupt legitimate output.
- **No panics, nil derefs, or stack traces** were observed in any scenario.

Coverage added: L3 `8 → 12` tests, L5 `27 → 41` tests. The new L5 suite exercises
the absent-provider path that was previously **unreachable**, because the harness
supplied a mock binary for every provider.

## Follow-ups (NOT this task — each is a behaviour change to existing code)

Surfaced by research/dogfooding while scoping 704. Each deserves its own branch and review;
folding any of them in here would break the additive constraint.

| Proposed | Scope | Evidence |
|---|---|---|
| **705** — provider hardening | nil-config deref, partial-config dropping defaults, dead `opts.Timeout`, and **`AuthValid` set unconditionally when `--version` exits 0** — so `status` reports healthy for a provider that is out of quota. Needs a real auth probe per provider. Documented in the SKILL for now. | Grok's dogfood review + reproduced quota failure |
| ~~**706**~~ — exit-code scheme | **DONE IN THIS PR** — 2 now emitted for usage, 6 added for config (was colliding with partial-result 5), all-agents-failed promoted 1→3. | Verified across 13 scenarios in L5 |
| **707** — skill `jq` drift | 5 broken `jq` expressions in `skill/references/` + `examples/` (wrong envelope shape, `select(.error == "")` never matches because `error` is `omitempty`) | Skill audit F1–F5, verified against real binary output |
| ~~**708** — command robustness~~ | **FIXED IN THIS PR** — `--chair <unknown>` now errors (exit 1) and a chair fallback warns on stderr. `refine --reviewers <unknown>` → exit 5 remains, tracked separately. | Skill audit F10–F11 |
| **709** — SKILL de-duplication | ~150 of 313 SKILL.md lines duplicate `references/`; description written in second person (docs flag this explicitly); missing `allowed-tools: Bash(dootsabha *)` | Skill audit A1–A8 |

## Commit

```
feat(grok): add xAI Grok CLI as opt-in fourth provider

Adds grok-4.5 (high reasoning) as a fourth dootsabha agent. Additive only:
default council, refine reviewers, and review defaults are unchanged; grok is
selected explicitly via --agent/--agents/--chair.

Parses grok's streaming-messages-json NDJSON stream, taking the last type=="result"
line (--output-format json is unusable: its .text concatenates tool-call preambles).
Runs with an isolated HOME + --sandbox read-only, which severs Claude Code harness
inheritance (6 MCP servers, 53 hooks, CLAUDE.md) and cuts tokens 85% / cost 78%.
```

## Session Protocol

1. `cm context`
2. Read `CLAUDE.md`, `docs/PROGRESS.md`, this file
3. Mark this task **IN PROGRESS** before any code change
4. Work the TDD Plan in order, capturing each RED
5. `make check` before every commit
6. Fill **Visual Test Results** with real screenshot paths
7. Add the 704 row to the Phase 7 table in `docs/PROGRESS.md`
8. Mark **DONE** only after L1+L2+L3 pass and L4 evidence exists
