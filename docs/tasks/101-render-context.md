# Task 1.2: Render Context & Output Foundation

## Status: DONE

## Depends On
- Task 1.1 (project scaffold)

## Parallelizable With
- Task 1.3 (config manager)

## Problem

All दूतसभा output must flow through a render context that handles: TTY detection, NO_COLOR, terminal width, pipe mode (no ANSI), and `--json` format. This is the output foundation that every later task builds on.

## PRD Reference
- §8 (Terminal UX standards — full section)
- §8.1 (Good Unix citizenship: stdout=data, stderr=logs)
- §8.2 (Color palette — provider colors)
- §8.3 (Graceful degradation matrix)
- §8.4 (Lipgloss pitfalls)

## Files to Create
- `internal/output/renderer.go` — `RenderContext` struct with TTY/color/width/format detection
- `internal/output/styles.go` — Provider colors, badges, theme constants
- `internal/output/json.go` — JSON output helper (with `schema_version`)
- `internal/output/table.go` — lipgloss table helper
- `internal/output/renderer_test.go` — Unit tests
- `internal/output/json_test.go` — JSON output validation

## Execution Steps

### Step 1: Implement RenderContext
- `RenderContext{IsTTY bool, HasColor bool, Width int, Format string}`
- Detect TTY via `os.Stdout.Fd()` + `golang.org/x/term`
- Detect NO_COLOR env var
- Detect terminal width (fallback 80)
- `--json` flag override

### Step 2: Implement styles
- Provider color constants (Claude amber, Codex emerald, Gemini blue, Error red, Success green)
- Provider dot renderer: `●` (TTY+color) / `*` (TTY+NO_COLOR) / `*` (pipe)
- Status indicators: `✓`/`✗` (TTY) / `OK`/`FAIL` (pipe)

### Step 3: Implement JSON helper
- `WriteJSON(w io.Writer, v any)` with indent
- Always includes `meta.schema_version: 1`
- Never includes ANSI codes

### Step 4: Implement table helper
- lipgloss/table wrapper respecting RenderContext
- Tab-separated fallback when piped

### Step 5: Unit tests
- TTY vs pipe detection (mock fd)
- NO_COLOR behavior
- JSON output validity
- No ANSI in piped output
- Table degradation

## Verification

### L1: Unit tests
```bash
make test
```

### L2: Render context behavior
```bash
go test -run TestRenderContext -v ./internal/output/...
```

## Completion Criteria

1. RenderContext correctly detects TTY, NO_COLOR, width
2. Provider colors render correctly in TTY mode
3. Piped output has zero ANSI codes
4. JSON helper produces valid JSON with schema_version
5. `make ci` passes

## Commit

```
feat(output): add render context with TTY/pipe/color detection

- RenderContext: TTY, NO_COLOR, width, format detection
- Provider color palette (lipgloss) with degradation
- JSON output helper with schema_version
- Table helper with pipe-mode fallback
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §8 (all subsections)
5. Execute steps 1-5
6. Run verification (L1 → L2)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
