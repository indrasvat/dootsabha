# Task 2.5: Retry Logic + Error Classification

## Status: PENDING

## Depends On
- Task 2.1 (parallel dispatch)

## Parallelizable With
- Task 2.4 (review command)

## Problem

Provider invocations can fail transiently (rate limits, timeouts, OOM) or permanently (auth, CLI not found). We need typed error classification with appropriate retry behavior using avast/retry-go.

## PRD Reference
- §7.2 (Reliability — transient vs permanent matchers, retry strategy, Ctrl+C shutdown)
- §6.1 (Exit codes: 3=provider, 4=timeout, 5=partial)

## Files to Create
- `internal/core/retry.go` — Retry logic with error classification
- `internal/core/errors.go` — Error types (TransientError, PermanentError, TimeoutError)
- `internal/core/retry_test.go` — Unit tests

## Files to Modify
- `internal/core/engine.go` — Wire retry into dispatch
- `internal/core/subprocess.go` — Classify subprocess errors

## Execution Steps

### Step 1: Define error types
- `TransientError` — retryable (rate limit, timeout, OOM, connection refused)
- `PermanentError` — never retry (auth, CLI not found, usage error)
- `TimeoutError` — wraps context.DeadlineExceeded

### Step 2: Implement classifiers
- Exit code 1 + stderr matches: "rate limit", "429", "timeout", "EAGAIN", "connection refused" → Transient
- Exit code 137 (OOM killed) → Transient
- Exit code 127 (CLI not found) → Permanent
- Exit code 2 (usage error) → Permanent
- Stderr matches: "auth", "token expired", "permission denied", "model not found" → Permanent
- Default (unknown exit code) → Permanent (fail-safe)

### Step 3: Implement retry with avast/retry-go
- `retry.Do(fn, retry.Attempts(3), retry.Delay(1s), retry.DelayType(retry.BackOffDelay))`
- Jitter: ±50% on delay
- Retries share per-agent `--timeout` budget (no reset)
- Only retry TransientError; PermanentError fails immediately

### Step 4: Unit tests
- Transient error → retried 2x → succeeds on 3rd → OK
- Transient error → all 3 retries fail → error
- Permanent error → no retry, immediate fail
- Timeout budget shared across retries
- Error classification for each matcher

## Verification

### L1: Unit tests
```bash
make test
```

### L2: Error classification
```bash
go test -run TestErrorClassification -v ./internal/core/...
go test -run TestRetry -v ./internal/core/...
```

## Completion Criteria

1. Transient vs permanent classification works for all matchers
2. Retry with exponential backoff + jitter
3. Retries share timeout budget (no reset)
4. Permanent errors fail immediately
5. Unknown exit codes default to permanent (fail-safe)
6. `make ci` passes

## Commit

```
feat(retry): add typed error classification and retry logic

- TransientError/PermanentError/TimeoutError types
- Stderr/exit code matchers per PRD §7.2
- avast/retry-go with exponential backoff + jitter
- Retries share per-agent timeout budget
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §7.2, §6.1
5. Execute steps 1-4
6. Run verification (L1 → L2)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
