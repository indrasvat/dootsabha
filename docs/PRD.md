# दूतसभा — Product Requirements Document

| Field | Value |
|-------|-------|
| **Version** | 1.4 |
| **Author** | indrasvat |
| **Date** | 2026-02-28 |
| **Status** | Draft |
| **Architecture** | `docs/dootsabha-architecture.html` — authoritative source for all interfaces, plugin system, and sequence diagrams |
| **Build Plan** | `docs/dootsabha-buildplan.html` — phased build plan with testing methodology and agent workflow |

---

## How to Use This Document (Progressive Disclosure)

> **Agents: do NOT read this entire file.** Read only the sections your task references.

```
Layer 1 — CLAUDE.md              ≤200 lines   Always read first. Build commands, pitfalls.
Layer 2 — PROGRESS.md            ~50 lines    Current state: what's done, what's next.
Layer 3 — Task file              ≤150 lines   Self-contained. Lists PRD §X.Y to read.
Layer 4 — PRD §X.Y (this file)   per section  Read ONLY the §X.Y sections listed in your task.
Layer 5 — Detail docs            as needed    Architecture, testing-strategy, build plan.
```

**Document hierarchy:**

| Document | Purpose | When to Read |
|----------|---------|-------------|
| `CLAUDE.md` | Build commands, conventions, pitfalls, learnings | Every session start |
| `docs/PROGRESS.md` | Phase/task completion state, session log | Every session start |
| `docs/tasks/NNN-*.md` | Per-task spec: steps, files, verification | When working on that task |
| `docs/PRD.md` (this) | Requirements, architecture, phases | Only §X.Y referenced by task |
| `docs/testing-strategy.md` | Testing implementation details | Only §N referenced by task |
| `docs/dootsabha-architecture.html` | Full interfaces, diagrams, sequences | Only when task references it |
| `docs/dootsabha-buildplan.html` | Build philosophy, phase checklists | Only when task references it |

**Per-task agent workflow:** `cm context` → CLAUDE.md → PROGRESS.md → task file → PRD §X.Y (only referenced sections)

---

## Table of Contents

