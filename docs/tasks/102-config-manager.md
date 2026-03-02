# Task 1.3: Config Manager (Viper)

## Status: DONE

## Depends On
- Task 1.1 (project scaffold)

## Parallelizable With
- Task 1.2 (render context)

## Problem

दूतसभा needs a config manager that merges: defaults → YAML file → env vars (`DOOTSABHA_*`) → CLI flags. Must support provider configuration, key redaction, and forward-compatible unknown keys.

## PRD Reference
- §5.2 (Config Manager component)
- §6.6 (Config command — show, --commented, --json, --reveal, redaction)

## Files to Create
- `internal/core/config.go` — Viper-based config loader
- `internal/core/config_test.go` — Unit tests
- `configs/default.yaml` — Default config with all providers

## Files to Modify
- `configs/default.yaml` — Expand with full provider config (if stub exists from 1.1)

## Execution Steps

### Step 1: Design config schema
```yaml
providers:
  claude:
    binary: claude
    model: sonnet-4-6
    flags: ["-p", "--output-format", "json", "--dangerously-skip-permissions"]
  codex:
    binary: codex
    model: gpt-5.3-codex
    flags: ["exec", "--json", "--sandbox", "danger-full-access"]
  gemini:
    binary: gemini
    model: gemini-3-pro
    flags: ["--yolo", "--output-format", "json"]
council:
  chair: claude
  parallel: true
  rounds: 1
timeout: 5m
session_timeout: 30m
```

### Step 2: Implement config loader
- Viper: `SetConfigFile`, `SetConfigType("yaml")`, `AutomaticEnv()` with `DOOTSABHA_` prefix
- Merge order: defaults → file → env → flags
- Unknown keys silently ignored (forward-compatible)

### Step 3: Implement key redaction
- Keys matching `*token*`, `*key*`, `*secret*` → `[REDACTED]` in show output
- `--reveal` flag bypasses redaction

### Step 4: Unit tests
- Merge order verified (flag > env > file > default)
- Redaction works for sensitive keys
- Unknown keys don't error
- `DOOTSABHA_PROVIDERS_CLAUDE_MODEL=opus-4-6` overrides `providers.claude.model`

## Verification

### L1: Unit tests
```bash
make test
```

### L2: Config merge
```bash
go test -run TestConfigMerge -v ./internal/core/...
```

## Completion Criteria

1. Config loads from YAML + env + flags with correct precedence
2. `DOOTSABHA_*` env vars override config values
3. Sensitive keys are redacted by default
4. Unknown keys ignored without error
5. `make ci` passes

## Commit

```
feat(config): add viper-based config manager with redaction

- YAML + env (DOOTSABHA_*) + flags merge with correct precedence
- Key redaction for *token*/*key*/*secret* patterns
- Forward-compatible: unknown keys silently ignored
- Default config with all 3 providers
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.2, §6.6
5. Execute steps 1-4
6. Run verification (L1 → L2)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
