# Exit Codes Reference

दूतसभा uses structured exit codes so agents can branch logic without parsing output.

- [Exit code table](#exit-code-table)
- [Per-command exit codes](#per-command-exit-codes)
- [Conditional patterns](#conditional-patterns)

## Exit Code Table

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | ExitSuccess | Everything OK |
| 1 | ExitError | General error; also usage errors, and `council` when **all** agents failed |
| 3 | ExitProvider | Provider error (CLI not found, auth invalid, agent crashed, quota exhausted) |
| 4 | ExitTimeout | At least one agent timed out |
| 5 | ExitPartial | **Config error, or** partial result (some agents failed) |

When several apply, the highest wins: `4 > 3 > 5 > 1 > 0`.

**Exit 5 is overloaded on every command** — a bad `--config` and a partial council
result both exit 5. Branch on the payload, not the code: `.data.error` present
means a config error; a `dispatch[]` array means a partial result you can still use.

> **`ExitUsage` (2) is not emitted.** The PRD and CLAUDE.md define it, but every
> usage error — unknown flag, missing argument, unknown provider, unknown chair —
> exits **1**. Do not branch on 2; treat 1 as "bad invocation or general error".

## Per-Command Exit Codes

| Command | 0 | 1 | 3 | 4 | 5 |
|---------|---|---|---|---|---|
| `council` | All agents + synthesis OK | Bad flags; all agents failed | Provider/synthesis error | Timeout | Partial (some failed) |
| `consult` | Agent responded | Bad flags; missing `--agent` | Provider error, quota exhausted | Timeout | Config error |
| `review` | Author + reviewer OK | Bad flags | Provider error | Timeout | Config error |
| `refine` | All rounds completed | Bad flags | Provider error | Timeout | Partial (some reviewers failed) |
| `status` | All healthy | Error | Some unhealthy¹ | — | — |
| `config show` | Success | — | — | — | Config error |

¹ An **opt-in** provider (`grok`) that is simply not installed does **not** make
`status` fail — it is listed as `not installed (optional)` and the command exits 0.
An installed-but-broken provider, or any absent required provider, still exits 3.

## Conditional Patterns

### Check if agents are healthy before running council

```bash
if dootsabha status --json > /dev/null 2>&1; then
  echo "All agents healthy — running council"
  dootsabha council --json "Your question here"
else
  echo "Some agents unavailable — falling back to single consult"
  dootsabha consult --json --agent claude "Your question here"
fi
```

### Branch on council result

```bash
dootsabha council --json "Design review for auth module" > result.json 2>/dev/null
case $? in
  0) echo "Full council result"; jq -r '.synthesis.content' result.json ;;
  5) echo "Partial result — some agents failed"; jq -r '.synthesis.content' result.json ;;
  4) echo "Timed out — try with fewer agents or longer timeout" ;;
  3) echo "Provider error — check agent health" ;;
  1) echo "All agents failed" ;;
esac
```

### Guard against errors

```bash
output=$(dootsabha consult --json --agent claude "Explain this error" 2>&1)
exit_code=$?

if [ $exit_code -ne 0 ]; then
  # exit 5 is overloaded: config error vs partial result. Check the payload.
  reason=$(echo "$output" | jq -r '.data.error // "agent failure"' 2>/dev/null)
  echo "dootsabha failed (exit $exit_code): $reason" >&2
  exit 1
fi

# Safe to parse JSON
echo "$output" | jq -r '.data.Content'
```

### Timeout handling with fallback

```bash
# Try council with 2-minute timeout
dootsabha council --json --timeout 2m "Complex question" > result.json 2>/dev/null
if [ $? -eq 4 ]; then
  echo "Council timed out — falling back to single agent"
  dootsabha consult --json --agent claude --timeout 5m "Complex question" > result.json
fi
```

### Partial result extraction

```bash
# Council may return exit 5 (partial) — still has useful data
dootsabha council --json "Question" > result.json 2>/dev/null
exit_code=$?

if [ $exit_code -eq 0 ] || [ $exit_code -eq 5 ]; then
  # Extract which agents succeeded
  jq -r '[.dispatch[] | select(.error == null) | .provider] | join(", ")' result.json

  # Synthesis is still attempted even with partial results
  jq -r '.synthesis.content' result.json
fi
```

### Health-gated workflow

```bash
# Count healthy agents
HEALTHY=$(dootsabha status --json | jq '[.data[] | select(.Healthy)] | length')

case $HEALTHY in
  0) echo "No agents available"; exit 1 ;;
  1) echo "One agent — using consult"
     AGENT=$(dootsabha status --json | jq -r '.data[] | select(.Healthy) | .Name')
     dootsabha consult --json --agent "$AGENT" "Question" ;;
  *) echo "$HEALTHY agents — using council"
     dootsabha council --json "Question" ;;
esac
```
