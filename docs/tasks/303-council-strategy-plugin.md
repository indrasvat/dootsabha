# Task 3.4: Council Strategy Plugin

## Status: PENDING

## Depends On
- Task 3.3 (extract providers)

## Parallelizable With
- None

## Problem

The council 3-stage pipeline (dispatch → review → synthesis) is currently hardcoded in the engine. Extract it into a Strategy plugin so alternative strategies can be implemented (e.g., debate, voting, tournament).

## PRD Reference
- §5.3 (Strategy plugin type: Execute method)
- §6.2 (Council command — the default strategy behavior)

## Files to Create
- `plugins/council-strategy/main.go` — Default council strategy plugin
- `plugins/council-strategy/strategy_test.go` — Unit tests

## Files to Modify
- `internal/core/engine.go` — Delegate to strategy plugin (with built-in fallback)

## Execution Steps

### Step 1: Define strategy contract
- Strategy receives: prompt, agents, config
- Strategy returns: final output (after all stages)
- Strategy internally manages: dispatch, review, synthesis stages

### Step 2: Implement default council strategy
- Implements Strategy gRPC service
- Reuses dispatch/review/synthesis logic
- Reports progress back to host

### Step 3: Wire engine to strategy plugin
- Engine checks for strategy plugin first
- Falls back to built-in logic if plugin not found

### Step 4: Unit tests
- Strategy plugin executes 3-stage pipeline
- Engine delegates to strategy
- Fallback to built-in when plugin missing

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Strategy via plugin
```bash
make build build-plugins
./bin/dootsabha council "PONG" --json | python3 -m json.tool
```

## Completion Criteria

1. Council strategy extracted into plugin
2. Engine delegates to strategy plugin
3. Fallback to built-in works
4. Zero regression
5. `make ci` passes

## Commit

```
feat(strategy): extract council pipeline into strategy plugin

- Default council-strategy plugin (dispatch → review → synthesis)
- Engine delegates to strategy via gRPC
- Built-in fallback when plugin not found
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.3, §6.2
5. Execute steps 1-4
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
