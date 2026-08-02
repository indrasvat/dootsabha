---
name: dootsabha
description: >
  Orchestrates multiple AI coding CLIs (Claude, Codex, Antigravity, Grok) as a
  council. Use for a second opinion from another model, multi-perspective code
  review, peer review and synthesis across agents, validating a PRD or design for
  gaps, iterative refinement through reviewer feedback, or checking agent health.
  Replaces manual `codex exec` / `agy -p` / `grok -p` subprocess calls with
  structured JSON and exit codes built for agent control flow.
allowed-tools: Bash(dootsabha:*), Bash(./bin/dootsabha:*)
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

Responses report `Model` as the *backend* id (`grok-4.5-build`), which differs from
the configured `grok-4.5`. Match on prefix, not equality.

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

### `status` proves liveness, not quota

`status` runs each CLI's `--version`. A `Healthy: true` / `Auth: ✓` row means the
binary is installed and runnable — it does **not** check credentials or remaining
quota. An agent can look healthy and still fail mid-call with a rate-limit or
quota-exhausted error. `agy` hits this most often.

Treat it as routine, not as a broken tool: the provider's own error text is
surfaced at exit **3** (that agent failed) or exit **5** (council — others
succeeded). Retry with a different agent, or read `meta.providers` to see who is
still working.

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
| …the same, **on failure** | `data` becomes `{provider, error}` — **lowercase** | `jq -r '.data.error'` |
| `council`, `review`, `refine` | top-level object, **snake_case** fields | `jq -r '.synthesis.content'` |

> Reading the wrong shape yields **`null`, not an error** — `jq -r '.Content'` on a
> consult prints `null` and exits 0. If a field comes back null, check the envelope
> before assuming the agent returned nothing.

```bash
RESULT=$(dootsabha council --json "Redis or Memcached for session caching?")
echo "$RESULT" | jq -r '.synthesis.content'      # the answer
echo "$RESULT" | jq '.meta.total_cost_usd'       # what it cost
echo "$RESULT" | jq -r '.dispatch[] | select(.error != null) | .provider'   # who failed
```

`synthesis` is `null` when all agents failed. Per-agent errors live in
`dispatch[].error` and `meta.providers`.

## Exit codes

One code, one action:

| Code | Meaning | Do |
|---|---|---|
| 0 | Complete, usable | proceed |
| 2 | Bad flags/args, unknown agent or chair | fix the command |
| 3 | Every requested agent failed — nothing usable (incl. `status` with no healthy agent) | retry, or pick another agent |
| 4 | Timeout | raise `--timeout`, shrink the prompt |
| 5 | Some agents failed, output usable (incl. `status` degraded) | use it, note the gaps |
| 6 | Config missing or invalid | fix the config |
| 1 | Unexpected internal error | report a bug |

Precedence: `2 > 6 > 4 > 3 > 5 > 1 > 0`.

`3` vs `5` is the distinction that matters: **3 means nothing usable came back, 5
means you have an answer with gaps.**

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
