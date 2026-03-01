# Task 2.6: Refine Command (Sequential Review + Incorporation)

## Status: PENDING

## Depends On
- Phase 1 complete
- Task 2.4 (review command — reuses getProvider, providerColor, fmtTokens)

## Parallelizable With
- Task 2.5 (JSON output schema — independent pipeline)

## Problem

The refine command (`dootsabha refine` / `sanshodhan`) implements a sequential review + incorporation pipeline: author generates v1, each reviewer reviews in turn, and the author incorporates feedback after each review — producing progressively refined versions. Inspired by Karpathy's llm-council anonymized review pattern. This is fundamentally different from `council` (parallel deliberation + synthesis) and `review` (one-shot author → reviewer).

## PRD Reference
- §6.8 (Refine command — flags, pipeline, acceptance criteria FR-REF-*)
- §6.4 (Review — refine extends the review loop pattern)
- §8 (Terminal UX standards)

## Files to Create
- `internal/cli/refine.go` — Refine command implementation + TTY/JSON rendering
- `internal/cli/refine_test.go` — Unit tests with mock providers
- `.claude/automations/test_dootsabha_refine.py` — L4 visual tests (11 tests)

## Files to Modify
- `internal/cli/root.go` — Add `rootCmd.AddCommand(newRefineCmd())`

## Execution Steps

### Step 1: Implement refine command
- `refine` (alias: `sanshodhan`, `संशोधन`)
- Flags: `--author`/`--kartaa` [default: claude], `--reviewers`/`--pareekshak` [default: codex,gemini], `--anonymous`/`--gupt` [default: true]
- Sequential pipeline: author v1 → reviewer[0] reviews → author incorporates → v2 → reviewer[1] reviews → author incorporates → v3 ...

### Step 2: Construct prompts (anonymous vs named)
- Anonymous (default): no provider names in prompts
- Named (--anonymous=false): include provider names

### Step 3: Handle failures
- Author fails on v1 → fail-fast, exit 3
- Reviewer[i] fails → skip, continue to next (or output current version if last), exit 5
- Author fails on incorporation → output previous version with warning, exit 5

### Step 4: Render output
- TTY: version progression with timing per step + footer
- JSON: `{"versions": [...], "final": {...}, "meta": {...}}`
- Piped: clean text, no ANSI

### Step 5: Unit tests
- Full pipeline success (author + 2 reviewers → 3 versions)
- Author fails on v1 → fail-fast
- Reviewer fails → skipped, pipeline continues
- JSON output matches schema
- Anonymous vs named prompt content

### Step 6: Wire command
- Add `newRefineCmd()` to root.go init()

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Real refine
```bash
make build
./bin/dootsabha refine "Explain goroutines" --author claude --reviewers codex,gemini
./bin/dootsabha refine "Say PONG" --json | python3 -m json.tool
```

### L4: Visual verification
```bash
uv run .claude/automations/test_dootsabha_refine.py
```
Expected: all 11 tests pass (initial_generation, reviewer_feedback, incorporation, version_progression, screenshot_refine, author_failure_failfast, screenshot_failfast, reviewer_skip, screenshot_skip, no_ansi_piped, json_valid).
Screenshots: `dootsabha_refine_output_{ts}.png`, `dootsabha_refine_failfast_{ts}.png`, `dootsabha_refine_skip_{ts}.png`, `dootsabha_refine_piped_{ts}.png`

## Completion Criteria

1. Sequential pipeline works (author → review → incorporate → review → incorporate)
2. Bilingual aliases work (sanshodhan/संशोधन, --kartaa, --pareekshak, --gupt)
3. Anonymous mode anonymizes prompts by default
4. Author failure on v1 = fail fast
5. Reviewer failure = skip and continue
6. JSON output matches schema (versions/final/meta)
7. `make ci` passes

## Visual Test Results

**L4 Script:** `.claude/automations/test_dootsabha_refine.py`
**Date:** —
**Status:** PENDING (awaiting implementation)

| Test | Result | Details |
|------|--------|---------|
| initial_generation | — | |
| reviewer_feedback | — | |
| incorporation | — | |
| version_progression | — | |
| screenshot_refine | — | |
| author_failure_failfast | — | |
| screenshot_failfast | — | |
| reviewer_skip | — | |
| screenshot_skip | — | |
| no_ansi_piped | — | |
| json_valid | — | |

**Screenshots:** (pending implementation)
**Findings:** (pending implementation)

## Commit

```
feat(refine): add refine command with sequential review + incorporation pipeline

- refine (sanshodhan): sequential author → review → incorporate loop
- Anonymous review mode by default (Karpathy llm-council pattern)
- Fail-fast on author failure, skip failed reviewers
- Bilingual flag aliases (--kartaa, --pareekshak, --gupt)
- JSON output with versions array, final, and meta
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.8
5. Execute steps 1-6
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
