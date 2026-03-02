# Task 4.5: Full L5 Acceptance Suite

## Status: DONE

## Depends On
- Task 4.3 (edge cases), Task 4.4 (context file)

## Parallelizable With
- None (final P4 task — comprehensive)

## Problem

L5 acceptance tests validate the complete agent workflow: JSON parseable by tools, correct exit codes, no ANSI in pipe, required fields present, performance targets met. This is the final quality gate before Phase 5.

## PRD Reference
- §7.1 (Performance targets)
- §10 (Testing strategy summary — L5 layer)

## Also Read
- `testing-strategy.md §4` (L5 agent workflow tests — 7 test types)
- `testing-strategy.md §5` (Anti-hallucination rules)

## Files to Create
- `scripts/test-agent-workflow.sh` — Replace stub with full L5 suite
- `.claude/automations/test_full_acceptance.py` — L4 visual for all commands

## Execution Steps

### Step 1: Implement L5 test suite
Per testing-strategy.md §4, test these 7 areas:
1. JSON valid — all commands with `--json` piped to `python3 -m json.tool`
2. Exit codes — each code path returns expected code
3. No ANSI — piped output has zero escape sequences
4. Required fields — JSON outputs have all mandatory fields
5. Status — `status --json` has all provider entries
6. Errors — error commands produce styled messages (not stack traces)
7. Performance — startup <200ms, invocation overhead <100ms

### Step 2: Implement L4 visual test
- iTerm2-driver script that runs all major commands
- Screenshots: consult, council, status, review, config, errors
- Verify visual rendering is correct

### Step 3: Run full test pyramid
```bash
make ci              # L1
make test            # L2
make test-binary     # L3
make test-visual     # L4
make test-agent      # L5
```

### Step 4: Fix any failures discovered
- Each failure documented and fixed
- Re-run until all layers pass

## Verification

### Full pyramid
```bash
make test-all
```

### L5 specific
```bash
make test-agent
```

### L4 specific
```bash
make test-visual
```

## Completion Criteria

1. All 7 L5 test types pass
2. L4 visual tests capture screenshots for all commands
3. Full `make test-all` passes
4. Performance targets met (§7.1)
5. `make ci` passes

## Commit

```
feat(acceptance): add full L5 acceptance suite and L4 visual tests

- L5 agent workflow: JSON, exit codes, no ANSI, fields, status, errors, perf
- L4 visual: screenshots for consult, council, status, review, config
- Full make test-all passing
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §7.1, §10
5. Read testing-strategy.md §4, §5
6. Execute steps 1-4
7. Run verification (full pyramid)
8. Fill Visual Test Results section
9. **Change status to `DONE`**
10. Update `docs/PROGRESS.md`
11. Commit
