# Task 3.6: Plugin Command (vistaarak list/inspect)

## Status: DONE

## Depends On
- Task 3.5 (extension discovery)

## Parallelizable With
- None (final P3 task)

## Problem

Users need to discover what plugins and extensions are available. The `dootsabha plugin` (`vistaarak`) command lists all gRPC plugins and PATH extensions with health status, and inspects individual plugin capabilities.

## PRD Reference
- §6.7 (Plugin command — list, inspect, acceptance criteria FR-PLG-*)

## Files to Create
- `internal/cli/plugin_cmd.go` — Plugin command implementation
- `internal/cli/plugin_cmd_test.go` — Unit tests

## Execution Steps

### Step 1: Implement plugin list
- `plugin list` (alias: `vistaarak soochi`)
- List all gRPC plugins from registry + PATH extensions
- Show: name, type, health status, version
- TTY: styled table with health indicators
- JSON: structured array

### Step 2: Implement plugin inspect
- `plugin inspect {name}` (alias: `vistaarak parikshan`)
- Show: capabilities, supported models, interface version, binary path
- For extensions: show binary path, whether trusted, context tier support

### Step 3: Render output
- TTY: lipgloss table with provider dots
- Piped: tab-separated, no ANSI
- JSON: full plugin details

### Step 4: Unit tests
- List with mock registry (3 providers + 2 extensions)
- Inspect shows capabilities
- JSON output valid
- Empty registry → helpful message

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Real plugins
```bash
make build build-plugins
./bin/dootsabha plugin list
./bin/dootsabha plugin inspect claude
./bin/dootsabha vistaarak soochi --json | python3 -m json.tool
```

## Completion Criteria

1. `plugin list` shows all plugins and extensions
2. `plugin inspect` shows capabilities
3. Bilingual aliases work
4. JSON output valid
5. `make ci` passes

## Commit

```
feat(plugin-cmd): add vistaarak list/inspect commands

- plugin list (vistaarak soochi): all plugins + extensions
- plugin inspect (vistaarak parikshan): capabilities, models
- Health status indicators, JSON output
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.7
5. Execute steps 1-4
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
