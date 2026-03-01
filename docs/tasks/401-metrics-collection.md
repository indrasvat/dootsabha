# Task 4.2: Metrics Collection (In-Process Counters)

## Status: PENDING

## Depends On
- Phase 3 complete

## Parallelizable With
- Task 4.1 (structured logging)

## Problem

दूतसभा needs in-process metrics to track: per-provider invocation count, latency, cost, tokens, error rates. These power the footer stats in council output and the JSON meta blocks.

## PRD Reference
- §6.2 (Council footer stats: total time, cost, tokens, agent status)
- §6.3 (Consult footer: tokens, cost, session)
- §7.1 (Performance targets)

## Files to Create
- `internal/observability/metrics.go` — In-process metrics collector
- `internal/observability/metrics_test.go` — Unit tests

## Files to Modify
- `internal/core/engine.go` — Record metrics during dispatch/review/synthesis
- `internal/output/schemas.go` — Ensure meta blocks pull from metrics

## Execution Steps

### Step 1: Design metrics collector
- `Metrics` struct with thread-safe counters
- Per-provider: invocation count, total duration, total cost, total tokens (in/out), error count
- Session-level: total duration, total cost, total tokens

### Step 2: Implement metrics recording
- `RecordInvocation(provider string, duration time.Duration, cost float64, tokensIn, tokensOut int, err error)`
- `Summary() MetricsSummary` — aggregated stats for footer/JSON

### Step 3: Wire into engine
- Record after each provider invocation (dispatch, review, synthesis)
- Feed into footer rendering and JSON meta blocks

### Step 4: Unit tests
- Record 3 invocations → summary correct
- Thread-safe concurrent recording
- Cost/token aggregation
- Error count tracking

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Footer stats
```bash
make build
./bin/dootsabha council "PONG"  # Check footer shows real stats
./bin/dootsabha council "PONG" --json | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['meta'])"
```

## Completion Criteria

1. Metrics collector tracks per-provider stats
2. Session-level aggregation correct
3. Footer stats populated from metrics
4. JSON meta blocks populated from metrics
5. `make ci` passes

## Commit

```
feat(metrics): add in-process metrics for provider invocations

- Thread-safe per-provider counters (duration, cost, tokens, errors)
- Session-level aggregation for footer stats
- Wired into engine dispatch/review/synthesis
- Powers JSON meta blocks and TTY footers
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.2 (footer stats), §7.1
5. Execute steps 1-4
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
