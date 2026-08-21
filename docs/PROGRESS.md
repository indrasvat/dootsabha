# दूतसभा Progress

## Phase 0: Spikes (All Complete)

| Task | Spike | Status | Key Finding |
|------|-------|--------|-------------|
| 0.1 | Codex JSONL | DONE | 3 undocumented event types (error events, cached_input_tokens, error items). bufio.Scanner + json.Unmarshal per line works. |
| 0.2 | Claude JSON | DONE | Must strip CLAUDECODE* env vars entirely (not empty-string). is_error field, not exit code, discriminates errors. |
| 0.3 | Gemini JSON | DONE | Dual-model architecture (flash-lite router + flash main). Wall-clock ~10s but API latency ~1s. No JSON error format — stderr only. |
| 0.4 | go-plugin gRPC | DONE | Handshake 7.6ms median (26x under 200ms gate). Re-launch on crash, don't retry. Kill plugins explicitly. |
| 0.5 | Subprocess Mgmt | DONE | exec.Command (NOT CommandContext) for SIGTERM→grace→SIGKILL. Setpgid works under macOS SIP. errgroup is the reaper. |
| 0.6 | Cobra Alias | DONE | cobra.ArbitraryArgs required for extension discovery. Devanagari aliases work natively. Tab completion needs ValidArgsFunction workaround. |
| 0.7 | Terminal UX | DONE | huh v0.8.0 has NO standalone spinner (use raw goroutine on stderr). All 4 lipgloss pitfalls reproduced and documented. Color gate required for piped output. |
| 0.8 | PTY vs Pipe | DONE | creack/pty NOT needed. All 3 CLIs work via plain os/exec pipe with YOLO+JSON flags. |

### Critical PRD Updates Needed

All 4 items addressed in PRD v1.6.

## Phase 1: Foundation (All Complete)

| Task | Description | Status | Agent (Jaane Bhi Do Yaaro) |
|------|-------------|--------|---------------------------|
| 1.1 | Project Scaffold + Makefile + Gating Hooks | DONE | vinod (Wave 1) |
| 1.2 | Render Context & Output Foundation | DONE | sudhir (Wave 2) |
| 1.3 | Config Manager (Viper) | DONE | shobha (Wave 2) |
| 1.4 | Subprocess Runner | DONE | dmello (Wave 2) |
| 1.5 | Claude Provider (Hardcoded) | DONE | tarneja (Wave 3) |
| 1.6 | Codex + Gemini Providers | DONE | ahuja (Wave 4) |
| 1.7 | CLI Wiring (consult/status/config) | DONE | asrani (Wave 5) |
| 1.8 | Status Bugfix (version, dot column, models) | DONE | — |
| 1.9 | Align code with architecture doc (model, flags) | DONE | — |

### What Works End-to-End
- `dootsabha consult --agent claude/codex/agy "prompt"` — invokes real CLIs, parses JSON/JSONL (agy is plain-text)
- `dootsabha status` — health table with provider dots (TTY/pipe/JSON modes)
- `dootsabha config show` — merged config with key redaction
- Bilingual aliases: paraamarsh, sthiti, vinyaas + Devanagari
- `make ci` — 0 lint issues, all tests pass
- `make test-binary` — 8/8 L3 smoke tests

## Phase 2: Council Pipeline (All Complete)

| Task | Description | Status | Agent |
|------|-------------|--------|-------|
| 2.1 | Parallel Dispatch (errgroup + progress) | DONE | council-builder |
| 2.2 | Peer Review Stage (32KB truncation, cross-review) | DONE | council-builder |
| 2.3 | Synthesis Stage (chair + fallback + multi-round) | DONE | council-builder |
| 2.4 | Review Command (author + reviewer pipeline) | DONE | review-builder |
| 2.6 | Refine Command (sequential review + incorporation) | DONE | — |
| 207 | Output Polish — Professional CLI Rendering | DONE | — |

