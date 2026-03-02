# Command Reference

Complete flag and output schema reference for all दूतसभा commands.

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | | `false` | Output as structured JSON |
| `--verbose` | `-v` | `0` | Verbosity level (-v=info, -vv=debug, -vvv=debug+source) |
| `--quiet` | `-q` | `false` | Suppress non-essential output |
| `--timeout` | | `5m` | Per-invocation timeout (e.g., 5m, 30s) |
| `--session-timeout` | | `30m` | Max total duration for multi-agent pipelines |
| `--config` | | auto-detected | Path to config file (YAML) |

## council (sabha / सभा)

Multi-agent deliberation with dispatch, peer review, and synthesis.

```bash
dootsabha council [flags] "<prompt>"
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--agents` | | `claude,codex,gemini` | Comma-separated agent names |
| `--chair` | | from config (`claude`) | Agent for synthesis |
| `--rounds` | | from config (`1`) | Number of deliberation rounds (max 5) |
| `--parallel` | | `true` | Run dispatch in parallel |

### Pipeline

1. **Dispatch** — Send prompt to all agents (parallel or sequential)
2. **Peer Review** — Each agent reviews other agents' outputs (skipped if <2 succeed)
3. **Synthesis** — Chair agent synthesizes all responses and reviews into a final answer

### JSON Output Schema

Written directly (no envelope wrapper). All fields snake_case.

```json
{
  "dispatch": [{
    "provider": "string",
    "model": "string",
    "content": "string",
    "duration_ms": 0,
    "cost_usd": 0.0,
    "tokens_in": 0,
    "tokens_out": 0,
    "error": "string (omitted if success)"
  }],
  "reviews": [{
    "reviewer": "string",
    "reviewed": ["string"],
    "content": "string",
    "error": "string (omitted if success)"
  }],
  "synthesis": {
    "chair": "string",
    "chair_fallback": "string (omitted if no fallback)",
    "content": "string"
  },
  "meta": {
    "schema_version": 1,
    "strategy": "council",
    "duration_ms": 0,
    "total_cost_usd": 0.0,
    "total_tokens_in": 0,
    "total_tokens_out": 0,
    "providers": { "claude": "ok", "codex": "ok" }
  }
}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All agents responded, synthesis complete |
| 1 | All agents failed |
| 3 | Provider error |
| 4 | At least one agent timed out |
| 5 | Partial result (some agents failed, synthesis may be incomplete) |

---

## consult (paraamarsh / परामर्श)

Query a single AI agent.

```bash
dootsabha consult [flags] --agent <name> "<prompt>"
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--agent` | `-a` | (required) | Agent name: claude, codex, or gemini |
| `--model` | | from config | Override model for this invocation |
| `--max-turns` | | `0` (no limit) | Maximum agent turns |

### JSON Output Schema

Wrapped in envelope. Fields are PascalCase (no json tags on ProviderResult struct).
`Duration` is `time.Duration` in nanoseconds, not milliseconds.

```json
{
  "meta": { "schema_version": 1 },
  "data": {
    "Content": "string",
    "Model": "string",
    "Duration": 0,
    "CostUSD": 0.0,
    "TokensIn": 0,
    "TokensOut": 0,
    "SessionID": "string"
  }
}
```

Extract content: `jq -r '.data.Content'`

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error |
| 3 | Provider error (CLI not found, auth invalid) |
| 4 | Timeout |
| 5 | Config error |

---

## review (sameeksha / समीक्षा)

Author produces content, reviewer reviews it.

```bash
dootsabha review [flags] "<prompt>"
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--author` | | `codex` | Agent that produces output |
| `--reviewer` | | `claude` | Agent that reviews output |
| `--model` | | from config | Override model for both agents |

### Pipeline

1. **Author** generates output from the prompt
2. **Reviewer** reviews the author's output with context

### JSON Output Schema

Written directly (no envelope wrapper). All fields snake_case.

```json
{
  "author": {
    "provider": "string",
    "model": "string",
    "content": "string",
    "duration_ms": 0,
    "cost_usd": 0.0,
    "tokens_in": 0,
    "tokens_out": 0
  },
  "review": {
    "provider": "string",
    "model": "string",
    "content": "string",
    "duration_ms": 0,
    "cost_usd": 0.0,
    "tokens_in": 0,
    "tokens_out": 0
  },
  "meta": {
    "schema_version": 1,
    "strategy": "review",
    "duration_ms": 0,
    "total_cost_usd": 0.0,
    "total_tokens_in": 0,
    "total_tokens_out": 0,
    "providers": { "codex": "ok", "claude": "ok" }
  }
}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Author and reviewer both succeeded |
| 1 | Error |
| 3 | Provider error |
| 4 | Timeout |
| 5 | Config error |

