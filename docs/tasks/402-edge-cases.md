# Task 4.3: Edge Cases & Error Paths

## Status: PENDING

## Depends On
- Task 4.1 (structured logging)

## Parallelizable With
- Task 4.4 (context file)

## Problem

Every error path must produce a helpful, styled message — not a stack trace. This task systematically covers: missing CLIs, auth failures, timeout expiry, SIGPIPE, Ctrl+C, plugin crashes, and all exit code paths.

## PRD Reference
- §6.1 (Exit codes and precedence matrix: 2 > 4 > 3 > 5 > 1 > 0)
- §7.2 (Reliability: transient/permanent, Ctrl+C, plugin crash)
- §7.4 (Security: config permissions warning)
- §8.1 (SIGPIPE handling)

## Files to Create
- `internal/core/errors_test.go` — Comprehensive error path tests (expand from 2.5)
- `.claude/automations/test_error_paths.py` — L4 visual test for error rendering

## Files to Modify
- `internal/cli/root.go` — Ensure all error paths use styled error output
- `internal/core/engine.go` — Exit code precedence logic
- `internal/core/subprocess.go` — SIGPIPE handler

## Execution Steps

### Step 1: Implement exit code precedence
- When multiple codes apply, return highest-precedence: 2 > 4 > 3 > 5 > 1 > 0
- Example: timeout (4) + partial (5) → exit 4

### Step 2: Style all error messages
- Missing CLI: "claude not found — install from https://..."
- Auth failure: "claude auth failed — run: claude auth login"
- Timeout: "codex timed out after 5m — try increasing --timeout"
- SIGPIPE: exit 0 silently (no "broken pipe" spam)

### Step 3: Ctrl+C shutdown
- Catch SIGINT/SIGTERM
- Kill child process groups (SIGTERM → 5s → SIGKILL)
- Print summary: "Interrupted. Killed 2 agents. Partial results not saved."
- Non-zero exit

### Step 4: Plugin crash recovery
- Plugin process dies → remove from registry
- Styled error: "claude plugin crashed — falling back to direct invocation"

### Step 5: L4 visual test
- Create iTerm2-driver script testing error rendering
- Verify: error messages are styled, not stack traces

### Step 6: Comprehensive tests
- Each exit code path tested
- Precedence matrix tested
- SIGPIPE test (pipe to head)
- Ctrl+C test (signal during council)

## Verification

### L1: Unit tests
```bash
make ci
```

### L3: Error paths
```bash
make build
./bin/dootsabha consult --agent nonexistent "test"; echo "exit: $?"  # exit 3
./bin/dootsabha consult --timeout 1ms "test"; echo "exit: $?"       # exit 4
./bin/dootsabha council "test" | head -1; echo "exit: $?"           # SIGPIPE → 0
./bin/dootsabha --badFlag 2>/dev/null; echo "exit: $?"              # exit 2
```

### L4: Visual
```bash
make test-visual
```

## Completion Criteria

1. Exit code precedence matrix correct
2. All error messages are styled and actionable
3. SIGPIPE handled (exit 0, no spam)
4. Ctrl+C clean shutdown verified
5. Plugin crash recovery works
6. `make ci` passes

## Commit

```
feat(errors): add styled error paths, exit code precedence, SIGPIPE

- Exit code precedence: 2 > 4 > 3 > 5 > 1 > 0
- Styled actionable error messages for all failure modes
- SIGPIPE handled gracefully (exit 0)
- Ctrl+C: SIGTERM + grace period + SIGKILL
- Plugin crash recovery with fallback
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.1, §7.2, §8.1
5. Execute steps 1-6
6. Run verification (L1 → L3 → L4)
7. Fill Visual Test Results section
8. **Change status to `DONE`**
9. Update `docs/PROGRESS.md`
10. Commit