### What Works End-to-End
- `dootsabha council "prompt"` — 3-stage pipeline: dispatch → peer review → synthesis
- `dootsabha council "prompt" --json` — JSON with dispatch/reviews/synthesis/meta
- `dootsabha council "prompt" --parallel=false` — sequential dispatch mode
- `dootsabha council "prompt" --agents claude,codex --chair codex` — agent/chair override
- `dootsabha council "prompt" --rounds 2` — multi-round with context chaining
- `dootsabha review "prompt" --author codex --reviewer claude` — 2-step pipeline
- `dootsabha review "prompt" --json` — JSON with author/review/meta
- `dootsabha refine "prompt" --author claude --reviewers codex,agy` — sequential review + incorporate
- `dootsabha refine "prompt" --json` — JSON with versions/final/meta
- Bilingual aliases: sabha/सभा (council), sameeksha/समीक्षा (review), sanshodhan/संशोधन (refine)
- Bilingual flags: --dootas, --adhyaksha, --chakra, --samantar, --kartaa, --pareekshak, --gupt
- Anonymous review mode (default) — Karpathy llm-council pattern
- Max 5 agents enforced, 32KB truncation for peer review + synthesis
- Chair failure → fallback to first healthy non-chair agent
- Exit code 5 for partial results (some agents failed)
- Progress rendering on stderr (TTY only)
- Professional CLI rendering: rounded header boxes, `──` section dividers, provider-colored dots, content separators, pipe-delimited footers
- Graceful degradation: TTY+color → TTY+NO_COLOR → piped (no ANSI, no box chars)
- `make ci` — 0 lint issues, all tests pass
- `make test-binary` — 8/8 L3 smoke tests
- L4 visual tests: 14/14 pass with 4 screenshots (refine, council, review, consult)

## Phase 3: Plugin System (All Complete)

| Task | Description | Status | Agent |
|------|-------------|--------|-------|
| 3.1 | Proto Definitions + Code Generation | DONE | — |
| 3.2 | Plugin Manager (Discovery, Loading, Registry) | DONE | — |
| 3.3 | Extract Providers to Plugins | DONE | — |
| 3.4 | Council Strategy Plugin | DONE | — |
| 3.5 | Extension Discovery | DONE | — |
| 3.6 | Plugin Command (vistaarak) | DONE | — |

### What Works End-to-End
- `proto/provider.proto`, `strategy.proto`, `hook.proto` — full gRPC service contracts
- `proto/gen/` — vendored generated Go code (48 L2 tests: message, serialization, edge cases)
- `internal/plugin/convert.go` — Go type ↔ proto conversion helpers (13 L2 tests)
- `internal/plugin/interfaces.go` — Go interfaces matching proto services
- `internal/plugin/handshake.go` — distinct HandshakeConfig per plugin type with magic cookies
- `internal/plugin/{provider,strategy,hook}_grpc.go` — GRPCServer/GRPCClient wrappers
- `internal/plugin/manager.go` — plugin discovery, loading, registry, graceful shutdown
- 3 mock plugin binaries (mock-provider, mock-strategy, mock-hook) — real go-plugin gRPC processes
- 24 go-plugin integration tests — actual RPC calls against running plugin binaries
- 21 manager tests — discovery, load, registry, remove, shutdown, end-to-end
- Full pipeline test: hook rewrites prompt → provider invokes → hook redacts response
- Crash recovery: kill plugin, detect error, relaunch succeeds
- Handshake mismatch: wrong MagicCookieValue correctly rejected
- 3 provider plugin binaries (claude-provider, codex-provider, agy-provider)
- Plugin smoke tests: 8/8 pass (binary existence, integration tests, no orphans)
- `make build-plugins` target builds all 3 provider plugins
- `make test-plugins` target runs plugin smoke tests
- Extension discovery: `dootsabha-{name}` binaries on $PATH auto-discovered and executed
- 12 extension discovery tests (discovery, dedup, edge cases, find, env)
- Council strategy plugin: `plugins/council-strategy/` — wraps dispatch→review→synthesis pipeline
- 12 strategy unit tests (response building, tokens, costs, errors, fallback, status map)
- `make build-plugins` target builds all 4 plugins (3 provider + 1 strategy)
- `dootsabha plugin list` / `vistaarak soochi` — lists gRPC plugins + PATH extensions
- `dootsabha plugin inspect {name}` / `vistaarak parikshan` — detailed plugin info
- Bilingual aliases: vistaarak/विस्तारक (plugin), soochi/सूची (list), parikshan/परीक्षण (inspect)
- JSON output: `--json` for machine consumption with schema_version envelope
- 14 plugin command tests (type inference, JSON, aliases, discovery, rendering)
- `make check` — 0 lint issues, all tests pass, 8/8 L3 smoke tests

