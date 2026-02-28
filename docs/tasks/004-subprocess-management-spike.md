# Task 0.5: Subprocess Management Spike

## Status: PENDING

## Depends On
- None

## Parallelizable With
- All other spikes (0.1–0.4, 0.6–0.8)

## Problem

दूतसभा spawns 3+ agent CLIs as subprocesses in parallel. We must validate: errgroup fan-out/fan-in, context cancellation propagation, process group cleanup (Setpgid + SIGTERM/SIGKILL), orphan reaper behavior, and macOS SIP interaction with process groups.

## PRD Reference
- §5.2 (Subprocess Runner — Setpgid, orphan reaper, grace period)
- §7.2 (Ctrl+C clean shutdown: SIGTERM → 5s grace → SIGKILL)
- §11 (Risk: macOS SIP + process group mgmt, orphaned processes)

## Files to Create
- `_spikes/subprocess/main.go` — Spike program
- `_spikes/subprocess/README.md` — Findings doc

## Execution Steps

### Step 1: Initialize spike module
- **No top-level `go.mod` exists yet** (created in Task 1.1). Each spike is a standalone module.
- `mkdir -p _spikes/subprocess && cd _spikes/subprocess`
- `go mod init dootsabha-spike/subprocess`
- `go get golang.org/x/sync`

### Step 2: Read context
1. Read PRD §5.2 (subprocess runner component)
2. Read PRD §7.2 (reliability: Ctrl+C behavior)

### Step 3: Write spike program
- Use `errgroup.Group` with context to fan-out 3 `sleep` commands
- Set `SysProcAttr.Setpgid = true` for process group isolation
- Implement context cancellation → SIGTERM to process group → grace period → SIGKILL
- Implement orphan reaper goroutine

### Step 4: Test scenarios
- Normal completion (all 3 succeed)
- One process fails → context cancelled → others killed
- Ctrl+C during execution → clean shutdown
- Kill parent → verify children are reaped (no orphans via `ps aux | grep`)
- macOS-specific: verify Setpgid works under SIP

### Step 5: Document findings
- errgroup + context cancellation patterns that work
- Process group cleanup timing (grace period needed?)
- Orphan reaper effectiveness
- macOS SIP behavior with Setpgid

## Verification

### L1: Spike runs
```bash
cd _spikes/subprocess && go run main.go
```

### L3: Process cleanup
```bash
# Run spike, Ctrl+C mid-execution, verify no orphans
go run main.go &
sleep 1 && kill -INT %1
ps aux | grep sleep  # Should find NO orphaned sleep processes
```

## Completion Criteria

1. errgroup fan-out/fan-in works with 3+ concurrent subprocesses
2. Context cancellation kills all children via process group
3. No orphaned processes after Ctrl+C or parent crash
4. macOS SIP behavior documented
5. README.md with recommended patterns for production

## Commit

```
spike(subprocess): validate errgroup, Setpgid, and orphan reaper

- errgroup fan-out with 3 concurrent subprocesses
- Process group cleanup via SIGTERM + grace period + SIGKILL
- Orphan reaper goroutine verified on macOS
```

## Session Protocol

1. Read CLAUDE.md — **skip if it doesn't exist yet (created in Task 1.1)**
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.2, §7.2
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
