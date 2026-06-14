---
name: dootsabha
description: >
  Multi-agent AI council orchestrator for coding tasks. Use when you need a
  second opinion from multiple LLMs, want to consult another AI agent (Claude,
  Codex, Antigravity), do a final review with codex or agy, run a multi-perspective
  code review, get peer review from multiple agents, have another LLM validate
  and review something, identify gaps in a PRD or design, refine output through
  iterative agent feedback, run it by another model, deliberate on a question
  with an AI council, incorporate findings from multiple LLMs, or check agent
  health status. Replaces manual `codex exec` and `agy -p` subprocess calls
  with structured JSON output and exit codes for agent control flow.
---

# दूतसभा — Multi-Agent Council Orchestrator

दूतसभा orchestrates AI coding agents (Claude Code, Codex CLI, Antigravity CLI) for
council-mode deliberation, peer review, iterative refinement, and single-agent
consultation. It produces structured JSON output with exit codes designed for
agent-to-agent workflows.

> **Provider note:** The third agent is `agy` (Google's Antigravity CLI), which
> replaced the retired Gemini CLI (sunset 2026-06-18). `agy` runs in plain-text
> print mode, so its results carry **no token counts or cost** — those JSON fields
> are `0`/empty for `agy`. `claude` and `codex` report full token/cost data.

**Prerequisite:** `dootsabha` binary on `$PATH` (see [installation](#installation))

## When to Use

- Get a **second opinion** from multiple LLMs on a question or design decision
- Do a **final review with codex/agy** — replaces manual `codex exec --yolo` and `agy -p` calls
- **Validate and review** a PRD, design doc, or implementation — identify gaps across multiple models
- Run **multi-perspective code review** with peer review and synthesis
- **Consult a single agent** (Claude, Codex, or Antigravity) with structured output
- **Refine output** through sequential reviewer feedback and author incorporation
- **Run it by another model** — quick cross-check without switching terminals
- **Incorporate findings** from multiple LLMs into a single synthesized answer
- Check **agent health** and availability before orchestrating workflows

## Agent Mode

Always use `--json` for structured, parseable output:

```bash
dootsabha <command> --json "<prompt>"
```

- `--json` returns structured JSON to stdout
- `--quiet` suppresses non-essential output (progress, spinners)
- `--timeout 5m` sets per-invocation timeout
- `-v` / `-vv` increases verbosity on stderr

## Quick Start

### 1. Council deliberation — multiple agents, peer review, synthesis

```bash
dootsabha council --json "What's the best approach to implement rate limiting?"
```

Dispatches to all configured agents. When running inside a Claude Code session,
defaults to codex and agy (Claude is already the host). When running standalone,
defaults to all three. Exit 0 = success, 5 = partial result.

### 2. Consult a single agent

```bash
dootsabha consult --json --agent claude "Explain this error: <error text>"
```

Queries one agent and returns structured output with cost, tokens, and duration.

### 3. Code review — author produces, reviewer critiques

```bash
dootsabha review --json "Review this function for security issues: <code>"
```

Default: codex authors, claude reviews. Override with `--author` and `--reviewer`.

### 4. Iterative refinement — sequential review + incorporation

```bash
dootsabha refine --json "Write a retry function with exponential backoff"
```

Author generates v1, each reviewer critiques, author incorporates feedback.
Produces versioned output showing the evolution.

### 5. Check agent health

```bash
dootsabha status --json
```

Returns health, version, model, and auth status for all configured providers.
Exit 0 = all healthy, 3 = one or more unhealthy.

## Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `council` | Multi-agent deliberation with synthesis | `--agents`, `--chair`, `--rounds`, `--parallel` |
| `consult` | Query a single agent | `--agent` (required), `--model`, `--max-turns` |
| `review` | Author + reviewer pipeline | `--author`, `--reviewer`, `--model` |
| `refine` | Sequential review + incorporation | `--author`, `--reviewers`, `--anonymous`, `--model` |
| `status` | Agent health check | (no command-specific flags) |
| `config show` | View configuration and source | `--reveal`, `--commented` |
| `config migrate` | Rewrite a stale `gemini` config to `agy` | `--dry-run`, `--json` |
| `plugin list` | Discover extensions | (no command-specific flags) |

For complete flag reference, see [references/command-reference.md](references/command-reference.md).

## JSON Output Shapes

### consult

Wrapped in an envelope. Fields are PascalCase (no json tags on struct).

```json
{
  "meta": { "schema_version": 1 },
  "data": {
    "Content": "Agent's response text",
    "Model": "claude-opus-4-8",
    "Duration": 3200000000,
    "CostUSD": 0.012,
    "TokensIn": 150,
    "TokensOut": 800,
    "SessionID": ""
  }
}
```

Extract content: `jq -r '.data.Content'`

### council

Written directly (no envelope wrapper). All fields snake_case with json tags.

```json
{
  "dispatch": [
    { "provider": "claude", "model": "...", "content": "...", "duration_ms": 3200, "cost_usd": 0.012, "tokens_in": 150, "tokens_out": 800 },
    { "provider": "codex",  "model": "...", "content": "...", "duration_ms": 2800, "cost_usd": 0.008, "tokens_in": 120, "tokens_out": 600 }
  ],
  "reviews": [
    { "reviewer": "claude", "reviewed": ["codex", "agy"], "content": "..." }
  ],
  "synthesis": {
    "chair": "claude",
    "content": "Synthesized answer combining all perspectives..."
  },
  "meta": {
    "schema_version": 1,
    "strategy": "council",
    "duration_ms": 12400,
    "total_cost_usd": 0.045,
    "total_tokens_in": 420,
    "total_tokens_out": 2000,
    "providers": { "claude": "ok", "codex": "ok", "agy": "ok" }
  }
}
```

**Note:** `synthesis` is `null` when all agents failed or synthesis was not reached.
`agy` contributes `0` to token/cost totals (plain-text print mode has no usage data).
On error with `--json`, per-agent errors are in `dispatch[].error` and `meta.providers`.

### review

Written directly (no envelope wrapper). All fields snake_case with json tags.

```json
{
  "author": { "provider": "codex", "model": "...", "content": "...", "duration_ms": 2800, "cost_usd": 0.008, "tokens_in": 120, "tokens_out": 600 },
  "review": { "provider": "claude", "model": "...", "content": "...", "duration_ms": 3200, "cost_usd": 0.012, "tokens_in": 150, "tokens_out": 800 },
  "meta": { "schema_version": 1, "strategy": "review", "duration_ms": 6000, "total_cost_usd": 0.020, "total_tokens_in": 270, "total_tokens_out": 1400, "providers": { "codex": "ok", "claude": "ok" } }
}
```

### refine

Written directly (no envelope wrapper). All fields snake_case with json tags.

```json
{
  "versions": [
    { "version": 1, "provider": "claude", "content": "v1 draft...", "duration_ms": 3200 },
    { "version": 2, "provider": "claude", "content": "v2 after codex review...", "reviewer": "codex", "review": "...", "duration_ms": 5400 },
    { "version": 3, "provider": "claude", "content": "v3 after agy review...", "reviewer": "agy", "review": "...", "duration_ms": 5200 }
  ],
  "final": { "version": 3, "content": "Final refined output..." },
  "meta": { "schema_version": 1, "strategy": "refine", "anonymous": true, "duration_ms": 18000, "total_cost_usd": 0.065, "total_tokens_in": 900, "total_tokens_out": 4200, "providers": { "claude": "ok", "codex": "ok", "agy": "ok" } }
}
```

### status

Wrapped in an envelope. Fields are PascalCase (no json tags on struct).

```json
{
  "meta": { "schema_version": 1 },
  "data": [
    { "Name": "agy",    "Healthy": true, "Version": "1.0.8", "Model": "Gemini 3.5 Flash (High)", "Auth": "✓", "Error": "" },
    { "Name": "claude", "Healthy": true, "Version": "2.1.63", "Model": "claude-opus-4-8", "Auth": "✓", "Error": "" },
    { "Name": "codex",  "Healthy": true, "Version": "0.106.0", "Model": "gpt-5.5", "Auth": "✓", "Error": "" }
  ]
}
```

Extract healthy count: `jq '[.data[] | select(.Healthy)] | length'`

## Exit Codes

Exit codes let you branch logic without parsing output:

| Code | Meaning | When |
|------|---------|------|
| 0 | Success | All agents responded, synthesis complete |
| 1 | Error | General error |
| 2 | Usage | Bad flags, missing arguments |
| 3 | Provider error | CLI not found, auth invalid, agent crashed |
| 4 | Timeout | At least one agent timed out |
| 5 | Partial result | Some agents failed, result is incomplete |

**Precedence** (when multiple errors occur): `2 > 4 > 3 > 5 > 1 > 0`

For conditional patterns, see [references/exit-codes.md](references/exit-codes.md).

## Core Workflow

```bash
# 1. Verify agents are available (status uses envelope + PascalCase)
dootsabha status --json | jq '[.data[] | select(.Healthy)] | length'

# 2. Get multi-perspective answer
RESULT=$(dootsabha council --json "Should we use Redis or Memcached for session caching?")

# 3. Extract the synthesized answer
echo "$RESULT" | jq -r '.synthesis.content'

# 4. Check cost
echo "$RESULT" | jq '.meta.total_cost_usd'

# 5. If partial result (exit 5), check which agents failed
echo "$RESULT" | jq '[.dispatch[] | select(.error)] | .[].provider'
```

## Installation

```bash
# From source
git clone https://github.com/indrasvat/dootsabha
cd dootsabha && make build
cp bin/dootsabha ~/.local/bin/

# Verify
dootsabha status --json
```

Requires at least one AI CLI installed: `claude`, `codex`, or `agy`.

## Configuration

दूतसभा works with built-in defaults. If `~/.config/dootsabha/config.yaml`
exists, it is auto-loaded as the default user config; otherwise the built-ins
are used. An explicit `--config <path>` overrides the auto-loaded default file.

`dootsabha config show` reports `config_source` so agents can distinguish
`built-in` configuration from a loaded file:

```bash
dootsabha config show --json | jq '.data.config_source'
```

Default user config path:

```yaml
providers:
  claude:
    binary: claude
    model: claude-opus-4-8
  codex:
    binary: codex
    model: gpt-5.5
  agy:
    binary: agy
    model: Gemini 3.5 Flash (High)

council:
  chair: claude
  parallel: true
  rounds: 1

timeout: 5m
```

> **Migrating from an older config?** If your `~/.config/dootsabha/config.yaml`
> still has a `gemini:` provider block, run `dootsabha config migrate` to rewrite
> it to `agy` (a `.bak` backup is written first). `dootsabha` also prints a one-line
> nudge on TTY when it detects a stale `gemini` reference.

Override with environment variables: `DOOTSABHA_PROVIDERS_CLAUDE_MODEL=claude-opus-4-8`

## Additional Resources

- [Command Reference](references/command-reference.md) — all commands, all flags, output schemas
- [Exit Codes](references/exit-codes.md) — branching logic and conditional patterns
- [Council Deliberation Example](examples/council-deliberation.md) — full multi-agent workflow
- [Review & Refine Example](examples/review-refine.md) — iterative improvement walkthrough