| § | Section | Lines | Typical Task References |
|---|---------|-------|------------------------|
| 1 | [Vision & Philosophy](#1-vision--philosophy) | ~26 | Rarely (scaffolding only) |
| 2 | [Problem Statement](#2-problem-statement) | ~23 | Rarely |
| 3 | [Target Audience](#3-target-audience) | ~24 | Exit code / JSON tasks |
| 4 | [Technology Stack](#4-technology-stack) | ~55 | Provider tasks, scaffolding |
| 5 | [Architecture](#5-architecture) | ~108 | Most tasks (selectively) |
| 6 | [Functional Requirements](#6-functional-requirements) | ~277 | Per-command: §6.1–§6.7 |
| 7 | [Non-Functional Requirements](#7-non-functional-requirements) | ~41 | Retry, perf, security tasks |
| 8 | [Terminal UX Standards](#8-terminal-ux-standards) | ~51 | Any output-visible task |
| 9 | [Implementation Phases](#9-implementation-phases) | ~104 | Planning, dependency checks |
| 10 | [Testing Strategy](#10-testing-strategy) | ~64 | All tasks (summary only) |
| 11 | [Risk Assessment](#11-risk-assessment) | ~17 | Spike tasks |
| 12 | [Open Questions](#12-open-questions) | ~15 | When blocked |
| 13 | [Change Log](#13-change-log) | ~7 | Never |

---

## 1. Vision & Philosophy

### 1.1 One-line Vision

दूतसभा (dūtasabhā — "The Council of Agents") is a plugin-extensible Go CLI that orchestrates AI coding agents (Claude Code, Codex CLI, Gemini CLI) through council-mode deliberation, peer review, and synthesis — producing higher-quality outputs than any single agent alone.

### 1.2 Design Principles

| Principle | Meaning | Implementation |
|-----------|---------|----------------|
| **Small core + plugin shell** | Core handles cross-cutting concerns; features are plugins | Providers, strategies, formatters, hooks are all go-plugin gRPC plugins |
| **Agent-first, human-beautiful** | Machine-consumable output AND gorgeous terminal rendering | `--json` for agents, lipgloss-styled output for humans |
| **Spike before you build** | Every technical assumption validated with throwaway code | Phase 0 exists entirely for spikes — 8 disposable programs |
| **Run it, don't just test it** | Binary execution proof, not just "tests pass" | Every phase requires actual terminal output as evidence |
| **Incremental, shippable layers** | Each phase produces a working binary that does something useful | P1=consult, P2=council, P3=plugins, P4=hardening, P5=ship |
| **Bilingual UX** | English primary, Sanskrit aliases for all commands/flags | `council`/`sabha`, `consult`/`paraamarsh`, `--agent`/`--doota` |

### 1.3 What दूतसभा Is NOT

- **Not a chat interface** — sends prompts, collects responses. No conversational turn-taking.
- **Not an API wrapper** — orchestrates CLI tools (claude, codex, gemini), not APIs directly.
- **Not a model evaluator** — it's a collaboration tool, not a benchmark suite (though `dootsabha-bench` extension could add this).
- **Not a daemon** — runs on demand, produces output, exits. No background process.

---

## 2. Problem Statement

### 2.1 The Single-Agent Limitation

AI coding agents are powerful individually, but each has blind spots, biases, and failure modes. Developers currently:
1. Run one agent at a time and hope for the best
2. Manually copy outputs between agents for cross-checking
3. Have no systematic way to get peer review across agents
4. Cannot synthesize the best parts of multiple agent outputs

### 2.2 What दूतसभा Solves

| Need | Current Workflow | दूतसभा |
|------|-----------------|---------|
| Multi-agent perspective | Copy-paste between terminals | `dootsabha council "question"` |
| Cross-agent review | Manual, tedious, error-prone | `dootsabha review "question"` |
| Synthesized best answer | Read 3 outputs, merge mentally | Council synthesis via chair agent |
| Agent health monitoring | `which claude && claude --version` per agent | `dootsabha status` — all at once |
| Machine-consumable multi-agent output | Doesn't exist | `--json` on all commands |
| Extensible agent tooling | Each tool is siloed | Plugin system + extension discovery |

---

## 3. Target Audience

### 3.1 Primary: Power Users & Agent Builders

- Developers who use multiple AI coding agents daily
- Want systematic multi-perspective answers to hard problems
- Comfortable with terminal tools, appreciate beautiful CLI output

### 3.2 Primary: AI Coding Agents (as Consumers)

- Claude Code, Codex, Gemini CLI using दूतसभा as a tool
- Need structured JSON output via `--json`
- Use exit codes for control flow (0=success, 1=error, 2=usage, 3=provider, 4=timeout, 5=partial)
- Operate in non-TTY environments (subprocess invocation)

### 3.3 Tertiary: Extension Developers

- Developers building on दूतसभा's plugin system
- Write new providers (wrapping additional AI CLIs)
- Write extensions (bench, cost, diff, replay)
- Write hooks (cost guards, audit logging, prompt injection)

---

## 4. Technology Stack

| Component | Choice | Version | Rationale |
|-----------|--------|---------|-----------|
| Language | Go | 1.26+ | Performance, static binary, ecosystem (cobra, go-plugin) |
| CLI framework | cobra | v1.10.2 | Industry standard (kubectl, gh, docker). Aliases for bilingual names. |
| Config | viper | v1.21.0 | YAML + env + flags merge. Pairs with cobra. |
| Plugin system | hashicorp/go-plugin | v1.7.0 | gRPC, process-isolated. Battle-tested (Terraform, Vault). |
| gRPC | google.golang.org/grpc + protobuf | latest | Required by go-plugin. Typed contracts via .proto. |
| Logging | log/slog (stdlib) | — | Zero deps, structured, JSON+text handlers. |
| Terminal styling | lipgloss | v1.1.0 | Colors, borders, padding, tables. Composable. |
| Spinners/progress | charmbracelet/huh | v0.8.0 | Only `huh.NewSpinner()` API used (stderr-only). Not using huh forms/TUI. Pairs with lipgloss. |
| Tables | lipgloss/table | (in lipgloss) | Built-in table rendering. |
| Concurrency | x/sync/errgroup | v0.15.0 | Fan-out/fan-in with error propagation. |
| Retry | avast/retry-go/v4 | v4.7.0 | Typed retry with exponential backoff. Context-aware. |
| Testing | stretchr/testify | latest | Assertions and mocks. |
| Subprocess | os/exec (stdlib) | — | Context-aware via CommandContext. Setpgid for cleanup. |
| Linter | golangci-lint | v2.9.0 | Strict: govet, errcheck, staticcheck, unused, gocritic, errorlint |
| Formatter | gofumpt | latest | Stricter than gofmt |

### 4.1 Verified CLI Tool Versions (2026-02-28)

| CLI | Installed Version | JSON Flag | YOLO Flag |
|-----|-------------------|-----------|-----------|
| claude | 2.1.63 | `--output-format json` | `--dangerously-skip-permissions` |
| codex | 0.106.0 | `--json` (JSONL stream) | `--sandbox danger-full-access` |
| gemini | 0.30.0 | `--output-format json` | `--yolo` or `--approval-mode yolo` |

**Codex JSONL format (verified):**
```
{"type":"thread.started","thread_id":"..."}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"..."}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"PONG"}}
{"type":"turn.completed","usage":{"input_tokens":N,"output_tokens":N}}
```
Final content is in `item.completed` where `item.type == "agent_message"`. Token usage is in `turn.completed`.

**Gemini flags (verified v0.30.0):**
- Both `--yolo` (boolean shorthand) and `--approval-mode yolo` work
- `-p`/`--prompt` flag is available for non-interactive mode (NOT deprecated)
- Positional `gemini "prompt"` also works for non-interactive

**Claude nested session gotcha:** `claude -p` cannot be run inside a Claude Code session (errors with "cannot launch inside another Claude Code session"). Must unset `CLAUDECODE` env var in subprocess.

### 4.2 Charmbracelet Version Pin Strategy

From gh-ghent learnings: charmbracelet dependencies can create version conflicts. Strategy:
- Pin lipgloss to v1.1.0 (stable release)
- Let bubbletea/bubbles/huh resolve via `go mod tidy`
- After any `go get` of charmbracelet packages, run `go mod tidy` and verify no downgrades
- Note: huh is only for spinners/progress in दूतसभा (not full TUI), so simpler than gh-ghent's BubbleTea usage

---

## 5. Architecture

> **Authoritative source:** `docs/dootsabha-architecture.html` — full diagrams, interfaces, sequence flows

### 5.1 Directory Structure

```
dootsabha/
├── cmd/dootsabha/main.go           # Entry point
├── internal/
│   ├── core/
│   │   ├── engine.go               # Session manager, state machine
│   │   ├── config.go               # Viper-based config loader
│   │   ├── subprocess.go           # os/exec wrapper with log capture
│   │   └── retry.go                # Retry logic with classification
│   ├── plugin/
│   │   ├── manager.go              # gRPC plugin discovery, loading, registry
│   │   ├── interfaces.go           # Provider, Strategy, Formatter, Hook interfaces
│   │   ├── grpc.go                 # gRPC server/client for go-plugin
│   │   ├── extension.go            # PATH-based extension discovery & exec
│   │   └── hooks.go                # Hook chain runner
│   ├── cli/
│   │   ├── root.go                 # Cobra root + global flags
│   │   ├── council.go              # council (sabha) subcommand
│   │   ├── consult.go              # consult (paraamarsh) subcommand
│   │   ├── review.go               # review (sameeksha) subcommand
│   │   ├── status.go               # status (sthiti) subcommand
│   │   ├── config_cmd.go           # config (vinyaas) subcommand
│   │   └── plugin_cmd.go           # plugin (vistaarak) subcommand
│   ├── output/
│   │   ├── renderer.go             # RenderContext{TTY, Color, Width, Format}
│   │   ├── json.go                 # JSON formatter
│   │   ├── table.go                # lipgloss table formatter
│   │   └── styles.go               # Provider colors, badges, theme
│   ├── providers/                  # Hardcoded providers (Phase 1-2, extracted in Phase 3)
│   │   ├── claude.go
│   │   ├── codex.go
│   │   └── gemini.go
│   ├── observability/
│   │   ├── logger.go               # slog setup
│   │   ├── metrics.go              # In-process metrics collector
│   │   └── trace.go                # Trace ID generation
│   └── version/
│       └── version.go              # Version, commit, date via ldflags
├── plugins/                        # gRPC plugin binaries (Phase 3+)
│   ├── claude/
│   ├── codex/
│   ├── gemini/
│   └── council-strategy/
├── proto/
│   ├── provider.proto
│   ├── strategy.proto
│   └── hook.proto
├── configs/
│   └── default.yaml                # Embedded default config
├── _spikes/                        # Phase 0 throwaway programs
├── scripts/
│   ├── smoke_test.sh               # L3 smoke tests
│   └── integration_test.sh         # L4 integration tests
├── testdata/                       # Test fixtures
├── Makefile
├── CLAUDE.md
├── .golangci.yml
└── go.mod
```

### 5.2 Core Components

| Component | File | Responsibility |
|-----------|------|----------------|
| **Config Manager** | `internal/core/config.go` | Viper: YAML + env + flags merge. Schema validation. Provider resolution. |
| **Subprocess Runner** | `internal/core/subprocess.go` | os/exec with context, timeout, Setpgid, stdout/stderr splitting, orphan reaper (kill process group after grace period if parent pipe breaks) |
| **Session Manager** | `internal/core/engine.go` | State machine: Init → Dispatch → Review → Synthesis → Output |
| **Retry Logic** | `internal/core/retry.go` | Transient vs permanent classification. Exponential backoff. |
| **Render Context** | `internal/output/renderer.go` | TTY detection, NO_COLOR, terminal width. All output flows through this. |
| **Plugin Manager** | `internal/plugin/manager.go` | go-plugin discovery, handshake, gRPC connections, health checks |

### 5.3 Plugin Types

| Type | Interface | Transport | Discovery | Built-in |
|------|-----------|-----------|-----------|----------|
| **Provider** | `Provider` (Invoke, Cancel, HealthCheck, Capabilities) | gRPC | plugins dir | claude, codex, gemini |
| **Strategy** | `Strategy` (Execute) | gRPC | plugins dir | council, consult |
| **Extension** | none (binary) | exec (stdin/out/err) | $PATH + plugins dir | none |
| **Hook** | `Hook` (PreInvoke, PostInvoke, PreSynthesis, PostSession) | gRPC | plugins dir | none |

### 5.4 Extension Context Protocol (3 Tiers)

| Tier | Mechanism | What | Example |
|------|-----------|------|---------|
| **1** | Env vars (`DOOTSABHA_*`) | Flat scalars: version, paths, session ID, TTY, terminal width | `DOOTSABHA_TTY=true` |
| **2** | Context file (JSON) | Full config, provider registry, workspace, capabilities | `jq '.providers.claude.healthy' $DOOTSABHA_CONTEXT_FILE` |
| **3** | Core callback | Invoke agents, run council, check status | `$DOOTSABHA_BIN consult --agent claude --json "question"` |

### 5.5 Key Design Decisions

| Decision | Rationale | Reference |
|----------|-----------|-----------|
| Hardcoded providers in P1-P2, plugins in P3 | Prove the core works before adding plugin complexity | Build plan P1.5-P1.6 |
| lipgloss not BubbleTea for output | दूतसभा is a command-run-exit tool, not an interactive TUI | Architecture §9 |
| huh for spinners (stderr only) | Need animated progress while agents run, but only on stderr | Architecture §9 |
| Subprocess per agent, not API | Wraps official CLIs — gets auth, model access, updates for free | Architecture §1 |
| JSONL decoder for Codex | Codex outputs event stream, not single JSON object | Spike 0.1, §4.1 |
| Unset CLAUDECODE in subprocess env | Claude Code refuses to run inside another Claude Code session | §4.1 verified gotcha |
| `errgroup` for parallel dispatch | Context cancellation + error propagation. Proven in gh-ghent. | Architecture §9 |

---

## 6. Functional Requirements

### 6.1 Root Command (`dootsabha`)

> **Also read:** §8 (terminal UX), §4 (tech stack for cobra/viper)

**Purpose:** Entry point; shows help.

**Global flags (persistent, inherited by all subcommands):**
- `--json` — Structured JSON output (stdout=data, stderr=logs)
- `--verbose` / `-v` — Increase log verbosity (-v, -vv, -vvv)
- `--quiet` / `-q` — Suppress non-error output
- `--timeout` / `--kaalseema` — Max time per individual agent invocation [default: 5m]
- `--session-timeout` / `--satra-seema` — Max total session time across all stages [default: 30m]
- `--watch` / `--nireekshana` — Stream output in real-time (Phase 4+ — see Q8 in §12)
- `--config` — Config file path [default: ~/.dootsabha/config.yaml]

**Behavior:**
1. With no subcommand: show help text with bilingual names
2. Unknown command: check for `dootsabha-{name}` extension on PATH → show resolved binary path + prompt confirm on first run → exec if trusted → error if not found. Trusted extensions are cached in `~/.dootsabha/trusted-extensions.yaml`.

**Exit codes (highest applicable wins):**
- `0` — Success
- `1` — General error
- `2` — Usage error (bad flags, missing args)
- `3` — Provider error (CLI failed, auth invalid)
- `4` — Timeout (at least one agent timed out)
- `5` — Partial result (some agents failed in council but synthesis produced)

**Precedence:** `2 > 4 > 3 > 5 > 1 > 0` — usage errors trump all (fail fast), timeouts next, then provider failures, then partial results. When multiple codes apply, the highest-precedence one is returned. Example: timeout + partial = exit 4.

**Acceptance criteria:**
- [ ] FR-ROOT-01: `dootsabha help` shows bilingual names (e.g., `council (sabha)`)
- [ ] FR-ROOT-02: `dootsabha --version` prints version string
- [ ] FR-ROOT-03: Unknown command checks for extension binary
- [ ] FR-ROOT-04: `--json` applies globally to all subcommands
- [ ] FR-ROOT-05: `--timeout` applies per-agent invocation; `--session-timeout` caps total session time
- [ ] FR-ROOT-06: Retries consume the same per-agent timeout budget (no reset)
- [ ] FR-ROOT-07: Handles SIGPIPE gracefully when piped to `head` (exit 0, no "broken pipe" error)

### 6.2 Council Command (`dootsabha council` / `sabha`)

> **Also read:** §5.2 (engine state machine), §7.2 (retry/degradation), §8 (UX)

**Purpose:** All configured agents deliberate → peer review → synthesized answer.

**Flags:**
- `--agents` / `--dootas` — Agents to include [default: claude,codex,gemini]
- `--chair` / `--adhyaksha` — Synthesis agent [default: claude]
- `--rounds` / `--chakra` — Deliberation rounds [default: 1]
- `--parallel` / `--samantar` — Run agents concurrently [default: true]
- All global flags

**3-Stage Pipeline (per round):**
1. **Dispatch** — Send prompt to all agents in parallel (or sequential if `--parallel=false`)
2. **Peer Review** — Each agent reviews the other agents' outputs
3. **Synthesis** — Chair agent produces final unified answer from all outputs + reviews

**Multi-round behavior (`--rounds > 1`):**
- Each round feeds the previous round's synthesis back as additional context
- Round N dispatch prompt = original prompt + "Previous synthesis: {round N-1 synthesis}"
- Stop conditions: (a) `--rounds` limit reached, (b) `--session-timeout` exceeded, (c) chair indicates convergence (synthesis matches previous)
- Cost control: each round multiplies token usage ~linearly. Default is 1 round; >3 is not recommended.
- Context cap: per-round context fed to next round is truncated to 32KB (same as peer review cap). For multi-round with many agents, consider structured summaries over raw output to prevent context window blowout.

**Chair failure semantics:**
- If chair fails during synthesis, **re-invoke** the first healthy non-chair agent with synthesis prompt: "You are acting as synthesis chair. Synthesize these responses: {outputs + reviews}"
- If all agents fail, exit code 1 (no synthesis possible)
- Chair fallback is logged as a warning; JSON output includes `"chair_fallback": "codex"` (name of fallback agent)

**Terminal output (TTY):**
```
═══ Stage 1: Dispatch ═══                              3 agents · parallel

● claude  ██████████████████████████████████████████ 3.1s ✓
● codex   ████████████████████████████████████       2.8s ✓
● gemini  ████████████████████████████               2.2s ✓

═══ Stage 2: Peer Review ═══

claude  reviewing codex + gemini ..................... ✓
codex   reviewing claude + gemini .................... ✓
gemini  reviewing claude + codex ..................... ✓

═══ Stage 3: Synthesis ═══                             chair: claude

[synthesized output here]

─────────────────────────────────────────────────────────
total: 8.4s │ cost: $0.042 │ tokens: 3,847 in · 1,203 out
agents: claude ✓ · codex ✓ · gemini ✓
```

**JSON output:**
```json
{
  "dispatch": [
    {"provider": "claude", "model": "sonnet-4-6", "content": "...", "duration_ms": 3100, "cost_usd": 0.003, "tokens_in": 847, "tokens_out": 234},
    {"provider": "codex", "model": "gpt-5.3-codex", "content": "...", "duration_ms": 2800},
    {"provider": "gemini", "model": "gemini-3-pro", "content": "...", "duration_ms": 2200}
  ],
  "reviews": [
    {"reviewer": "claude", "reviewed": ["codex", "gemini"], "content": "..."},
    {"reviewer": "codex", "reviewed": ["claude", "gemini"], "content": "..."},
    {"reviewer": "gemini", "reviewed": ["claude", "codex"], "content": "..."}
  ],
  "synthesis": {
    "chair": "claude",
    "content": "..."
  },
  "meta": {
    "schema_version": 1,
    "session_id": "ds_x7k2m",
    "strategy": "council",
    "duration_ms": 8400,
    "total_cost_usd": 0.042,
    "total_tokens_in": 3847,
    "total_tokens_out": 1203,
    "providers": {"claude": "ok", "codex": "ok", "gemini": "ok"}
  }
}
```

**Graceful degradation:**
- If one agent fails permanently, continue with remaining agents + warn user
- If one agent times out, retry once, then degrade
- Exit code 5 if partial result (some agents failed but synthesis produced)

**Acceptance criteria:**
- [ ] FR-COU-01: Dispatches to all configured agents in parallel
- [ ] FR-COU-02: Shows progress bars (stderr) during dispatch
- [ ] FR-COU-03: Peer review stage — each agent reviews other agents' outputs
- [ ] FR-COU-04: Synthesis — chair agent produces unified answer
- [ ] FR-COU-05: `--json` produces valid JSON with `meta.schema_version` field; cost/token fields are `null` when provider doesn't report them
- [ ] FR-COU-06: Graceful degradation when one agent fails
- [ ] FR-COU-07: `--agents` overrides configured agent list
- [ ] FR-COU-08: `--chair` overrides synthesis agent
- [ ] FR-COU-09: Footer stats: total time, cost, tokens, agent status
- [ ] FR-COU-09a: Piped output (`| cat`) has no ANSI codes, no spinner artifacts
- [ ] FR-COU-10: `--parallel=false` runs agents sequentially
- [ ] FR-COU-11: Max 5 agents in council (O(n²) peer review scaling). Error if exceeded.
- [ ] FR-COU-12: Peer review input truncated to 32KB per agent output to cap context size

### 6.3 Consult Command (`dootsabha consult` / `paraamarsh`)

> **Also read:** §4.1 (CLI versions/flags), §5.2 (subprocess runner), §8 (UX)

**Purpose:** Ask a single agent directly.

**Flags:**
- `--agent` / `--doota` — Agent to consult [default: claude]
- `--model` / `--pratyaya` — Override model for this invocation
- `--max-turns` — Max agentic turns [default: 0 = unlimited]
- All global flags

**Terminal output (TTY):**
```
● claude · sonnet-4-6                                    ⏱ 2.3s · $0.003

A goroutine is a lightweight thread of execution managed by the Go runtime...

─────────────────────────────────────────────────────────
tokens: 847 in · 234 out │ cost: $0.003 │ session: ds_x7k2m
```

**JSON output:**
```json
{
  "provider": "claude",
  "model": "sonnet-4-6",
  "content": "...",
  "duration_ms": 2300,
  "cost_usd": 0.003,
  "tokens_in": 847,
  "tokens_out": 234,
  "session_id": "ds_x7k2m",
  "exit_code": 0
}
```

**Acceptance criteria:**
- [ ] FR-CON-01: Invokes selected agent with prompt
- [ ] FR-CON-02: Shows spinner (stderr) while agent runs
- [ ] FR-CON-03: Styled output with provider color dot, timing, cost
- [ ] FR-CON-04: `--json` produces valid JSON
- [ ] FR-CON-05: `--agent codex` uses Codex CLI with JSONL parsing
- [ ] FR-CON-06: `--agent gemini` uses Gemini CLI
- [ ] FR-CON-07: `--model opus-4-6` overrides provider default model
- [ ] FR-CON-08: `--timeout 30s` kills agent after 30s with structured error
- [ ] FR-CON-09: Piped output has no ANSI codes, no spinner artifacts

### 6.4 Review Command (`dootsabha review` / `sameeksha`)

> **Also read:** §6.3 (consult, since review builds on it), §8 (UX)

**Purpose:** One agent reviews another's output.

**Flags:**
- `--author` / `--kartaa` — Agent that produces initial output [default: codex]
- `--reviewer` / `--pareekshak` — Agent that reviews [default: claude]
- All global flags

**Two-step pipeline:**
1. Invoke author agent with prompt
2. Invoke reviewer agent with: "Review the following output from {author}. Identify strengths, weaknesses, errors. Be specific." + author's output

**Acceptance criteria:**
- [ ] FR-REV-01: Author produces output, reviewer reviews it
- [ ] FR-REV-02: Styled output shows both author output and review
- [ ] FR-REV-03: `--json` includes both `author` and `review` sections
- [ ] FR-REV-04: `--author` and `--reviewer` override defaults
- [ ] FR-REV-05: If author fails, reviewer is not invoked (fail-fast)

### 6.5 Status Command (`dootsabha status` / `sthiti`)

> **Also read:** §5.3 (plugin types), §8.3 (degradation matrix)

**Purpose:** Health check all providers, show versions & auth state.

**Terminal output (TTY):**
```
दूतसभा · status                                              v0.1.0

PROVIDER   MODEL           STATUS    VERSION    LATENCY
● claude   sonnet-4-6      ✓ ready   2.1.63     —
● codex    gpt-5.3-codex   ✓ ready   0.106.0    —
● gemini   gemini-3-pro    ✗ auth    0.30.0     —
                            ↳ OAuth token expired — run: gemini auth login

Plugins: 3 providers · 1 strategy · 0 hooks
Extensions: bench, cost, tui
```

**JSON output:**
```json
{
  "version": "0.1.0",
  "providers": {
    "claude": {"healthy": true, "model": "sonnet-4-6", "cli_version": "2.1.63", "auth_valid": true},
    "codex": {"healthy": true, "model": "gpt-5.3-codex", "cli_version": "0.106.0", "auth_valid": true},
    "gemini": {"healthy": false, "model": "gemini-3-pro", "cli_version": "0.30.0", "auth_valid": false, "error": "OAuth token expired"}
  },
  "plugins": {"providers": 3, "strategies": 1, "hooks": 0},
  "extensions": ["bench", "cost", "tui"]
}
```

**Acceptance criteria:**
- [ ] FR-STA-01: Shows all configured providers with health status
- [ ] FR-STA-02: Provider color dots (● in provider-specific color)
- [ ] FR-STA-03: Actionable error messages for unhealthy providers
- [ ] FR-STA-04: `--json` produces valid JSON with all provider details
- [ ] FR-STA-05: Shows plugin and extension counts
- [ ] FR-STA-06: Piped output: no colors, no Unicode dots, text table

### 6.6 Config Command (`dootsabha config` / `vinyaas`)

> **Also read:** §5.2 (config manager), §7.4 (security/redaction)

**Purpose:** View resolved configuration.

**Subcommands:**
- `config show` — Dump resolved config (merged: defaults + file + env + flags)
- `config show --commented` — Annotated config with explanations

**Acceptance criteria:**
- [ ] FR-CFG-01: Shows effective merged configuration
- [ ] FR-CFG-02: `--json` outputs config as JSON
- [ ] FR-CFG-03: `--commented` includes inline documentation
- [ ] FR-CFG-04: Config precedence: defaults < file < env (`DOOTSABHA_*`) < flags. Override chain testable via `config show --json`.
- [ ] FR-CFG-05: Unknown config keys are silently ignored (forward-compatible)
- [ ] FR-CFG-06: Keys matching `*token*`, `*key*`, `*secret*` are redacted in `config show` output unless `--reveal` flag is passed

### 6.7 Plugin Command (`dootsabha plugin` / `vistaarak`)

> **Also read:** §5.3 (plugin types), §5.4 (extension context protocol)

**Purpose:** List and inspect plugins & extensions.

**Subcommands:**
- `plugin list` / `vistaarak soochi` — All plugins + extensions with health
- `plugin inspect {name}` / `vistaarak parikshan` — Detailed plugin info

**Acceptance criteria:**
- [ ] FR-PLG-01: Lists gRPC plugins and PATH extensions
- [ ] FR-PLG-02: Shows health status per plugin
- [ ] FR-PLG-03: `inspect` shows capabilities, models, interface version
- [ ] FR-PLG-04: `--json` for machine consumption

---

## 7. Non-Functional Requirements

### 7.1 Performance

| Metric | Target | Rationale |
|--------|--------|-----------|
| Binary startup to first output | < 200ms | CLI tool, not a daemon |
| Single agent invocation overhead | < 100ms | Subprocess setup + JSON parsing, not the agent itself |
| Plugin handshake (gRPC) | < 100ms | go-plugin handshake measured in Spike 0.4 |
| Parallel dispatch overhead vs sequential | < 50ms | errgroup setup cost |
| Memory usage | < 50MB | CLI tool collecting text responses |

### 7.2 Reliability

- **Transient failures** → retry 2x with exponential backoff + jitter (1s±0.5s, 4s±1s), max elapsed = per-agent `--timeout`
  - Matchers: exit code 1 + stderr contains "rate limit"/"429"/"timeout"/"EAGAIN"/"connection refused"
  - Matchers: exit code 137 (OOM killed)
- **Permanent failures** → fail fast with actionable error, never retry
  - Matchers: exit code 127 (CLI not found), exit code 2 (usage error)
  - Matchers: stderr contains "auth"/"token expired"/"permission denied"/"model not found"
  - Default: unknown exit codes are treated as permanent (fail-safe)
- **Partial results** in council → continue with remaining agents, exit code 5
- **Plugin crash** → core recovers gracefully (process isolation via go-plugin)
- **Ctrl+C** → clean shutdown: SIGTERM to process groups, 5s grace period, SIGKILL if still alive, print summary, non-zero exit. Reaper goroutine ensures no orphaned agent processes survive.

### 7.3 Compatibility

- Go 1.26+ (use latest idioms: range-over-func, enhanced loop vars. Run `go fix` on toolchain upgrades only.)
- macOS (darwin/arm64, darwin/amd64) — primary development platform
- Linux (linux/amd64, linux/arm64) — CI and server use
- Requires: `claude`, `codex`, and/or `gemini` CLIs installed (graceful degradation if missing)

### 7.4 Security

- No credential storage — inherits auth from underlying CLIs
- No network calls except via subprocess (claude/codex/gemini do the calling)
- Unset `CLAUDECODE` env var when spawning claude subprocess (prevents nested session error)
- Config file permissions: warn if world-readable (may contain preferences)

---

## 8. Terminal UX Standards

> This is a core quality bar, not optional polish. Every output must be beautiful in a terminal and correct when piped.

### 8.1 Good Unix Citizenship

| Principle | Implementation | Validation |
|-----------|---------------|------------|
| **stdout = data, stderr = logs** | All JSON/results on stdout. Spinners, progress, warnings on stderr. | `dootsabha consult --json "test" 2>/dev/null \| jq .` must be valid JSON |
| **Meaningful exit codes** | 0=success, 1=error, 2=usage, 3=provider, 4=timeout, 5=partial | Test each path explicitly |
| **NO_COLOR + pipe detection** | Respect `NO_COLOR` env. Auto-detect TTY via `os.Stdout.Fd()`. No color/spinner when piped. | `dootsabha status \| cat \| grep -P '\\x1b\\['` finds nothing |
| **Ctrl+C graceful shutdown** | Catch SIGINT/SIGTERM. Kill child process groups. Print summary. | Hit Ctrl+C mid-council: clean message, no stack trace |
| **Width-aware formatting** | Detect terminal width. Tables respect it. Degrade at <60 cols. | 40-col terminal: output doesn't wrap hideously |

### 8.2 Color Palette (lipgloss)

```go
var (
    ClaudeColor  = lipgloss.Color("#F59E0B")  // Amber/gold
    CodexColor   = lipgloss.Color("#10B981")  // Emerald
    GeminiColor  = lipgloss.Color("#3B82F6")  // Blue
    ErrorColor   = lipgloss.Color("#EF4444")  // Red
    SuccessColor = lipgloss.Color("#10B981")  // Green
    WarnColor    = lipgloss.Color("#F59E0B")  // Amber
    MutedColor   = lipgloss.Color("#64748B")  // Slate
    AccentColor  = lipgloss.Color("#06B6D4")  // Cyan
)
```

### 8.3 Graceful Degradation Matrix

| Context | TTY + Color | TTY + NO_COLOR | Piped (not TTY) | --json |
|---------|------------|----------------|-----------------|--------|
| Provider indicator | `●` colored | `*` | `*` | `"provider":"claude"` |
| Status healthy | `✓ ready` green | `OK ready` | `OK ready` | `"healthy":true` |
| Status unhealthy | `✗ auth` red | `FAIL auth` | `FAIL auth` | `"healthy":false` |
| Progress | Animated spinner | Static dots | Nothing (silence) | Nothing |
| Tables | lipgloss borders | lipgloss, no color | Tab-separated | JSON array |
| Footer stats | Styled separator | Plain separator | Omitted | JSON `"meta":{}` |

### 8.4 Lipgloss Pitfalls (from gh-ghent learnings)

These are verified gotchas from cm memory and gh-ghent CLAUDE.md:

1. **Background bleed:** Use `termenv.SetBackgroundColor()` before rendering, `output.Reset()` after. `lipgloss.Background()` only affects rendered chars, empty cells bleed terminal default.
2. **Width calculation:** AVOID `lipgloss.Width()` on inner modal elements — causes padding bleed when composited. Use `strings.Repeat(" ", delta)` for manual padding.
3. **Switch shadowing:** Use `typedMsg := msg.(type)` in type switches, NOT `msg := msg.(type)`. The latter creates a local variable; modifications don't propagate.
4. **ANSI reset:** Use explicit `\033[0m` resets between styled elements to prevent color bleed.

---

## 9. Implementation Phases

> **Strategy: Spike-first, incremental, shippable.** Each phase produces a working binary.
> Providers are hardcoded in P1-P2, extracted to plugins in P3.

### Phase 0: Spikes & Assumption Validation (~1.5 days)

**Goal:** Validate every technical assumption before writing production code.

| Task | Description | Risk Being Validated |
|------|-------------|---------------------|
| 0.1 | Codex JSONL parsing spike | Reliably extract content from JSONL event stream |
| 0.2 | Claude JSON output spike | Verify exact JSON schema, error cases, cost field |
| 0.3 | Gemini JSON output spike | Verify `--yolo` behavior, JSON schema, positional prompt |
| 0.4 | go-plugin gRPC handshake spike | Minimal host+plugin pair, measure overhead |
| 0.5 | Subprocess management spike | errgroup, context cancellation, process group cleanup |
| 0.6 | Cobra alias behavior spike | Bilingual names, flag aliases, unknown cmd handler |
| 0.7 | Terminal UX foundations spike | lipgloss under pipe/NO_COLOR/narrow, spinner patterns |
| 0.8 | PTY vs pipe subprocess spike | Verify CLI behavior (claude/codex/gemini) with plain pipes vs PTY. YOLO flags should prevent interactive prompts, but confirm JSON output is identical in pipe mode. If not, evaluate `creack/pty`. |

**Output:** `_spikes/` directory with 8 throwaway programs + README.md documenting findings.
**Gate:** All assumptions validated or architecture redesigned.

### Phase 1: Single Agent, Beautiful Output (~2.5 days)

**Goal:** `dootsabha consult "question"` works against real CLIs with beautiful terminal output.

| Task | Description | Depends On | Parallel With |
|------|------------|------------|---------------|
| 1.1 | Project scaffold + Makefile (full set §10.2) + CLAUDE.md + gating hooks (§10.5) | — | — |
| 1.2 | Render context & output foundation (renderer.go, styles) | 1.1 | 1.3 |
| 1.3 | Config manager (viper, YAML + env + flags) | 1.1 | 1.2 |
| 1.4 | Subprocess runner (os/exec, context, Setpgid) | 1.1 | 1.2, 1.3 |
| 1.5 | Claude provider (hardcoded, not plugin yet) | 1.2, 1.3, 1.4 | — |
| 1.6 | Codex + Gemini providers (hardcoded) | 1.5 | — |
| 1.7 | Cobra CLI wiring (root, consult, status, config) | 1.5, 1.6 | — |

**Gate:** `dootsabha consult "PONG"` returns PONG from all 3 CLIs. `dootsabha status` shows real health data. All outputs beautiful in TTY, clean when piped, valid when `--json`.
**PRD sections needed:** §4, §5, §6.1, §6.3, §6.5, §8

### Phase 2: Council Pipeline & Review Mode (~2.5 days)

**Goal:** Full 3-stage council pipeline. Review mode. Retry logic.

| Task | Description | Depends On | Parallel With |
|------|------------|------------|---------------|
| 2.1 | Parallel dispatch (errgroup, progress rendering) | Phase 1 | — |
| 2.2 | Peer review stage (cross-review prompts) | 2.1 | — |
| 2.3 | Synthesis stage (chair agent, final output) | 2.2 | — |
| 2.4 | Review subcommand (author + reviewer) | Phase 1 | 2.1 |
| 2.5 | Retry logic + error classification | 2.1 | 2.4 |
| 2.6 | JSON output for all modes (council, review, meta) | 2.3, 2.4 | — |

**Gate:** Full council run with 3 agents produces synthesized answer. Review mode works. Graceful degradation when one agent removed. Ctrl+C clean shutdown.
**PRD sections needed:** §6.2, §6.4, §7.2

### Phase 3: Plugin Architecture (~3 days)

**Goal:** Extract hardcoded providers into go-plugin gRPC plugins. Extension discovery.

| Task | Description | Depends On | Parallel With |
|------|------------|------------|---------------|
| 3.1 | Proto definitions + code generation | Phase 2 | — |
| 3.2 | Plugin manager (discovery, loading, registry) | 3.1 | — |
| 3.3 | Extract provider plugins (claude, codex, gemini) | 3.2 | — |
| 3.4 | Council strategy plugin | 3.3 | — |
| 3.5 | Extension discovery ($PATH + plugins dir) | 3.2 | 3.3, 3.4 |
| 3.6 | Plugin subcommand (vistaarak list/inspect) | 3.5 | — |

**Gate:** Zero regression from plugin extraction. `dootsabha vistaarak list` shows plugins + extensions. Custom extension works.
**PRD sections needed:** §5.3, §5.4, §6.7

### Phase 4: Hardening & Observability (~2 days)

**Goal:** Production-ready error handling, logging, metrics, edge cases.

| Task | Description | Depends On | Parallel With |
|------|------------|------------|---------------|
| 4.1 | Structured logging (slog, JSON/text, levels) | Phase 3 | 4.2 |
| 4.2 | Metrics collection (in-process counters) | Phase 3 | 4.1 |
| 4.3 | Edge cases & error paths (all error scenarios) | 4.1 | 4.4 |
| 4.4 | Tier 2 context file for extensions | 4.1 | 4.3 |
| 4.5 | Full L5 acceptance suite | 4.3, 4.4 | — |

**Gate:** Full L5 acceptance pass. Every error path produces helpful, styled message.
**PRD sections needed:** §7, §8

### Phase 5: Documentation, SKILL & Ship (~2 days)

**Goal:** README, SKILL, build, release, final acceptance.

| Task | Description | Depends On | Parallel With |
|------|------------|------------|---------------|
| 5.1 | README (hero, quick start, screenshots) | Phase 4 | 5.2 |
| 5.2 | Default config + embedded docs | Phase 4 | 5.1 |
| 5.3 | Claude Code SKILL for दूतसभा | 5.1, 5.2 | 5.4 |
| 5.4 | Build & release (goreleaser, CI, v0.1.0) | 5.1, 5.2 | 5.3 |
| 5.5 | Final acceptance (clean install, L5, SKILL test) | 5.3, 5.4 | — |

**Gate:** Clean install works. README quick start copy-pasteable. SKILL enables agent discovery.
**PRD sections needed:** §6.6, build plan P5

---

## 10. Testing Strategy

> **Cardinal Rule:** Every feature MUST be verified by _actually running_ the binary and visually inspecting output via iTerm2-driver screenshots. Unit tests are necessary but NOT sufficient.
>
> **Full reference:** `docs/testing-strategy.md` — all implementation details, templates, scripts, hooks.
> Task files reference specific sections as `testing-strategy.md §N`.

### 10.1 Five-Layer Testing Pyramid

| Layer | What | Speed | Make Target | Mocks? | Costs $? |
|-------|------|-------|-------------|--------|----------|
| **L1** | Compile + lint + `go vet` + `gofumpt` | <5s | `make ci-fast` | N/A | No |
| **L2** | Unit tests (`go test -race -shuffle=on`) | <2s | `make test` | Yes | No |
| **L3** | Binary smoke (mock providers + exit codes) | <10s | `make test-binary` | Mock CLIs | No |
| **L4** | Integration + visual (real CLIs + iTerm2-driver) | 30-60s | `make test-visual` | No | ~$0.05 |
| **L5** | Agent workflow (JSON, exit codes, no ANSI, perf) | 2-5min | `make test-agent` | No | ~$0.50 |

### 10.2 Make Targets (Full Set)

| Category | Targets |
|----------|---------|
| **Build** | `build`, `install`, `clean` |
| **Test** | `test`, `test-race`, `coverage`, `test-integration`, `test-binary` (L3), `test-visual` (L4), `test-agent` (L5), `test-all` |
| **Lint** | `lint`, `lint-fix`, `fmt`, `vet` |
| **Deps** | `tidy`, `verify` |
| **CI** | `ci` (lint+test+vet), `ci-fast` (fmt+vet+test), `check` (fmt+lint+vet+test+smoke) |
| **Tools** | `tools`, `hooks`, `version`, `help` |

Full Makefile with build flags, gotestsum, colored output — implemented during task 1.1. See gh-ghent `Makefile` for reference template.

### 10.3 Key Testing Infrastructure

| Component | Location | Reference |
|-----------|----------|-----------|
| Mock providers (claude, codex, gemini) | `testdata/mock-providers/` | `testing-strategy.md §1` |
| iTerm2-driver canonical template | `.claude/automations/` | `testing-strategy.md §2` |
| L4 gating hooks (pre-task-done, pre-push) | `scripts/hooks/` | `testing-strategy.md §3` |
| L5 agent workflow tests | `scripts/test-agent-workflow.sh` | `testing-strategy.md §4` |
| Task verification checklist | (in task files) | `testing-strategy.md §6` |

### 10.4 Anti-Hallucination Rules (Summary)

> Full list: `testing-strategy.md §5` (10 rules). Top 5:

1. **NEVER claim DONE without actual terminal output as proof.** Screenshots, not assertions.
2. **`make ci` MUST pass before any task is marked DONE.**
3. **Every task file MUST have `## Visual Test Results`** with screenshot reviews and findings.
4. **Pre-push hook blocks** if IN PROGRESS tasks fail L4 gate.
5. **Mock providers for L2/L3 only.** L4+ uses real CLIs with tiny prompts ("PONG").

### 10.5 Session Protocol

> Full protocol: `testing-strategy.md §7` (12 steps). Summary:

```
Read CLAUDE.md → Read task → Mark IN PROGRESS → Read PRD §X.Y →
Execute → Verify (L1→L5) → Fill Visual Test Results →
Mark DONE → Update PROGRESS.md → Commit
```

**Hard rules:** L4 cannot be skipped. Visual Test Results cannot be empty. `cm context` runs at session start.

---

## 11. Risk Assessment

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Codex JSONL format changes | HIGH | MEDIUM | Spike 0.1 captures exact format. Version-pinned parsing. L4 integration tests. |
| go-plugin gRPC overhead too high | MEDIUM | LOW | Spike 0.4 measures. If >200ms, use in-process providers with plugin opt-in. |
| Claude/Gemini JSON schema undocumented | HIGH | MEDIUM | Spikes 0.2/0.3 capture schemas. Lenient parsing (ignore unknown fields). |
| Council synthesis quality is poor | MEDIUM | MEDIUM | Prompt engineering in P2. Iterate. Configurable via strategy plugin. |
| Claude nested session error | HIGH | HIGH | Unset `CLAUDECODE` env var in subprocess. Validated in Spike 0.2. |
| charmbracelet version conflicts | MEDIUM | MEDIUM | Pin lipgloss v1.1.0. Let `go mod tidy` resolve. From gh-ghent: always re-verify. |
| Token cost during development | LOW | HIGH | Mock providers for L2/L3. Tiny prompts ("PONG") for L4. L5 runs sparingly. |
| macOS SIP + process group mgmt | MEDIUM | LOW | Spike 0.5 validates on macOS specifically. |
| CLIs need PTY, not pipe | MEDIUM | MEDIUM | Spike 0.8 verifies YOLO+JSON flags work via plain pipes. If not, add `creack/pty`. |
| Orphaned agent processes on crash | HIGH | MEDIUM | Reaper goroutine + process group kill with grace period. Spike 0.5 validates. |

---

## 12. Open Questions

| # | Question | Status | Decision |
|---|----------|--------|----------|
| Q1 | Should `dootsabha` without subcommand show summary or help? | Open | Currently: help. Could default to status. |
| Q2 | Prompt from stdin vs positional arg? | Resolved | Positional arg primary. If no arg AND stdin is a pipe, read stdin. `--prompt-file` for file input. Precedence: `--prompt-file > arg > stdin`. |
| Q3 | Should providers be hardcoded in MVP, plugins deferred to v0.2? | Open | Build plan has both in MVP (P1-P2 hardcoded → P3 plugins). Could ship P1-P2 as v0.1, plugins as v0.2. |
| Q4 | Should we vendor proto-generated code? | Open | Vendoring avoids protoc dependency for contributors. But adds git bloat. |
| Q5 | BubbleTea TUI extension (`dootsabha-tui`) — scope it for MVP? | Open | Build plan mentions it as future extension. Could be v0.2. |
| Q6 | Gemini `-p` vs positional prompt — which is more reliable? | Resolved | Both work in v0.30.0. Positional is simpler. Use positional, fallback to `-p`. |
| Q7 | Gemini `--yolo` vs `--approval-mode yolo`? | Resolved | Both work in v0.30.0. Use `--yolo` (simpler). Keep `--approval-mode yolo` as fallback. |
| Q8 | `--watch` streaming — what does it look like? | Open | Deferred to Phase 4. Needs spec for TTY stream events, non-TTY line-buffered format, and `--json` NDJSON stream. |

---

## 13. Change Log

| Date | Version | Change |
|------|---------|--------|
| 2026-02-28 | 1.0 | Initial PRD — synthesized from architecture + build plan docs, verified against installed CLI versions |
| 2026-02-28 | 1.1 | Codex review: exit-code precedence matrix, timeout model (agent + session), round state machine, chair fallback, prompt input contract frozen, PATH extension trust, JSON schema_version, retry classifiers, peer review caps, universal pipe checks, config precedence FRs, go fix scope corrected, --watch deferred |
| 2026-02-28 | 1.2 | Gemini review: PTY vs pipe spike added, subprocess reaper pattern, chair fallback re-invocation fix, Provider.Cancel method, config key redaction, SIGPIPE handling, multi-round context cap, orphaned process risk |
| 2026-02-28 | 1.3 | Testing overhaul: extracted testing-strategy.md (447 lines), PRD §10 is now a compact summary (64 lines) |
| 2026-02-28 | 1.4 | Progressive disclosure structure: document hierarchy section, "Also read" cross-refs on all §6 commands, annotated ToC with line counts and typical references |
