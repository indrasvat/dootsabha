<p align="center">
  <img src="assets/logo.png" alt="दूतसभा" width="700">
</p>

# दूतसभा (dootsabha)

**The Council of AI Messengers** — a plugin-extensible Go CLI that orchestrates AI coding agents through council-mode deliberation, peer review, and synthesis.

```
dootsabha council "What's the best way to implement a rate limiter in Go?"
```

Three agents think independently. They review each other's work. A chair synthesizes the best answer. You get one output that's better than any single agent alone.

---

## Why

AI coding agents are powerful individually, but each has blind spots. Today you:

1. Run one agent and hope for the best
2. Manually copy-paste between agents for cross-checking
3. Mentally merge three different answers

दूतसभा automates this. One command, three perspectives, one synthesized answer.

| Need | Before | After |
|------|--------|-------|
| Multi-agent perspective | Copy-paste between terminals | `dootsabha council "question"` |
| Cross-agent review | Manual, tedious | `dootsabha review "question"` |
| Iterative refinement | Read 3 outputs, merge mentally | `dootsabha refine "question"` |
| Agent health check | `which claude && claude --version` | `dootsabha status` |
| Machine-consumable output | Doesn't exist | `--json` on all commands |

---

## Prerequisites

You need at least one of these AI CLI tools installed:

