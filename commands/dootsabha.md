# दूतसभा (dootsabha) — Multi-Agent Council

दूतसभा orchestrates multiple AI coding agents (Claude Code, Codex CLI, Antigravity CLI (agy), xAI Grok CLI (grok)) through council-mode deliberation, peer review, and synthesis. `grok` is opt-in — select it with `--agent grok` / `--agents …,grok` / `--chair grok`. Use it when you need multi-perspective answers from multiple AI agents.

## Quick Reference

### Get a multi-perspective answer (council)

```bash
dootsabha council --json "What's the best way to implement a rate limiter in Go?"
```

Output structure:
```json
{
  "dispatch": [
    { "provider": "claude", "model": "claude-opus-4-8", "content": "...", "duration_ms": 5000, "cost_usd": 0.01, "tokens_in": 100, "tokens_out": 500 },
    { "provider": "codex", "content": "..." },
    { "provider": "agy", "content": "...", "cost_usd": 0, "tokens_in": 0, "tokens_out": 0 }
  ],
  "reviews": [
    { "reviewer": "claude", "reviewed": ["codex", "agy"], "content": "..." }
  ],
  "synthesis": { "chair": "claude", "content": "The synthesized best answer..." },
  "meta": { "schema_version": 1, "strategy": "council", "total_cost_usd": 0.05, "total_tokens_in": 1000, "total_tokens_out": 3000, "duration_ms": 15000, "providers": { "claude": "ok", "codex": "ok", "agy": "ok" } }
}
```

Extract synthesis: `dootsabha council --json "question" | jq -r '.synthesis.content'`

### Query a single agent (consult)

```bash
dootsabha consult --json --agent claude "Explain Go interfaces"
```

Output structure (envelope format):
```json
{
  "meta": { "schema_version": 1 },
  "data": {
    "Content": "...",
    "Model": "claude-opus-4-8",
    "Duration": 3000000000,
    "CostUSD": 0.005,
    "TokensIn": 50,
    "TokensOut": 500,
    "SessionID": "abc123"
  }
}
```

Extract content: `dootsabha consult --json --agent claude "question" | jq -r '.data.Content'`

### Get code reviewed (review)

One agent writes, another reviews:

```bash
dootsabha review --json "Write a retry function with exponential backoff" --author codex --reviewer claude
```

Output structure:
```json
{
  "author": { "provider": "codex", "model": "gpt-5.5", "content": "...", "duration_ms": 4000 },
  "review": { "provider": "claude", "model": "claude-opus-4-8", "content": "...", "duration_ms": 3000 },
  "meta": { "schema_version": 1, "strategy": "review", "duration_ms": 7000, "total_cost_usd": 0.02, "providers": { "codex": "ok", "claude": "ok" } }
}
```

Extract review: `dootsabha review --json "question" | jq -r '.review.content'`

### Iterative refinement (refine)

Author writes, reviewers review sequentially, author incorporates feedback:

```bash
dootsabha refine --json "Implement a concurrent-safe LRU cache" --author claude --reviewers codex,agy
```

Output structure:
```json
{
  "versions": [
    { "version": 1, "provider": "claude", "content": "initial draft...", "duration_ms": 5000 },
    { "version": 2, "provider": "claude", "content": "revised after codex review...", "reviewer": "codex", "review": "codex's feedback...", "duration_ms": 4000 }
  ],
  "final": { "version": 2, "content": "final version..." },
  "meta": { "schema_version": 1, "strategy": "refine", "anonymous": true, "duration_ms": 15000, "total_cost_usd": 0.03, "providers": { "claude": "ok", "codex": "ok", "agy": "ok" } }
}
```

Extract final version: `dootsabha refine --json "question" | jq -r '.final.content'`

### Check agent health (status)

```bash
dootsabha status --json
```

Output structure (envelope format):
```json
{
  "meta": { "schema_version": 1 },
  "data": [
    { "Name": "agy", "Healthy": true, "Version": "<version>", "Model": "Gemini 3.5 Flash (High)", "Reachable": "\u2713", "Installed": true },
    { "Name": "claude", "Healthy": true, "Version": "<version>", "Model": "claude-opus-4-8", "Reachable": "\u2713", "Installed": true },
    { "Name": "codex", "Healthy": true, "Version": "<version>", "Model": "gpt-5.5", "Reachable": "\u2713", "Installed": true },
    { "Name": "grok", "Healthy": false, "Version": "", "Model": "", "Reachable": "\u2014", "Error": "...", "Installed": false }
  ]
}
```

`grok` is opt-in. When its CLI is absent it shows `Installed: false` and `status`
still exits 0 — only required or installed-but-broken providers fail the check.

Check for unhealthy agents: `dootsabha status --json | jq '.data[] | select(.Healthy == false)'`

### View configuration (config)

```bash
dootsabha config show --json
dootsabha config show --commented  # Human-readable with inline docs
```

### Migrate stale config (config migrate)

Rewrites a legacy config that still references the retired `gemini` provider to the new `agy` (Antigravity CLI) provider. Writes a `<config>.bak` backup before modifying:

```bash
dootsabha config migrate            # migrate in place (writes <config>.bak)
dootsabha config migrate --dry-run  # show what would change, write nothing
dootsabha config migrate --json     # structured result for agent consumption
```

## Exit Codes

| Code | Meaning | Agent Response |
|------|---------|---------------|
| 0 | Success | Use the output |
| 1 | Unexpected internal error | Report a bug |
| 2 | Bad command — flags, args, unknown agent/chair | Fix the command syntax |
| 6 | Config missing or invalid | Fix the config file |
| 3 | Provider error | Agent CLI not found or crashed; check `dootsabha status` |
| 4 | Timeout | Increase `--timeout` or simplify prompt |
| 5 | Partial result | Some agents failed but output is still usable |



## Common Patterns

### Get best answer for a hard question
```bash
dootsabha council --json "question" | jq -r '.synthesis.content'
```

### Compare agent perspectives
```bash
dootsabha council --json "question" | jq -r '.dispatch[] | "\(.provider): \(.content[:200])"'
```

### Use specific agents only
```bash
dootsabha council --json "question" --agents claude,codex --chair claude
```

### Override model
```bash
dootsabha consult --json --agent claude --model claude-opus-4-8 "question"
```

### Set timeout
```bash
dootsabha council --json "question" --timeout 10m
```

### Check if agent is available before using
```bash
if dootsabha status --json | jq -e '.data[] | select(.Name == "claude" and .Healthy == true)' > /dev/null 2>&1; then
  dootsabha consult --json --agent claude "question"
fi
```

## Global Flags

| Flag | Purpose |
|------|---------|
| `--json` | JSON output (always use this from agents) |
| `--quiet` | Suppress progress output |
| `--timeout 5m` | Per-agent timeout |
| `--session-timeout 30m` | Total pipeline timeout |
| `-v` / `-vv` / `-vvv` | Verbosity (info / debug / debug+source) |
| `--config path` | Custom config file |
