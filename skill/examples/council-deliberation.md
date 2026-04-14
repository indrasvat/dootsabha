# Example: Council Deliberation Walkthrough

A complete walkthrough of using दूतसभा council mode to get multi-agent
perspectives on a design decision.

## Scenario

You're deciding between Redis and PostgreSQL for a job queue, and want
input from multiple AI agents before committing to an approach.

## Step 1: Check Agent Health

```bash
dootsabha status --json | jq '.data[] | {Name, Healthy, Model}'
```

Output (status uses envelope + PascalCase):
```json
{"Name":"claude","Healthy":true,"Model":"claude-opus-4-6"}
{"Name":"codex","Healthy":true,"Model":"gpt-5.4"}
{"Name":"gemini","Healthy":true,"Model":"gemini-3.1-pro-preview"}
```

Exit code: `0` (all healthy).

## Step 2: Run Council

```bash
dootsabha council --json \
  "We need a job queue for background tasks: email sending, PDF generation,
   and webhook delivery. Compare Redis-based (Bull/BullMQ) vs PostgreSQL-based
   (pgboss/Graphile Worker) approaches. Consider: operational complexity,
   failure handling, observability, and our existing Postgres stack." > result.json
```

This runs three stages:
1. **Dispatch** — All three agents answer independently
2. **Peer Review** — Each agent reviews the others' answers
3. **Synthesis** — Chair (claude) synthesizes everything

## Step 3: Extract the Synthesis

```bash
# Get the synthesized answer
jq -r '.synthesis.content' result.json

# See which agent chaired
jq -r '.synthesis.chair' result.json
```

## Step 4: Check Individual Perspectives

```bash
# See each agent's take
jq -r '.dispatch[] | "--- \(.provider) ---\n\(.content)\n"' result.json

# See peer reviews (reviewed is an array of agent names)
jq -r '.reviews[] | "\(.reviewer) on \(.reviewed | join(", ")): \(.content)\n"' result.json
```

## Step 5: Check Cost and Duration

```bash
jq '{
  duration_s: (.meta.duration_ms / 1000),
  cost_usd: .meta.total_cost_usd,
  agents: .meta.providers
}' result.json
```

Output (`providers` is a map of agent → status):
```json
{
  "duration_s": 12.4,
  "cost_usd": 0.045,
  "agents": { "claude": "ok", "codex": "ok", "gemini": "ok" }
}
```

## Step 6: Handling Partial Results

If one agent fails, council still produces a result (exit 5):

```bash
dootsabha council --json "Question" > result.json
exit_code=$?

if [ $exit_code -eq 5 ]; then
  # See which agents failed
  jq -r '.dispatch[] | select(.error != "") | "\(.provider): \(.error)"' result.json

  # Synthesis still works with remaining agents
  jq -r '.synthesis.content' result.json
fi
```

## Narrowing to Specific Agents

If you only want claude and codex (skip gemini):

```bash
dootsabha council --json --agents claude,codex "Your question"
```

## Choosing a Different Chair

Let codex synthesize instead of claude:

```bash
dootsabha council --json --chair codex "Your question"
```

## Sequential Mode (for Debugging)

Run agents one at a time instead of in parallel:

```bash
dootsabha council --json --parallel=false "Your question"
```
