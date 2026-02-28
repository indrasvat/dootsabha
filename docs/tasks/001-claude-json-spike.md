# Task 0.2: Claude JSON Output Spike

## Status: PENDING

## Depends On
- None

## Parallelizable With
- All other spikes (0.1, 0.3–0.8)

## Problem

Claude CLI's `--output-format json` schema is not formally documented. We must capture the exact JSON structure, cost fields, error cases, and the `CLAUDECODE` env var nested session gotcha before writing production code.

## PRD Reference
- §4.1 (Claude CLI version, JSON flag, nested session gotcha)
- §5.5 (Key decision: unset CLAUDECODE in subprocess env)
- §11 (Risk: Claude JSON schema undocumented)

## Files to Create
- `_spikes/claude-json/main.go` — Spike program
- `_spikes/claude-json/README.md` — Findings doc

## Execution Steps

### Step 1: Read context
1. Read PRD §4.1 (Claude CLI flags + nested session gotcha)

### Step 2: Write spike program
- Run `claude -p "Say PONG" --output-format json --dangerously-skip-permissions` with `CLAUDECODE` unset
- Parse JSON response and extract: content, session_id, cost, model, tokens
- Test with and without `CLAUDECODE` set to verify the nested session error
- Test `--model` override

### Step 3: Test error cases
- Invalid auth (unset API key) → capture error JSON structure
- Timeout behavior
- Invalid model name

### Step 4: Document findings
- Exact JSON schema with all fields
- Which fields are nullable/optional
- Error response format
- CLAUDECODE env var behavior confirmed

## Verification

### L1: Spike runs
```bash
cd _spikes/claude-json && go run main.go
```

### L3: Real CLI output
```bash
claude -p "Say PONG" --output-format json --dangerously-skip-permissions 2>/dev/null | python3 -m json.tool
```

## Completion Criteria

1. Spike successfully parses Claude JSON output
2. All JSON fields documented with Go types
3. CLAUDECODE nested session gotcha confirmed and documented
4. Error response format documented

## Commit

```
spike(claude-json): validate JSON output schema and env var gotcha

- Captures exact JSON schema from claude --output-format json
- Confirms CLAUDECODE env var must be unset in subprocess
- Documents all fields, nullable fields, error format
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4.1, §5.5
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