## Phase 4: Hardening & Polish (All Complete)

| Task | Description | Status | Agent |
|------|-------------|--------|-------|
| 4.1 | Structured Logging (slog) | DONE | — |
| 4.2 | Metrics Collection (In-Process Counters) | DONE | — |
| 4.3 | Edge Cases & Error Paths | DONE | — |
| 4.4 | Tier 2 Context File for Extensions | DONE | — |
| 4.5 | Full L5 Acceptance Suite | DONE | — |

### What Works End-to-End
- `internal/observability/logger.go` — slog with JSON/text handlers on stderr
- `-v`/`-vv`/`-vvv` verbosity levels (Warn → Info → Debug → Debug+source)
- Session trace ID (`ds_{random5}`) in all log entries
- 9 logger tests (level mapping, JSON/text output, filtering, source)
- `internal/observability/metrics.go` — thread-safe per-provider metrics collector
- Per-provider: invocations, duration, cost, tokens (in/out), errors
- Session-level: total duration, total cost, total tokens
- 9 metrics tests (single, multiple, errors, concurrent, summary, aggregation)
- Exit code precedence matrix: 2 > 4 > 3 > 5 > 1 > 0 (PRD §6.1)
- Exit code constants: ExitSuccess(0), ExitError(1), ExitUsage(2), ExitProvider(3), ExitTimeout(4), ExitPartial(5)
- 30 precedence tests (pairwise + multi-code scenarios)
- SIGPIPE handling: exit 0 cleanly when piped to head
- `internal/plugin/context_file.go` — Tier 2 JSON context for extensions
- ContextFile struct: version, session_id, workspace, providers, capabilities, tty, terminal_width
- WriteContextFile creates temp JSON, DefaultContextFile with sensible defaults
- Wired into execExtension: context file created → DOOTSABHA_CONTEXT_FILE env var set → cleanup on exit
- 7 context file tests (valid JSON, all fields, cleanup, providers, capabilities, defaults, empty)
- L5 agent workflow tests: 27 tests across 10 categories (JSON, exit codes, ANSI, fields, status, errors, perf, aliases, context file, SIGPIPE)
- L4 full acceptance suite: 24 visual tests with 8 screenshots (help, status, consult, config, plugin, errors, json, piped)
- Performance verified: startup 25ms, --help 26ms, consult 33ms (all well under 2s target)
- `make check` — 0 lint issues, all tests pass, 8/8 L3 smoke tests
- `make test-agent` — 27/27 L5 tests pass

## Phase 5: Documentation & Release (All Complete)

| Task | Description | Status | Agent |
|------|-------------|--------|-------|
| 5.1 | README + User Guide | DONE | — |
| 5.2 | Default Config + Embedded Docs | DONE | — |
| 5.3 | Claude Code SKILL | DONE | — |
| 5.4 | Build & Release CI | DONE | — |
| 5.5 | Final Acceptance | DONE | — |

