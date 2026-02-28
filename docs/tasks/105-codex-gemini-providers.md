# Task 1.6: Codex + Gemini Providers (Hardcoded)

## Status: PENDING

## Depends On
- Task 1.5 (claude provider — establishes patterns)

## Parallelizable With
- None

## Problem

Add codex and gemini providers following the pattern established in Task 1.5. Codex requires JSONL event stream parsing (not simple JSON). Gemini uses positional prompt with `--yolo` flag.

## PRD Reference
- §4.1 (Codex JSONL format, Gemini flags — verified behavior)
- §5.2 (Provider interface — same as claude)
- §6.3 (Consult command — provider output format)

## Files to Create
- `internal/providers/codex.go` — Codex provider with JSONL parsing
- `internal/providers/gemini.go` — Gemini provider
- `internal/providers/codex_test.go` — Unit tests with JSONL fixtures
- `internal/providers/gemini_test.go` — Unit tests

## Files to Modify
- `internal/providers/types.go` — Add any Codex-specific types if needed

## Execution Steps

### Step 1: Read spike findings
1. Read `_spikes/codex-jsonl/README.md` (Spike 0.1 — JSONL parsing)
2. Read `_spikes/gemini-json/README.md` (Spike 0.3 — JSON schema, flag behavior)

### Step 2: Implement Codex provider
- Build args: `codex exec --json --sandbox danger-full-access --skip-git-repo-check "prompt"`
- Parse JSONL line-by-line: find `item.completed` with `item.type == "agent_message"`
- Extract token usage from `turn.completed`
- Health check: `codex --version`

### Step 3: Implement Gemini provider
- Build args: `gemini --yolo --output-format json "prompt"` (positional prompt)
- Parse single JSON response
- Health check: `gemini --version`

### Step 4: Unit tests
- Codex JSONL parsing with fixture data
- Codex: missing agent_message event → error
- Codex: multiple agent_message events → take last
- Gemini JSON parsing
- Both: error handling (auth, timeout)

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Real CLIs (tiny prompt)
```bash
make build
./bin/dootsabha consult --agent codex "Say PONG"
./bin/dootsabha consult --agent gemini "Say PONG"
```

## Completion Criteria

1. Codex JSONL event stream parsed correctly
2. Gemini JSON output parsed correctly
3. Both providers follow same interface as claude
4. Health checks work for both
5. `make ci` passes

## Commit

```
feat(providers): add codex (JSONL) and gemini providers

- Codex: JSONL event stream parsing (agent_message + turn.completed)
- Gemini: --yolo --output-format json with positional prompt
- Health checks via --version for both
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4.1
5. Read spike findings (0.1, 0.3)
6. Execute steps 1-4
7. Run verification (L1 → L3)
8. **Change status to `DONE`**
9. Update `docs/PROGRESS.md`
10. Commit
