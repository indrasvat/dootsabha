# Task 2.3: Synthesis Stage (Chair Agent)

## Status: PENDING

## Depends On
- Task 2.2 (peer review)

## Parallelizable With
- None (final pipeline stage)

## Problem

Council Stage 3: the chair agent synthesizes all dispatch outputs + peer reviews into a unified answer. Must handle chair failure (re-invoke fallback agent with synthesis prompt), multi-round behavior, and stop conditions.

## PRD Reference
- §6.2 (Synthesis stage, chair failure semantics, multi-round behavior, stop conditions)
- §6.2 acceptance criteria FR-COU-04, FR-COU-05, FR-COU-08

## Files to Create
- `internal/core/synthesis.go` — Synthesis logic with chair fallback
- `internal/core/synthesis_test.go` — Unit tests

## Files to Modify
- `internal/core/engine.go` — Add synthesis stage, multi-round support

## Execution Steps

### Step 1: Implement synthesis
- Chair invoked with: "Synthesize these agent responses and reviews into a unified answer:\n\n{dispatch outputs}\n\n{reviews}"
- Chair defaults to claude (configurable via `--chair`)

### Step 2: Implement chair failure fallback
- If chair fails → **re-invoke** first healthy non-chair agent with synthesis prompt
- NOT reuse existing output — fresh invocation with synthesis role
- Log as warning; JSON output includes `"chair_fallback": "codex"`

### Step 3: Implement multi-round support
- `--rounds N`: each round feeds previous synthesis back as context
- Round N prompt = original + "Previous synthesis: {N-1 output}"
- Stop conditions: round limit, session timeout, chair convergence
- Context cap: 32KB per round fed to next round

### Step 4: Render final output
- TTY: styled output with stage headers, footer stats
- JSON: full structure per §6.2 JSON schema
- Piped: clean text, no ANSI

### Step 5: Unit tests
- Synthesis produces unified answer
- Chair failure → fallback invoked with synthesis prompt
- Multi-round: 2 rounds produce expected chain
- Stop conditions (timeout, round limit)

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Full council
```bash
make build
./bin/dootsabha council "What is a goroutine?" --agents claude,codex,gemini
./bin/dootsabha council "PONG" --json | python3 -m json.tool
./bin/dootsabha council "PONG" --rounds 2
```

## Completion Criteria

1. Chair agent produces synthesized answer
2. Chair failure → fallback agent re-invoked with synthesis prompt
3. Multi-round works with context chaining
4. JSON output matches §6.2 schema
5. Footer stats: total time, cost, tokens, agent status
6. `make ci` passes

## Commit

```
feat(council): add synthesis stage with chair fallback and multi-round

- Chair agent synthesizes dispatch outputs + reviews
- Chair failure: re-invoke fallback with synthesis prompt
- Multi-round with context chaining and 32KB cap
- Stop conditions: round limit, session timeout, convergence
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.2 (synthesis, chair failure, multi-round)
5. Execute steps 1-5
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
