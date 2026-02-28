# Task 0.1: Codex JSONL Parsing Spike

## Status: PENDING

## Depends On
- None (first spike)

## Parallelizable With
- All other spikes (0.2–0.8)

## Problem

Codex CLI outputs a JSONL event stream, not a single JSON object. We must reliably extract the final content from `item.completed` where `item.type == "agent_message"` and token usage from `turn.completed`. This spike validates parsing before production code.

## PRD Reference
- §4.1 (Codex JSONL format — verified structure with 4 event types)
- §11 (Risk: Codex JSONL format changes)

## Files to Create
- `_spikes/codex-jsonl/main.go` — Spike program
- `_spikes/codex-jsonl/README.md` — Findings doc

## Execution Steps

### Step 1: Read context
1. Read PRD §4.1 (Codex JSONL format block)
2. Read PRD §11 (Codex JSONL risk)

### Step 2: Write spike program
- Run `codex exec --json "Say PONG" 2>/dev/null` and capture stdout
- Parse JSONL line-by-line with `json.Decoder`
- Extract final content from `item.completed` where `item.type == "agent_message"`
- Extract token usage from `turn.completed`
- Handle edge cases: empty stream, no agent_message, malformed JSON lines

### Step 3: Test with real CLI
- Run spike against real Codex CLI
- Test with different prompts (short, long, error-inducing)
- Test with `--sandbox danger-full-access` and `--skip-git-repo-check`

### Step 4: Document findings
- Exact event types observed
- Any undocumented fields
- Error behavior (auth failure, rate limit, timeout)
- Recommended Go types for JSONL events

## Verification

### L1: Spike runs
```bash
cd _spikes/codex-jsonl && go run main.go
```

### L3: Real CLI output
```bash
codex exec --json "Say PONG" 2>/dev/null | head -20
# Compare against spike's parsed output
```

## Completion Criteria

1. Spike successfully parses Codex JSONL event stream
2. Extracts agent_message content reliably
3. Extracts token usage from turn.completed
4. README.md documents: event types, Go types, edge cases, error behavior

## Commit

```
spike(codex-jsonl): validate JSONL event stream parsing

- Line-by-line json.Decoder for Codex JSONL
- Extracts agent_message content + turn.completed usage
- Documents event types and error behavior
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4.1
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