### What Works End-to-End
- `configs/default.yaml` — comprehensive with inline YAML comments for all options
- `ConfigComments` map — 14 entries covering all config keys for `--commented` output
- `dootsabha config show` — works with zero-config (embedded defaults via Viper)
- `dootsabha config show --commented` — inline `# description` for every field
- `dootsabha config show --json` — valid JSON output
- `dootsabha config show --reveal` — shows sensitive values (disables redaction)
- 11 config tests: defaults, file load, env override, unknown keys, redaction, reveal, duration parsing, merge order, comments keys, comments not empty, no-file defaults
- README.md: hero, quick start, commands reference, config guide, output modes, exit codes, extensions guide, plugin system, bilingual interface, development guide
- All README commands verified: --version, status --json, config show --commented, config show --json, all 7 bilingual aliases
- `skill/SKILL.md` — Claude Code SKILL with YAML frontmatter, trigger words, and accurate JSON schemas
- `skill/references/` — command-reference.md, exit-codes.md
- `skill/examples/` — council-deliberation.md, review-refine.md
- SKILL follows gh-ghent structure: frontmatter + supporting files + progressive disclosure
- SKILL jq patterns verified against actual binary output
- CI workflow: fmt-check + fix-check + lint + vet + test + build + test-binary on all branches
- Release workflow: 4 cross-compile targets (darwin/linux × amd64/arm64), checksums, GitHub release
- Cross-compilation verified: linux/amd64 (ELF x86-64), linux/arm64 (ELF aarch64)
- Version injection verified: ldflags → `dootsabha version v0.1.0-test (abcd1234)`
- Final acceptance: L1 (pre-commit) + L2 (all tests) + L3 (8/8 smoke) + L4 (117 screenshots) + L5 (27/27 agent tests)
- All JSON outputs valid: consult, status, config, plugin
- Zero ANSI in piped output: consult, status, config, plugin
- All 7 bilingual aliases verified
- Cross-compilation: 4 targets (darwin/linux × amd64/arm64)
- All checklist items from Task 5.5 verified

## Phase 6: Extension Showcase (All Complete)

| Task | Description | Status | Agent |
|------|-------------|--------|-------|
| 6.1 | Recap Extension + Enhanced Discovery | DONE | — |

### What Works End-to-End
- `ExtensionDirs()` returns `[~/.local/bin, /usr/local/bin]` — user-local wins scan order
- Extra dirs prepended before $PATH in both `FindExtension` and `DiscoverExtensions`
- 3 new L2 tests for `ExtensionDirs()` (paths, order, prepend behavior)
- `dootsabha recap` — workspace intelligence briefing via Python extension
- Uses ALL Tier 2 context fields: version, session_id, workspace, providers, capabilities, tty, terminal_width
- Rich TTY output: manual header box (Go CommandHeader style), provider matrix with colored dots, Rule dividers, styled suggestions
- Graceful degradation: TTY+color → piped (no ANSI, `*` dots, `---` markers) → standalone (no providers)
- `dootsabha plugin list` shows recap as extension from `~/.local/bin`
- Git analysis: branch, recent commits, staged/unstaged counts, language detection, topic extraction
- Suggestion engine: cross-references workspace + providers + capabilities → actionable commands
- L4 visual tests: 10/10 pass with 2 screenshots (TTY, piped)

## Phase 7: Maintenance

