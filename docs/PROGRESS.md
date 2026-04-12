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
- `dootsabha consult --agent claude/codex/gemini "prompt"` — invokes real CLIs, parses JSON/JSONL
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
- `dootsabha refine "prompt" --author claude --reviewers codex,gemini` — sequential review + incorporate
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
- 3 provider plugin binaries (claude-provider, codex-provider, gemini-provider)
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
| 702 | Codex default model → `gpt-5.4` | DONE | — |
