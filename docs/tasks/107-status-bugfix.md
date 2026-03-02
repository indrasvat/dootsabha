# Task 1.8: Status Command Bugfix (version, dot column, models)

## Status: DONE

## Depends On
- Task 1.7 (CLI wiring — status command exists)

## Parallelizable With
- None

## Problem

Three visual/data bugs in `dootsabha status`:
1. **Version parsing** — Claude shows "Code)" instead of "2.1.63". `parseVersion()` takes last whitespace token from "2.1.63 (Claude Code)".
2. **Dot column waste** — The colored `●` occupies its own headerless column, wasting ~15 chars of table width.
3. **Missing models** — Codex and Gemini show empty MODEL column because viper defaults had `model: ""`.

## PRD Reference
- §6.5 (Status command — health table)
- §8.2 (Color palette — provider dots)

## Files to Modify
- `internal/providers/claude.go` — `parseVersion()` heuristic
- `internal/cli/status.go` — `renderStatusTable()` column layout
- `internal/core/config.go` — viper defaults for codex/gemini models
- `internal/providers/codex.go` — `providerConfig()` fallback model
- `internal/providers/gemini.go` — `providerConfig()` fallback model
- `configs/default.yaml` — model fields
- `internal/providers/claude_test.go` — fix mock stdout, add version parsing tests

## Files to Create
- `.claude/automations/test_dootsabha_status.py` — L4 visual test

## Execution Steps

### Step 1: Fix parseVersion()
- Scan tokens for first one starting with a digit
- Strip surrounding parens: `strings.Trim(tok, "()")`
- Handles: "2.1.63 (Claude Code)", "codex-cli 0.106.0", "0.30.0"

### Step 2: Merge dot into provider column
- Remove empty `""` header from Headers()
- Prepend `dot + " " + name` in Row()
- 5 columns instead of 6

### Step 3: Set default models
- Viper defaults: codex → "gpt-5.3-codex", gemini → "gemini-3-pro"
- Provider fallbacks: same values
- configs/default.yaml: match

### Step 4: Update tests
- Mock stdout → "2.1.63 (Claude Code)\n" (matches real CLI)
- Add TestVersionParsing table test (claude, codex, gemini, bare, fallback)

### Step 5: L4 visual test
- Create iTerm2-driver script per testing-strategy.md §2
- Verify: versions correct, dot merged, models populated, colors rendered

## Verification

### L1: Unit tests
```bash
make ci
```

### L3: Real binary
```bash
make build
./bin/dootsabha status
./bin/dootsabha status --json | python3 -m json.tool
./bin/dootsabha status | cat  # piped mode, no ANSI
```

### L4: Visual
```bash
uv run .claude/automations/test_dootsabha_status.py
```

## Completion Criteria

1. Claude version shows "2.1.63" (not "Code)")
2. No separate empty dot column — dot merged into PROVIDER
3. All 3 providers show model names
4. `make ci` passes
5. JSON output has correct version/model for all providers
6. Piped output degrades cleanly (no ANSI)
7. L4 visual test passes with screenshots

## Commit

```
fix(status): version parsing, dot column merge, default provider models

- parseVersion: find first digit-starting token, strip parens
- Merge colored dot into PROVIDER column (5 cols, not 6)
- Set codex default model to gpt-5.3-codex, gemini to gemini-3-pro
- Update test mock to match real claude --version output
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Execute steps 1-5
5. Run verification (L1 → L3 → L4)
6. Fill Visual Test Results section
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit

## Visual Test Results

**L4 Script:** `.claude/automations/test_dootsabha_status.py`
**Date:** 2026-03-01
**Status:** PASS (6/7 PASS, 0 FAIL, 1 UNVERIFIED)

| Test | Result | Details |
|------|--------|---------|
| version_correctness | PASS | Found semver versions: 2.1.63, 0.106.0, 0.30.0 |
| dot_merged | UNVERIFIED | Lipgloss box-drawing chars prevented automated column detection; screenshot confirms dot is in PROVIDER column |
| screenshot_healthy | PASS | `dootsabha_status_healthy.png` captured |
| models_populated | PASS | All models present: sonnet-4-6, gpt-5.3-codex, gemini-3-pro |
| table_layout | PASS | All providers and columns present |
| no_ansi_piped | PASS | No ANSI codes in piped output |
| json_valid | PASS | Valid JSON with correct providers, versions, models |

**Screenshots:**
- `dootsabha_status_healthy.png` — Status table with colored dots, correct versions, all models populated
- `dootsabha_status_piped.png` — Piped output with `*` dots, tab-separated, no ANSI

**Findings:** All 3 bugs fixed. The dot_merged test is UNVERIFIED programmatically (lipgloss border chars don't match the pipe-based heuristic) but visually confirmed via screenshot.
