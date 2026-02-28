# Task 4.1: Structured Logging (slog)

## Status: PENDING

## Depends On
- Phase 3 complete

## Parallelizable With
- Task 4.2 (metrics collection)

## Problem

दूतसभा needs structured logging via Go's stdlib `log/slog`. Must support JSON and text handlers, configurable levels via `-v`/`-vv`/`-vvv`, and ensure logs go to stderr only (stdout is for data).

## PRD Reference
- §4 (log/slog in tech stack — zero deps)
- §8.1 (stdout=data, stderr=logs)
- §6.1 (--verbose / -v flag)

## Files to Create
- `internal/observability/logger.go` — slog setup with JSON/text handlers
- `internal/observability/trace.go` — Trace ID generation (ds_{random5})
- `internal/observability/logger_test.go` — Unit tests

## Files to Modify
- `internal/cli/root.go` — Wire -v/-vv/-vvv to log level

## Execution Steps

### Step 1: Implement logger setup
- `NewLogger(level slog.Level, format string, w io.Writer) *slog.Logger`
- JSON handler for `--json` mode, text handler for TTY
- Always writes to stderr
- Default level: Warn. `-v` = Info, `-vv` = Debug, `-vvv` = Debug + source

### Step 2: Implement trace ID
- Generate `ds_{random5}` session ID at startup
- Include in all log entries as `session_id` attribute
- Same ID used in JSON meta blocks

### Step 3: Add logging to existing code
- Provider invocations: log at Info level
- Errors: log at Error with structured fields
- Plugin lifecycle: log at Debug
- Subprocess: log command at Debug, exit code at Info

### Step 4: Unit tests
- Log level mapping from -v flags
- JSON handler produces valid JSON to stderr
- Text handler produces readable output
- Trace ID format

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Verbose output
```bash
make build
./bin/dootsabha consult -v "PONG" 2>&1 >/dev/null | head  # stderr only
./bin/dootsabha consult -vvv "PONG" 2>&1 >/dev/null | head  # debug + source
```

## Completion Criteria

1. slog with JSON + text handlers
2. -v/-vv/-vvv mapped to log levels
3. All logs go to stderr
4. Trace ID in all log entries
5. `make ci` passes

## Commit

```
feat(logging): add structured logging with slog and trace IDs

- slog with JSON/text handlers on stderr
- -v/-vv/-vvv verbosity levels
- Session trace ID (ds_{random5}) in all entries
- Logging wired to providers, engine, plugins
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4 (slog), §8.1, §6.1
5. Execute steps 1-4
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
