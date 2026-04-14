# Task 1.9: Align Code with Architecture Doc

## Status: DONE

## Depends On
- Task 1.8 (Status bugfix — models now populated)

## Parallelizable With
- None

## Problem

Cross-reference audit of `docs/dootsabha-architecture.html` vs code revealed 3 discrepancies:

1. **Claude model name** — Architecture uses `opus-4-6` (short form). Code had `claude-opus-4-6` (full API ID). Architecture §4.1 confirms short names work via `--model opus-4-6`.
2. **Codex missing `--ephemeral`** — Architecture §4.1 specifies `--ephemeral` flag for codex. Code omitted it. Confirmed working via `codex exec --ephemeral` test.
3. **Gemini `--yolo` bug** — Architecture §4.1 warns: "Gotcha: `--yolo` shorthand has known bug (#13561). Always use `--approval-mode yolo`." Code used `--yolo`.

## PRD Reference
- §3.1 (Provider configs)
- §8.1 (Status output — model display)

## Files Modified
- `internal/core/config.go` — viper defaults (model, codex flags, gemini flags)
- `internal/core/config_test.go` — update default assertions and YAML fixtures
- `internal/providers/claude.go` — fallback model
- `internal/providers/codex.go` — fallback flags, doc comment
- `internal/providers/gemini.go` — fallback flags, doc comments
- `configs/default.yaml` — all 3 provider configs
- `testdata/mock-providers/mock-codex` — handle `--ephemeral` flag
- `testdata/mock-providers/mock-gemini` — handle `--approval-mode` flag
- `scripts/test-binary.sh` — gemini test uses `--approval-mode yolo`
- `.claude/automations/test_dootsabha_status.py` — expected model names
- `docs/tasks/107-status-bugfix.md` — visual test results
- `docs/PROGRESS.md` — add task 1.9

## Execution Steps

### Step 1: Claude model `claude-opus-4-6` → `opus-4-6`
- Updated viper default, provider fallback, configs/default.yaml
- Updated config_test.go assertions and YAML fixtures
- L4 test expected models updated

### Step 2: Add `--ephemeral` to codex flags
- Updated viper default, provider fallback, configs/default.yaml
- Updated mock-codex to handle the flag
- Updated doc comment in codex.go Invoke()

### Step 3: Gemini `--yolo` → `--approval-mode yolo`
- Updated viper default, provider fallback, configs/default.yaml
- Updated mock-gemini to handle `--approval-mode` flag
- Updated test-binary.sh to use new flag format
- Updated doc comments in gemini.go

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
bash scripts/test-binary.sh
```

## Completion Criteria

1. `dootsabha status` shows `opus-4-6` (not `claude-opus-4-6`)
2. JSON output has correct model for all 3 providers
3. Codex flags include `--ephemeral`
4. Gemini flags use `--approval-mode yolo` (not `--yolo`)
5. `make ci` passes
6. L3 smoke tests pass (8/8)

## Commit

```
fix: align provider configs with architecture doc

- Claude model: claude-opus-4-6 → opus-4-6 (short form per arch §4.1)
- Codex: add --ephemeral flag (confirmed working, per arch §4.1)
- Gemini: --yolo → --approval-mode yolo (bug #13561, per arch §4.1)
- Update mocks, tests, and config fixtures to match
```
