# Task 2.6: JSON Output for All Modes

## Status: PENDING

## Depends On
- Task 2.3 (synthesis), Task 2.4 (review command)

## Parallelizable With
- None (final P2 integration task)

## Problem

Ensure `--json` flag produces valid, machine-consumable JSON for all command modes: council, consult, review, status, config. All JSON must include `meta.schema_version`, cost fields nullable, and zero ANSI codes.

## PRD Reference
- §6.2 (Council JSON schema — full example)
- §6.3 (Consult JSON schema)
- §6.4 (Review JSON — author + review sections)
- §6.5 (Status JSON schema)
- §6.6 (Config JSON)

## Files to Create
- `internal/output/schemas.go` — JSON output struct types for all commands
- `internal/output/schemas_test.go` — Validation tests

## Files to Modify
- All command files to ensure `--json` uses consistent schema types

## Execution Steps

### Step 1: Define JSON schema types
- `CouncilOutput` — dispatch, reviews, synthesis, meta
- `ConsultOutput` — provider, model, content, duration, cost, tokens
- `ReviewOutput` — author, review, meta
- `StatusOutput` — version, providers, plugins, extensions
- `ConfigOutput` — resolved config with redaction
- `MetaBlock` — schema_version, session_id, strategy, duration, cost, tokens, providers

### Step 2: Ensure consistency
- All outputs include `meta.schema_version: 1`
- Cost/token fields are `null` when provider doesn't report them (not `0`)
- `session_id` format: `ds_{random5}`

### Step 3: Validate all JSON paths
- Run each command with `--json` and pipe through `python3 -m json.tool`
- Verify no ANSI codes in JSON output
- Verify cost fields are nullable

### Step 4: Unit tests
- Each schema type marshals to valid JSON
- Nullable fields serialize as `null`
- schema_version present in all outputs
- No ANSI in any JSON output

## Verification

### L1: Unit tests
```bash
make test
```

### L3: All commands with --json
```bash
make build
./bin/dootsabha consult --json "PONG" | python3 -m json.tool
./bin/dootsabha council --json "PONG" | python3 -m json.tool
./bin/dootsabha review --json "PONG" | python3 -m json.tool
./bin/dootsabha status --json | python3 -m json.tool
./bin/dootsabha config show --json | python3 -m json.tool
# No ANSI in any output
for cmd in consult council review status; do
  ./bin/dootsabha $cmd --json "PONG" | grep -cP '\x1b\[' | grep -q '^0$'
done
```

## Completion Criteria

1. All commands produce valid JSON with `--json`
2. `meta.schema_version: 1` in all outputs
3. Cost/token fields are `null` when not available
4. Zero ANSI codes in JSON output
5. `make ci` passes

## Commit

```
feat(json): ensure consistent JSON output across all commands

- JSON schema types for council, consult, review, status, config
- meta.schema_version: 1 in all outputs
- Nullable cost/token fields
- ANSI-free JSON output validation
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.2-§6.6 (JSON schemas only)
5. Execute steps 1-4
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
