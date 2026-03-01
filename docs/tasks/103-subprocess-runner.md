# Task 1.4: Subprocess Runner

## Status: DONE

## Depends On
- Task 1.1 (project scaffold)

## Parallelizable With
- Task 1.2 (render context), Task 1.3 (config manager)

## Problem

दूतसभा executes AI CLIs as subprocesses. The runner must handle: context-aware execution, process group isolation (Setpgid), stdout/stderr splitting, timeout enforcement, orphan reaping, and CLAUDECODE env var unsetting.

## PRD Reference
- §5.2 (Subprocess Runner — Setpgid, orphan reaper, grace period)
- §7.2 (Reliability: SIGTERM → 5s grace → SIGKILL, Ctrl+C shutdown)
- §4.1 (CLAUDECODE env var gotcha)

## Files to Create
- `internal/core/subprocess.go` — Subprocess runner with process group management
- `internal/core/subprocess_test.go` — Unit tests

## Execution Steps

### Step 1: Read spike findings
1. Read `_spikes/subprocess/README.md` (Spike 0.5 findings)
2. Read `_spikes/pty-pipe/README.md` (Spike 0.8 findings)

### Step 2: Implement SubprocessRunner
- `Run(ctx context.Context, binary string, args []string, env []string) (Result, error)`
- `Result{Stdout, Stderr []byte, ExitCode int, Duration time.Duration}`
- Set `SysProcAttr.Setpgid = true` for process group isolation
- Remove `CLAUDECODE` from env when running claude
- Context cancellation → SIGTERM to process group → 5s grace → SIGKILL

### Step 3: Implement orphan reaper
- Background goroutine that checks for orphaned child processes
- Kill process group after grace period if parent pipe breaks
- Log reaper actions at debug level

### Step 4: Unit tests
- Normal execution (echo command)
- Timeout enforcement (context with deadline)
- Exit code capture
- Stdout/stderr separation
- CLAUDECODE env var removal
- Context cancellation cleanup

## Verification

### L1: Unit tests
```bash
make test
```

### L2: Subprocess behavior
```bash
go test -run TestSubprocessRunner -v ./internal/core/...
```

### L3: Real CLI (if available)
```bash
# Quick smoke with mock provider
DOOTSABHA_CLAUDE_BIN=testdata/mock-providers/mock-claude go test -run TestMockProvider -v ./internal/core/...
```

## Completion Criteria

1. Subprocess executes with context-aware timeout
2. Setpgid process group isolation works
3. CLAUDECODE env var removed for claude subprocess
4. Context cancellation kills process group cleanly
5. Orphan reaper goroutine implemented
6. `make ci` passes

## Commit

```
feat(subprocess): add runner with Setpgid, reaper, and env sanitization

- os/exec wrapper with context-aware timeout
- Process group isolation via Setpgid
- CLAUDECODE env var removal for claude subprocess
- Orphan reaper goroutine with grace period
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.2, §7.2, §4.1
5. Read spike findings (0.5, 0.8)
6. Execute steps 1-4
7. Run verification (L1 → L2 → L3)
8. **Change status to `DONE`**
9. Update `docs/PROGRESS.md`
10. Commit
