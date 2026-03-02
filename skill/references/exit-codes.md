# Exit Codes Reference

दूतसभा uses structured exit codes so agents can branch logic without parsing output.

## Exit Code Table

| Code | Constant | Meaning | Precedence |
|------|----------|---------|------------|
| 0 | ExitSuccess | Everything OK | 1 (lowest) |
| 1 | ExitError | General error | 2 |
| 2 | ExitUsage | Bad flags, missing arguments | 6 (highest) |
| 3 | ExitProvider | Provider error (CLI not found, auth invalid) | 4 |
| 4 | ExitTimeout | At least one agent timed out | 5 |
| 5 | ExitPartial | Partial result (some agents failed) | 3 |

When multiple errors occur in a multi-agent pipeline, the highest-precedence code wins:
`2 > 4 > 3 > 5 > 1 > 0`

## Per-Command Exit Codes

| Command | 0 | 1 | 2 | 3 | 4 | 5 |
|---------|---|---|---|---|---|---|
| `council` | All agents + synthesis OK | All agents failed | Bad flags | Provider error | Timeout | Partial (some failed) |
| `consult` | Agent responded | Error | Bad flags | Provider error | Timeout | Config error |
| `review` | Author + reviewer OK | Error | Bad flags | Provider error | Timeout | Config error |
| `refine` | All rounds completed | Error | Bad flags | Provider error | Timeout | Partial (some reviewers failed) |
| `status` | All healthy | Error | — | Some unhealthy | — | — |
| `config show` | Success | — | — | — | — | Config error |

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

if [ $exit_code -eq 2 ]; then
  echo "Usage error: $output" >&2
  exit 1
fi

if [ $exit_code -ne 0 ]; then
  echo "Agent error (exit $exit_code)" >&2
  exit 1
fi

# Safe to parse JSON
echo "$output" | jq -r '.content'
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
  jq -r '[.dispatch[] | select(.error == "") | .provider] | join(", ")' result.json

  # Synthesis is still attempted even with partial results
  jq -r '.synthesis.content' result.json
fi
```

### Health-gated workflow

```bash
# Count healthy agents
HEALTHY=$(dootsabha status --json | jq '[.[] | select(.healthy)] | length')

case $HEALTHY in
  0) echo "No agents available"; exit 1 ;;
  1) echo "One agent — using consult"
     AGENT=$(dootsabha status --json | jq -r '.[] | select(.healthy) | .name')
     dootsabha consult --json --agent "$AGENT" "Question" ;;
  *) echo "$HEALTHY agents — using council"
     dootsabha council --json "Question" ;;
esac
```