---

## refine (sanshodhan / संशोधन)

Sequential review and incorporation pipeline.

```bash
dootsabha refine [flags] "<prompt>"
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--author` | | `claude` | Agent that produces and refines |
| `--reviewers` | | `codex,gemini` | Comma-separated ordered reviewer list |
| `--anonymous` | | `true` | Anonymize reviewer identities in prompts |
| `--model` | | from config | Override model for all agents |

### Pipeline

1. **Author** generates v1 from prompt
2. For each reviewer in order:
   - **Reviewer** critiques current version
   - **Author** incorporates feedback → new version
3. Output contains full version history

### JSON Output Schema

Written directly (no envelope wrapper). All fields snake_case.

```json
{
  "versions": [{
    "version": 1,
    "provider": "string",
    "content": "string",
    "reviewer": "string (empty for v1)",
    "review": "string (empty for v1)",
    "duration_ms": 0,
    "cost_usd": 0.0,
    "tokens_in": 0,
    "tokens_out": 0
  }],
  "final": {
    "version": 0,
    "content": "string"
  },
  "meta": {
    "schema_version": 1,
    "strategy": "refine",
    "anonymous": true,
    "duration_ms": 0,
    "total_cost_usd": 0.0,
    "total_tokens_in": 0,
    "total_tokens_out": 0,
    "providers": { "claude": "ok", "codex": "ok", "gemini": "ok" }
  }
}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All rounds completed |
| 1 | Error |
| 3 | Provider error |
| 4 | Timeout |
| 5 | Partial result (some reviewers failed) |

---

## status (sthiti / स्थिति)

Show health status of all configured agents.

```bash
dootsabha status [flags]
```

No command-specific flags.

### JSON Output Schema

Wrapped in envelope. Fields are PascalCase (no json tags on healthRow struct).

```json
{
  "meta": { "schema_version": 1 },
  "data": [{
    "Name": "string",
    "Healthy": true,
    "Version": "string",
    "Model": "string",
    "Auth": "string",
    "Error": "string"
  }]
}
```

Extract healthy agents: `jq '[.data[] | select(.Healthy)] | length'`

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All providers healthy |
| 1 | Error |
| 3 | One or more providers unhealthy |

---

## config show (vinyaas / विन्यास)

View current configuration.

```bash
dootsabha config show [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Output as JSON |
| `--reveal` | `false` | Show sensitive values (tokens, keys) |
| `--commented` | `false` | Include field descriptions as YAML comments |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 5 | Config error (file not found, parse error) |

---

## plugin list / inspect (vistaarak / विस्तारक)

Discover and inspect plugins and extensions.

```bash
dootsabha plugin list [flags]
dootsabha plugin inspect <name> [flags]
```

### JSON Output Schema (list)

Wrapped in envelope:

```json
{
  "meta": { "schema_version": 1 },
  "data": [{
    "name": "string",
    "type": "string",
    "path": "string",
    "status": "string"
  }]
}
```

Type values: `plugin`, `extension`, `provider`, `strategy`, `hook`.
Status values: `installed`, `available`.
