---
name: dootsabha
description: >
  Orchestrates multiple AI coding CLIs (Claude, Codex, Antigravity, Grok) as a
  council. Use for a second opinion from another model, multi-perspective code
  review, peer review and synthesis across agents, validating a PRD or design for
  gaps, iterative refinement through reviewer feedback, or checking agent health.
  Replaces manual `codex exec` / `agy -p` / `grok -p` subprocess calls with
  structured JSON and exit codes built for agent control flow.
allowed-tools: Bash(dootsabha:*)
---

# दूतसभा — Multi-Agent Council Orchestrator

Runs several AI coding CLIs against one prompt, cross-reviews their answers, and
synthesizes a result. Output is JSON with exit codes designed for branching.

**Prerequisite:** `dootsabha` on `$PATH`. Verify with `dootsabha status --json`.

## Agents

| Agent | Tokens | Cost | Session ID | Default council member |
|---|---|---|---|---|
| `claude` | ✅ | ✅ | ✅ | ✅ (standalone only) |
| `codex` | ✅ | — | — | ✅ |
| `agy` | — | — | — | ✅ |
| `grok` | ✅ | ✅ | ✅ | ❌ **opt-in** |

`agy` runs plain-text print mode, so its token/cost/session fields are `0`/empty.

**`grok` is opt-in.** It never joins a council automatically — name it explicitly:
`--agent grok`, `--agents claude,codex,grok`, `--chair grok`, `--reviewers codex,grok`.
Check availability first; `status` lists it as `not installed (optional)` when the
`grok` CLI is absent, and that does **not** fail the command:

```bash
dootsabha status --json | jq -r '.data[] | select(.Name=="grok" and .Healthy) | .Name'
```

Non-empty output means grok is usable. Reach for it when a task benefits from a
fourth, independent perspective — it reports full token and cost data.

## Always use `--json`

```bash
dootsabha <command> --json "<prompt>"
```

Stdout is always **exactly one JSON document**, including on failure. Other global
flags: `--quiet`, `--timeout 5m`, `-v`/`-vv` (stderr only).

## Commands

| Command | Purpose | Key flags |
|---|---|---|
| `council` | Dispatch → peer review → synthesis | `--agents`, `--chair`, `--rounds`, `--parallel` |
| `consult` | Query one agent | `--agent` (required), `--model`, `--max-turns` |
| `review` | Author writes, reviewer critiques | `--author`, `--reviewer` |
| `refine` | Sequential review + incorporation | `--author`, `--reviewers`, `--anonymous` |
| `status` | Agent health | — |
| `config show` | Effective config + source | `--reveal`, `--commented` |
| `plugin list` | Discover extensions | — |

Inside a Claude Code session `council` defaults to `codex,agy` (Claude is already
the host); standalone it defaults to `claude,codex,agy`.

Full flags, output schemas, and the provider matrix:
[references/command-reference.md](references/command-reference.md).

## Extracting results

Two envelope shapes — this trips up most callers:

| Commands | Shape | Extract |
|---|---|---|
| `consult`, `status`, `config`, `plugin` | `{meta, data}`, **PascalCase** fields | `jq -r '.data.Content'` |
| `council`, `review`, `refine` | top-level object, **snake_case** fields | `jq -r '.synthesis.content'` |

```bash
RESULT=$(dootsabha council --json "Redis or Memcached for session caching?")
echo "$RESULT" | jq -r '.synthesis.content'      # the answer
echo "$RESULT" | jq '.meta.total_cost_usd'       # what it cost
echo "$RESULT" | jq -r '.dispatch[] | select(.error != null) | .provider'   # who failed
```

`synthesis` is `null` when all agents failed. Per-agent errors live in
`dispatch[].error` and `meta.providers`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Error — includes `council` when **all** agents failed |
| 3 | Provider error — CLI missing, auth invalid, agent crashed |
| 4 | Timeout |
| 5 | Partial — some agents failed, result still usable |

Precedence: `4 > 3 > 5 > 1 > 0`.

> Exit **2** is documented in the PRD but the CLI does not emit it — usage errors
> exit 1. Do not branch on 2.

Branching patterns: [references/exit-codes.md](references/exit-codes.md).

## Configuration

Works with built-in defaults. `~/.config/dootsabha/config.yaml` is auto-loaded if
present; `--config <path>` overrides it. Check which is in effect:

```bash
dootsabha config show --json | jq -r '.data.config_source.type'   # built-in | file
```

Per-provider overrides use `DOOTSABHA_PROVIDERS_<NAME>_{BINARY,MODEL}`. Config
shape and every key: [references/command-reference.md](references/command-reference.md).

## Resources

- [Command reference](references/command-reference.md) — flags, schemas, provider matrix
- [Exit codes](references/exit-codes.md) — branching logic
- [Council example](examples/council-deliberation.md) · [Review & refine example](examples/review-refine.md)
