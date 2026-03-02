# Task 2.4: Review Subcommand (Author + Reviewer)

## Status: DONE

## Depends On
- Phase 1 complete

## Parallelizable With
- Task 2.1 (parallel dispatch — independent pipeline)

## Problem

The review command (`dootsabha review` / `sameeksha`) is a 2-step pipeline: author agent produces output, then reviewer agent reviews it. Simpler than council — no parallel dispatch, no synthesis.

## PRD Reference
- §6.4 (Review command — flags, pipeline, acceptance criteria FR-REV-*)
- §6.3 (Consult — review builds on same provider invocation patterns)

## Files to Create
- `internal/cli/review.go` — Review command implementation
- `internal/cli/review_test.go` — Unit tests
- `.claude/automations/test_dootsabha_review.py` — L4 visual tests (all 8 tests verify review command)

## Execution Steps

### Step 1: Implement review command
- `review` (alias: `sameeksha`)
- Flags: `--author`/`--kartaa` [default: codex], `--reviewer`/`--pareekshak` [default: claude]
- Two-step: invoke author → invoke reviewer with author's output

### Step 2: Construct review prompt
- Reviewer prompt: "Review the following output from {author}. Identify strengths, weaknesses, errors. Be specific.\n\n{author output}"

### Step 3: Handle failures
- If author fails → fail fast, don't invoke reviewer (FR-REV-05)
- If reviewer fails → return author output with error

### Step 4: Render output
- TTY: show author output section, then reviewer section, styled
- JSON: `{"author": {...}, "review": {...}, "meta": {...}}`
- Piped: clean text, no ANSI

### Step 5: Unit tests
- Author + reviewer both succeed
- Author fails → reviewer not invoked
- Reviewer fails → author output returned with error
- JSON output matches schema

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Real review
```bash
make build
./bin/dootsabha review "What is a goroutine?" --author codex --reviewer claude
./bin/dootsabha review "PONG" --json | python3 -m json.tool
```

### L4: Visual verification
```bash
uv run .claude/automations/test_dootsabha_review.py
```
Expected: all 8 tests pass (author_section, reviewer_section, both_agents_labeled, screenshot_review, author_failure_failfast, screenshot_failfast, no_ansi_piped, json_valid).
Screenshots: `dootsabha_review_output_{ts}.png`, `dootsabha_review_failfast_{ts}.png`, `dootsabha_review_piped_{ts}.png`

## Completion Criteria

1. Two-step pipeline works (author → reviewer)
2. Bilingual aliases work
3. Author failure = fail fast
4. JSON output correct
5. `make ci` passes

## Visual Test Results

**L4 Script:** `.claude/automations/test_dootsabha_review.py`
**Date:** —
**Status:** PENDING (awaiting Phase 2 implementation)

| Test | Result | Details |
|------|--------|---------|
| author_section | — | |
| reviewer_section | — | |
| both_agents_labeled | — | |
| screenshot_review | — | |
| author_failure_failfast | — | |
| screenshot_failfast | — | |
| no_ansi_piped | — | |
| json_valid | — | |

**Screenshots:** (pending implementation)
**Findings:** (pending implementation)

## Commit

```
feat(review): add review command with author + reviewer pipeline

- review (sameeksha): 2-step author → reviewer pipeline
- Fail-fast on author failure
- Bilingual flag aliases (--kartaa, --pareekshak)
- JSON output with both author and review sections
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.4
5. Execute steps 1-5
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