| Agent | Install |
|-------|---------|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | `npm install -g @anthropic-ai/claude-code` |
| [Codex CLI](https://github.com/openai/codex) | `npm install -g @openai/codex` |
| [Antigravity CLI (agy)](https://github.com/google/antigravity) | Install per Google's instructions (`agy`) |
| [Grok CLI](https://docs.x.ai) | xAI Grok Build TUI (`grok`) — optional, opt-in |

> **Note:** `agy` is Google's Go-built [Antigravity CLI](https://github.com/google/antigravity), the official successor to the retired Gemini CLI (Google sunset the Gemini CLI on 2026-06-18). The dootsabha provider name and binary are both `agy`.

> **Note:** `grok` is xAI's Grok CLI. It is **opt-in** — it is not part of any
> default council. Select it explicitly with `--agent grok`, `--agents claude,codex,grok`,
> or `--chair grok`. dootsabha runs it read-only and with an isolated `$HOME`, so it
> does not inherit your Claude Code skills, hooks, or MCP servers.

Verify they're on your PATH:

```bash
claude --version
codex --version
agy --version
grok --version   # optional
```

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/indrasvat/dootsabha/main/install.sh | sh
```

Detects your OS/arch, downloads the latest release, verifies the checksum, and installs to a directory on your `$PATH`. The installer resolves the latest release through GitHub's public web redirect first, so normal installs do not spend anonymous GitHub API quota. It also installs the [agent skill](#skill) with `npx skills add indrasvat/dootsabha --global --yes` when `npx` is available.

<details>
<summary>More options</summary>

```bash
# Non-interactive with defaults (CI/scripts)
curl -fsSL https://raw.githubusercontent.com/indrasvat/dootsabha/main/install.sh | NONINTERACTIVE=1 sh

# Specific version or custom directory
curl -fsSL https://raw.githubusercontent.com/indrasvat/dootsabha/main/install.sh | VERSION=v0.1.0 INSTALL_DIR=~/.local/bin sh

# Skip the agent skill install
curl -fsSL https://raw.githubusercontent.com/indrasvat/dootsabha/main/install.sh | INSTALL_SKILL=0 sh

# If your network blocks GitHub redirects, pin a version to skip latest lookup
curl -fsSL https://raw.githubusercontent.com/indrasvat/dootsabha/main/install.sh | VERSION=v0.4.2 sh

# From source
git clone https://github.com/indrasvat/dootsabha.git
cd dootsabha && make build
cp bin/dootsabha ~/.local/bin/
```

</details>

## Quick Start

```bash
# Check which agents are available
dootsabha status

# Ask a single agent
dootsabha consult "What is a goroutine?" --agent claude

# Run a full council — 3 agents deliberate
dootsabha council "What's the best error handling pattern in Go?"

# One agent writes, another reviews
dootsabha review "Write a retry function with exponential backoff"

# Sequential review + incorporation
dootsabha refine "Implement a concurrent-safe LRU cache"

# JSON output for scripting/agents
dootsabha council "question" --json | jq -r '.synthesis.content'
```

---

## Commands

### `consult` — Query a single agent

```bash
dootsabha consult "What is a goroutine?" --agent claude
dootsabha consult "Explain channels" --agent codex --model gpt-5.5
dootsabha consult "What is a mutex?" --agent agy --json
```

Aliases: `paraamarsh`, `परामर्श`

### `council` — Multi-agent deliberation

Three-stage pipeline: **dispatch** (all agents answer) → **peer review** (each reviews the others) → **synthesis** (chair produces final answer).

```bash
# Default: all 3 agents (or codex,agy when running inside Claude Code)
dootsabha council "What's the best way to handle errors in Go?"

# Pick agents and chair
dootsabha council "question" --agents claude,codex --chair codex

# Sequential dispatch (useful for rate-limited APIs)
dootsabha council "question" --parallel=false

# Multi-round deliberation
dootsabha council "question" --rounds 2
```

Aliases: `sabha`, `सभा`

### `review` — Author + reviewer pipeline

One agent produces output, another reviews it.

```bash
dootsabha review "Write a retry function" --author codex --reviewer claude
dootsabha review "Implement a worker pool" --json
```

Aliases: `sameeksha`, `समीक्षा`

### `refine` — Sequential review + incorporation

Author generates content → reviewers review sequentially → author incorporates feedback.

```bash
dootsabha refine "Implement a concurrent-safe LRU cache"
dootsabha refine "question" --author claude --reviewers codex,agy
dootsabha refine "question" --anonymous=false  # reveal author identity to reviewers
```

Aliases: `sanshodhan`, `संशोधन`

### `status` — Agent health check

```bash
dootsabha status         # TTY: colored health table
dootsabha status --json  # JSON: machine-consumable
```

Aliases: `sthiti`, `स्थिति`

### `config` — Configuration management

```bash
dootsabha config show              # Current merged config (sensitive values redacted)
dootsabha config show --commented  # With inline documentation
dootsabha config show --json       # JSON output
dootsabha config show --reveal     # Show sensitive values

dootsabha config migrate           # Migrate a stale `gemini` block to `agy`
dootsabha config migrate --dry-run # Preview changes without writing
dootsabha config migrate --json    # JSON output
```

`config show` includes a `config_source` entry so you can tell whether the
effective configuration came from the built-in defaults, an auto-loaded default
file, or an explicit `--config` path.

`config migrate` rewrites a stale `gemini:` provider block in your config to the
`agy` provider, writing a `<config>.bak` backup first. dootsabha also prints a
one-line stderr nudge on a TTY when it detects a leftover `gemini` reference in
your config.

Aliases: `vinyaas`, `विन्यास`

### `plugin` — Plugin & extension management

```bash
dootsabha plugin list                # All plugins + PATH extensions
dootsabha plugin inspect claude      # Detailed plugin info
dootsabha plugin list --json         # JSON output
```

Aliases: `vistaarak`, `विस्तारक`

---

## Configuration

दूतसभा works with zero configuration — sensible defaults are built in. On
startup, it auto-loads `~/.config/dootsabha/config.yaml` when that file exists.
If the file does not exist, it falls back to the built-in defaults. Passing
`--config <path>` uses that explicit file instead of the default user config.

To customize the default user config, create `~/.config/dootsabha/config.yaml`:

```yaml
providers:
  claude:
    binary: claude
    model: claude-opus-4-8
    flags:
      - --dangerously-skip-permissions
      - --no-session-persistence
  codex:
    binary: codex
    model: gpt-5.5
    flags:
      - --sandbox
      - danger-full-access
      - --ephemeral
      - --skip-git-repo-check
      - -c
      - model_reasoning_effort=medium
  agy:
    binary: agy
    model: Gemini 3.7 Flash (High)
    flags:
      - --dangerously-skip-permissions
  grok:                             # opt-in; grok-4.5 stays pinnable here
    binary: grok
    model: grok-4.6
    flags:
      - --reasoning-effort          # xhigh | high | medium | low
      - high

council:
  chair: claude       # Agent that synthesizes final output (fallback: first healthy non-chair)
  parallel: true      # Run dispatch phase in parallel
  rounds: 1           # Number of deliberation rounds (max 5)

timeout: 5m           # Budget for ONE agent call — each call gets its own window
session_timeout: 30m  # Ceiling for a whole pipeline (0 = unbounded)
```

### Two timeouts, two jobs

`timeout` is per invocation; `session_timeout` bounds the whole pipeline.
`review --timeout 8m --session-timeout 20m` gives the author up to 8m *and* the
reviewer up to 8m, while the command as a whole cannot exceed 20m. Whichever
budget runs out first is named in the error, so you know which one to raise:

```
Error: invocation timeout after 8m0s: codex invoke: …   # raise --timeout
Error: session timeout after 20m0s: claude invoke: …    # raise --session-timeout
```

The session ceiling is checked between calls, and an agent CLI that ignores
SIGTERM gets a 5s grace period before it is killed — so a run can finish a few
seconds past the ceiling. That overshoot is bounded by the one grace period; it
does not grow with the length of the pipeline.

### Config merge order

**built-in defaults → auto-loaded `~/.config/dootsabha/config.yaml` or explicit `--config` file → env vars → CLI flags**

Check the active source at any time:

```bash
dootsabha config show --json | jq '.data.config_source'
```

Environment variables use `DOOTSABHA_` prefix with `_` separators:

```bash
export DOOTSABHA_PROVIDERS_CLAUDE_MODEL=claude-opus-4-8
export DOOTSABHA_PROVIDERS_AGY_BINARY=agy
export DOOTSABHA_PROVIDERS_GROK_BINARY=grok
export DOOTSABHA_COUNCIL_CHAIR=codex
export DOOTSABHA_TIMEOUT=10m
```

---

## Output Modes

All commands support three output modes:

| Mode | When | Behavior |
|------|------|----------|
| **TTY + color** | Interactive terminal | Styled with lipgloss, rounded boxes, colored provider dots |
| **TTY + NO_COLOR** | `NO_COLOR=1` set | Structured layout without ANSI escape codes |
| **Piped** | `cmd \| jq`, non-TTY | Plain text, no styling, no box drawing characters |

```bash
# Force JSON for scripting
dootsabha council "question" --json

# Pipe-friendly (auto-detected)
dootsabha council "question" | cat

# Suppress progress output
dootsabha council "question" --quiet
```

> **Note on token/cost data:** all four agents report token counts in `--json`
> output. `claude` and `grok` also report cost and session IDs; `agy` reports a
> session ID (its conversation id) but **no cost** — the Antigravity CLI does not
> emit one, so `cost_usd` is `0` rather than estimated. `codex` reports neither.

---

## Exit Codes

| Code | Meaning | Example |
|------|---------|---------|
| 0 | Success | All agents responded |
| 1 | Unexpected internal error | A bug — please report |
| 2 | Bad command | Unknown flag, missing argument, unknown agent or chair |
| 6 | Config error | Config file missing, unreadable, or invalid |
| 3 | Provider error | Agent CLI not found or crashed |
| 4 | Timeout | Agent exceeded deadline |
| 5 | Partial result | Some agents failed, result still useful |



---

## Extensions

दूतसभा discovers any binary on `$PATH` named `dootsabha-{name}` and makes it available as a subcommand:

```bash
# If dootsabha-bench exists on PATH:
dootsabha bench --runs 5 "question"
```

### Writing an extension

Create an executable named `dootsabha-{name}`:

```bash
#!/bin/bash
# dootsabha-hello — A simple extension
echo "Hello from dootsabha extension!"
echo "Args: $@"
```

```bash
chmod +x dootsabha-hello
# Place on PATH
dootsabha hello world  # "Hello from dootsabha extension!" / "Args: world"
```

### Context tiers

Extensions receive context through environment variables:

| Tier | Mechanism | Content |
|------|-----------|---------|
| **Tier 1** | Environment variables | `DOOTSABHA_VERSION`, `DOOTSABHA_SESSION_ID`, `DOOTSABHA_WORKSPACE` |
| **Tier 2** | Context file | `DOOTSABHA_CONTEXT_FILE` → JSON with session, providers, capabilities, TTY info |

Read the context file for rich session data:

```bash
#!/bin/bash
# dootsabha-info — Show context
if [ -n "$DOOTSABHA_CONTEXT_FILE" ]; then
    echo "Context:"
    cat "$DOOTSABHA_CONTEXT_FILE" | python3 -m json.tool
else
    echo "No context file available"
fi
```

### Listing extensions

```bash
dootsabha plugin list  # Shows both gRPC plugins and PATH extensions
```

---

## Claude Code Integration

When running inside a Claude Code session, दूतसभा automatically:
- **Defaults council agents to `codex,agy`** — Claude is already the host, no need to call it again
- **Preserves all `CLAUDE_CODE_*` env vars** — Bedrock, Vertex, and Foundry routing works seamlessly
- You can still explicitly add Claude with `--agents claude,codex,agy` if needed

### Skill

दूतसभा ships with a [Claude Code skill](https://code.claude.com/docs/en/skills) in `skill/SKILL.md` that teaches AI agents how to use all commands, parse JSON output, and handle exit codes. The installer adds the global skill automatically when `npx` is available:

```bash
npx skills add indrasvat/dootsabha --global --yes
```

Agents also discover the checked-in skill when working directly in this repo.

The skill triggers when you ask for things like:
- "get a second opinion from another LLM"
- "do a final review with codex/agy"
- "run it by another model"
- "validate and review this PRD"

See `skill/` for the full skill with command reference, exit code patterns, and workflow examples.

---

## Plugin System

दूतसभा uses [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) for gRPC-based plugins. Three plugin types:

| Type | Interface | Purpose |
|------|-----------|---------|
| **Provider** | `Invoke`, `HealthCheck`, `Capabilities` | Wrap a new AI CLI |
| **Strategy** | `Execute` | Custom deliberation pipeline |
| **Hook** | `HandleEvent` | Pre/post processing (cost guard, PII redaction) |

Plugin binaries are discovered in `~/.config/dootsabha/plugins/` and the built-in `plugins/` directory.

---

## Bilingual Interface

Every command and flag has a Sanskrit/Hindi alias:

| English | Transliteration | Devanagari |
|---------|----------------|------------|
| `council` | `sabha` | `सभा` |
| `consult` | `paraamarsh` | `परामर्श` |
| `review` | `sameeksha` | `समीक्षा` |
| `refine` | `sanshodhan` | `संशोधन` |
| `status` | `sthiti` | `स्थिति` |
| `config` | `vinyaas` | `विन्यास` |
| `plugin` | `vistaarak` | `विस्तारक` |

```bash
# These are equivalent:
dootsabha council "question"
dootsabha sabha "question"
dootsabha सभा "question"
```

---

## Development

```bash
make build        # Build bin/dootsabha
make test         # Unit tests
make test-binary  # L3 smoke tests (binary + mock providers)
make ci           # Lint + test (pre-push gate)
make check        # Full suite: fmt + lint + vet + test + smoke
make help         # All targets
```

### Project structure

```
cmd/dootsabha/        Entry point
internal/cli/         Cobra commands
internal/core/        Engine, config, subprocess, retry
internal/output/      Renderer, styles, formatters
internal/providers/   Claude/Codex/Antigravity wrappers
internal/plugin/      go-plugin gRPC infrastructure
internal/observability/ Structured logging + metrics
proto/                gRPC service definitions
plugins/              Built-in provider + strategy plugins
skill/                Claude Code skill (SKILL.md + references + examples)
configs/              Default configuration
scripts/              Smoke tests, agent tests
testdata/             Mock providers + plugins
```

---

## License

MIT
