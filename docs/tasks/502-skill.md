# Task 5.3: Claude Code SKILL for दूतसभा

## Status: DONE

## Depends On
- Task 5.1 (README), Task 5.2 (default config)

## Parallelizable With
- Task 5.4 (build & release)

## Problem

Create a Claude Code SKILL that enables agents to discover and use दूतसभा. The SKILL teaches Claude Code how to invoke council/consult/review/status, interpret JSON output, and use exit codes for control flow.

## PRD Reference
- §3.2 (AI coding agents as consumers — JSON, exit codes)
- §6 (All commands — for SKILL examples)

## Files to Create
- `skill/SKILL.md` — Main skill definition (with frontmatter)
- `skill/references/command-reference.md` — All commands, flags, output schemas
- `skill/references/exit-codes.md` — Exit code patterns for control flow
- `skill/examples/council-deliberation.md` — Full council workflow example
- `skill/examples/review-refine.md` — Review and refine walkthrough

## Execution Steps

### Step 1: Write SKILL definition
- Name: `dootsabha`
- Description: Multi-agent council for AI coding agents
- Usage examples for each command
- JSON output parsing examples
- Exit code interpretation

### Step 2: Include agent workflow examples
- "Get multi-perspective answer": `dootsabha council --json "question" | jq .synthesis.content`
- "Get code review": `dootsabha review --json "review this function" | jq .review.content`
- "Check agent health": `dootsabha status --json | jq '.providers | to_entries[] | select(.value.healthy == false)'`
- "Quick single-agent": `dootsabha consult --json --agent claude "question" | jq .content`

### Step 3: Include error handling
- Exit code meanings and agent response patterns
- Timeout handling
- Partial result handling

### Step 4: Test with Claude Code
- Invoke SKILL in a Claude Code session
- Verify agent can use दूतसभा successfully

## Verification

### L5: Agent workflow
```bash
# Claude Code should be able to:
dootsabha consult --json "Say PONG" | python3 -m json.tool
dootsabha status --json | python3 -c "import json,sys; print(json.load(sys.stdin)['providers'])"
```

## Completion Criteria

1. SKILL teaches agents to use all दूतसभा commands
2. JSON parsing examples work
3. Exit code handling documented
4. SKILL tested in Claude Code session
5. `make ci` passes

## Commit

```
feat(skill): add Claude Code SKILL for agent discovery

- SKILL with council, consult, review, status examples
- JSON output parsing patterns
- Exit code interpretation for agent control flow
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §3.2, §6
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
