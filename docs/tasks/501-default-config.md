# Task 5.2: Default Config + Embedded Docs

## Status: DONE

## Depends On
- Phase 4 complete

## Parallelizable With
- Task 5.1 (README)

## Problem

Ship a well-documented default config that users can copy and customize. The `config show --commented` output should serve as inline documentation for all config options.

## PRD Reference
- §6.6 (Config command — show, --commented)
- §5.2 (Config Manager — defaults, schema)

## Files to Modify
- `configs/default.yaml` — Complete with all options + comments
- `internal/core/config.go` — Embed default config, support --commented

## Execution Steps

### Step 1: Write comprehensive default.yaml
- All provider configs with comments explaining each option
- Council defaults (chair, parallel, rounds)
- Timeout defaults (agent, session)
- Logging defaults
- All values documented with inline YAML comments

### Step 2: Implement --commented output
- `config show --commented` includes inline explanations
- Shows merge sources (file, env, flag) next to each value

### Step 3: Embed in binary
- Use `//go:embed configs/default.yaml` for zero-dependency defaults
- Falls back to embedded when no config file found

### Step 4: Test config init
- `dootsabha config show` works without any config file
- `dootsabha config show --commented` shows all explanations
- `dootsabha config show --json` produces valid JSON

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Config output
```bash
make build
./bin/dootsabha config show
./bin/dootsabha config show --commented
./bin/dootsabha config show --json | python3 -m json.tool
```

## Completion Criteria

1. Default config covers all options with comments
2. `--commented` output is helpful documentation
3. Embedded config works without external file
4. JSON config output is valid
5. `make ci` passes

## Commit

```
feat(config): add comprehensive default config with embedded docs

- Complete default.yaml with all options documented
- config show --commented with inline explanations
- go:embed for zero-dependency defaults
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.6, §5.2
5. Execute steps 1-4
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