| Task | Description | Status | Agent |
|------|-------------|--------|-------|
| 702 | Provider default model refresh | DONE | — |
| 703 | Replace Gemini provider with Antigravity (agy) | DONE | — |
| 704 | Add xAI Grok CLI as opt-in fourth provider | DONE | — |
| 705 | Per-provider timeouts + enforced session timeout (#20) | DONE | — |
| 706 | Default grok model → grok-4.6 (+ xhigh effort) | DONE | — |
| 707 | Default agy model → Gemini 3.7 Flash (High) + `--output-format json` | DONE | — |
| 708 | Forward the per-call budget to `agy --print-timeout` | PENDING | 707 |

### What Works End-to-End (705)

- `--timeout` is the budget for **one agent call**; every call in a pipeline gets
  its own window. A slow author no longer starves the reviewer (issue #20).
- `--session-timeout` is enforced for the first time and bounds the **whole
  pipeline** — unhidden, with a `--satra-seema` alias. `0` disables the ceiling.
- Both exit `4`; the message names which fired (`invocation timeout after …` /
  `session timeout after …`) so the caller raises the right knob.
- `core.Budget` owns the two deadlines; `core.StepContext` derives one call's
  window. The council engine reads `InvokeOptions.Timeout`, which until now was
  declared but never used.
- A single agent hitting its own deadline no longer ends the run: `refine` moves
  on to the next reviewer, `council` keeps its healthy agents' output.
- A timed-out chair still reaches the exit code, including through a strategy
  plugin (`chair_error` on the proto), and the ceiling warning is pipeline-aware.
- `internal/cli/outcome.go` is the single exit-code decision for every pipeline,
  replacing three per-command aggregators (90 insertions, 367 deletions). Four
  freeze guards, each negative-tested, make forgetting a stage or result type a
  build failure.
- Tests: 23/23 L3, 234/234 L5, new unit coverage in core/cli/plugin, `-race` clean.
  Real-CLI proof of old-vs-new in `.shux/out/20/09-real-cli-before-after.png`.

### What Works End-to-End (706)

- Default grok model bumped `grok-4.5` → **`grok-4.6`** (grok CLI 1.0.5's own
  default). The literal used to appear in **six** places: the provider constant,
  the viper default, the plugin's `DefaultModel` *and* `SupportedModels[0]`,
  `configs/default.yaml`, and the extension-context defaults. Now
  `providers.GrokDefaultModel` is the single source of truth that the plugin and
  the extension context **read** (so those cannot drift at all), and the two that
  structurally cannot import it — the viper default in `internal/core`, and the
  static `configs/default.yaml` skeleton — are covered by
  `TestGrokDefaultModelSourcesAgree`. The skeleton was previously guarded by
  nothing: breaking it alone failed zero tests.
  (This addresses grok's default only; the other three providers still carry
  duplicated model literals — same drift class, out of scope here.)
- **Not a migration.** `grok-4.5` is still live and still offered in
  `SupportedModels`; an explicit `providers.grok.model: grok-4.5` is never
  rewritten. A *malformed* pin (`model: [grok-4.5]` — a plausible typo, since the
  sibling `flags` key IS a list) used to be silently dropped, so the bumped
  default ran while `config show` still displayed the pin. `LoadConfig` now
  rejects a non-string `binary`/`model` for any provider with exit 6.
- `xhigh` reasoning effort (new in grok-4.6) passes through in all four spellings
  (`--reasoning-effort`/`--effort`, space and `=` forms), each asserted at argv;
  the default stays `high`. Fixed `--reasoning-effort=` (empty value) forwarding an
  empty argv token.
- `mock-grok` now **echoes** the `-m` it receives as `<model>-build`, so L3/L5 can
  prove model forwarding end-to-end. L5 already drove grok through the binary
  extensively, but only on **failure** paths (`/nonexistent/grok`, hostile stubs);
  the success path had no coverage, and the mock's hardcoded model meant even that
  could not have caught a forwarding regression. Its default is now a sentinel
  (`unset`), so a dropped `-m` fails loudly instead of echoing the right answer
  by accident.

### What Works End-to-End (707)

- Default agy model is **`Gemini 3.7 Flash (High)`** (Google shipped 3.7 Flash on
  2026-08-13; agy 1.1.17 makes it Antigravity's own default). दूतसभा always emits
  `--model`, so it had been actively forcing every call back onto 3.5.
- **Not a migration.** 3.6/3.5/3.1 are still listed by `agy models`, stay in
  `SupportedModels`, and an explicit `providers.agy.model` is never rewritten.
- The literal lived in **six** unsynced places. Three now read
  `providers.AgyDefaultModel`; core's viper default and its migration writer share
  one `core` constant; `configs/default.yaml` — previously guarded by nothing — is
  covered by a drift test.
- **agy is now driven with `--output-format json`.** `TokensIn`/`TokensOut` and
  `SessionID` (its conversation id) are populated. `CostUSD` stays `0`: the CLI
  reports no cost and दूतसभा does not estimate one.
- **Error text is no longer lost.** In JSON mode agy writes the reason to *stdout*
  and leaves stderr empty; the old provider read stderr and surfaced a bare
  `exit code 1`.
- `status` is a **tool-level** diagnostic, not the failure discriminator: a turn
  whose tool call failed but which still answered returns exit 0 + `status:
  "ERROR"` + a usable response. The exit code decides; degraded turns log at Warn.
- `--output-format` joins `--model` as a **pinned** flag — a configured
  `--output-format text` can no longer break the parser.
- `mock-agy` now speaks the JSON envelope and **echoes the `--model`** it receives
  (sentinel default `unset`), so L3/L5 can see forwarding at all — it previously
  discarded the flag outright.
- `AgyProvider.providerConfig` is nil-safe, matching `GrokProvider`.
- L3 28 → 33, L5 236 → 238.
- Real-CLI findings recorded in `docs/agy-cli-findings.md`; the
  `--print-timeout` interaction it uncovered is tracked as task 708.

### What Works End-to-End (704)

- `grok` (xAI Grok CLI) is a fourth agent, **opt-in**: default council,
  refine reviewers and review defaults are unchanged. Select with `--agent grok`,
  `--agents …,grok`, `--chair grok`, `--reviewers …,grok`.
- Parses `--output-format streaming-messages-json`, taking the last `type=="result"`
  line. `--output-format json` is unusable — its `.text` concatenates tool-call
  preambles into the answer.
- Runs with `--sandbox read-only` and an isolated `$HOME`, which severs Claude Code
  harness inheritance (6 MCP servers, 53 hooks, `CLAUDE.md`) and cuts tokens ~85%.
- Richest telemetry of any provider: content + tokens + cost + session id.
- **Exit codes reworked** to one code per caller action:
  `0` proceed · `1` internal bug · `2` fix the command · `3` retry/other agent ·
  `4` raise timeout · `5` usable with gaps · `6` fix the config.
  Precedence `2 > 6 > 4 > 3 > 5 > 1 > 0`, computed via `core.HighestExitCode`.
- `--json` carries the same exit code as text mode, and stdout is always exactly
  one JSON document — both now enforced by the harness on every case.
- L3 8 → 12, L5 27 → 183 tests.

### What Works End-to-End (703)
- Gemini CLI retired (sunset 2026-06-18) → replaced by `agy` (Antigravity CLI) as the 3rd agent
- `internal/providers/agy.go` — plain-text print-mode provider (`agy --dangerously-skip-permissions -p`); no token/cost/session data
- `plugins/agy/` — gRPC provider plugin (replaces `plugins/gemini/`); `make build-plugins` builds `agy-provider`
- Defaults: council `claude,codex,agy` (`codex,agy` inside Claude Code); refine reviewers `codex,agy`; default model `Gemini 3.5 Flash (High)` *(superseded by 707 — see above)*
- `dootsabha config migrate` (स्थानांतरण) — rewrites stale `providers.gemini`/`council.chair: gemini` → `agy`, writes `<config>.bak`; `--dry-run`, `--json`
- TTY stderr nudge when a stale `gemini` config reference is detected
- `mock-agy` (plain text) replaces `mock-gemini`; all gemini references removed from code/config/scripts/docs/skill
- Real-CLI verified: consult/council/review/refine/status against live `agy` 1.0.8
- `make ci` — 0 lint issues, all tests pass; `make test-binary` — 8/8 L3 smoke
