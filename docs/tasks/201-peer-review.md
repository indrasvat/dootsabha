# Task 2.2: Peer Review Stage

## Status: PENDING

## Depends On
- Task 2.1 (parallel dispatch)

## Parallelizable With
- None (builds on dispatch results)

## Problem

Council Stage 2: each agent reviews the other agents' outputs. Must truncate inputs to 32KB per agent output, construct cross-review prompts, and run reviews in parallel.

## PRD Reference
- §6.2 (Council command — peer review stage, 32KB cap, cross-review prompt)
- §6.2 acceptance criteria FR-COU-03, FR-COU-11, FR-COU-12

## Files to Create
- `internal/core/review.go` — Peer review logic
- `internal/core/review_test.go` — Unit tests

## Files to Modify
- `internal/core/engine.go` — Add peer review stage to pipeline

## Execution Steps

### Step 1: Implement peer review
- For each agent: construct prompt with other agents' outputs
- Prompt template: "Review the following outputs from {agents}. Identify strengths, weaknesses, errors. Be specific.\n\n{agent1 output}\n\n{agent2 output}"
- Truncate each agent output to 32KB before inclusion

### Step 2: Run reviews in parallel
- Use errgroup for parallel review invocations
- Each agent reviews all OTHER agents (not itself)

### Step 3: Handle edge cases
- 2 agents: each reviews 1 output (no truncation issue)
- 1 agent: skip peer review stage entirely
- Agent that failed dispatch: excluded from review

### Step 4: Progress rendering
- Show review progress on stderr: `claude reviewing codex + gemini ... ✓`

### Step 5: Unit tests
- 3 agents: 3 reviews, each reviewing 2 outputs
- 2 agents: 2 reviews, each reviewing 1 output
- 1 agent: peer review skipped
- Truncation at 32KB verified
- Failed dispatch agent excluded

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Real council
```bash
make build
./bin/dootsabha council "What is a goroutine?" --agents claude,codex,gemini
# Should show all 3 stages
```

## Completion Criteria

1. Each agent reviews other agents' outputs
2. Input truncated to 32KB per agent output
3. Skips peer review with <2 agents
4. Reviews run in parallel
5. `make ci` passes

## Commit

```
feat(council): add peer review stage with 32KB truncation

- Each agent reviews other agents' dispatch outputs
- Input truncation to 32KB per agent output
- Parallel review invocations via errgroup
- Skips review stage with <2 agents
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.2 (peer review section)
5. Execute steps 1-5
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
