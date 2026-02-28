# Task 2.1: Parallel Dispatch (errgroup + progress)

## Status: PENDING

## Depends On
- Phase 1 complete (all providers and CLI wiring)

## Parallelizable With
- Task 2.4 (review command — independent pipeline)

## Problem

The council command's Stage 1 dispatches prompts to all configured agents in parallel using errgroup. Must show progress bars (stderr) during dispatch and collect results for peer review stage.

## PRD Reference
- §6.2 (Council command — 3-stage pipeline, dispatch stage)
- §5.5 (Key decision: errgroup for parallel dispatch)
- §7.2 (Partial results — continue with remaining agents, exit code 5)

## Files to Create
- `internal/core/engine.go` — Session manager with dispatch stage
- `internal/core/engine_test.go` — Unit tests with mock providers

## Files to Modify
- `internal/cli/root.go` — Add `council` subcommand registration

## Execution Steps

### Step 1: Implement dispatch engine
- `Engine.Dispatch(ctx, prompt, agents) ([]ProviderResult, error)`
- Use `errgroup.Group` with context for parallel invocation
- Collect results in thread-safe slice
- Respect `--parallel=false` for sequential mode

### Step 2: Implement progress rendering
- huh spinner on stderr showing agent status during dispatch
- Per-agent progress: `● claude ████████ 3.1s ✓`
- Update in real-time as agents complete

### Step 3: Handle partial failures
- If one agent fails permanently, continue with remaining
- If one agent times out, retry once, then degrade
- Return partial results + error for exit code 5 determination

### Step 4: Agent count cap
- Enforce max 5 agents (O(n²) peer review scaling)
- Error if `--agents` specifies >5

### Step 5: Unit tests
- 3 agents all succeed → 3 results
- 1 agent fails → 2 results + partial error
- Context cancellation → all agents killed
- Sequential mode (--parallel=false)
- >5 agents → error

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Mock providers
```bash
make build
./bin/dootsabha council "Say PONG" --agents claude,codex,gemini
```

## Completion Criteria

1. Parallel dispatch with errgroup works
2. Progress rendering on stderr
3. Partial failure → remaining agents continue
4. Max 5 agents enforced
5. `make ci` passes

## Commit

```
feat(engine): add parallel dispatch with errgroup and progress

- errgroup fan-out to all configured agents
- huh spinner progress on stderr
- Partial failure handling (continue + exit code 5)
- Max 5 agents cap for O(n²) peer review
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.2, §7.2
5. Execute steps 1-5
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
