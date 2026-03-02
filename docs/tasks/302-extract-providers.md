# Task 3.3: Extract Provider Plugins (claude, codex, gemini)

## Status: DONE

## Depends On
- Task 3.2 (plugin manager)

## Parallelizable With
- None (requires careful extraction from hardcoded code)

## Problem

Extract the 3 hardcoded providers from `internal/providers/` into standalone gRPC plugin binaries under `plugins/`. The core must continue working identically — zero regression from plugin extraction.

## PRD Reference
- §5.3 (Provider plugin interface: Invoke, Cancel, HealthCheck, Capabilities)
- §5.5 (Key decision: hardcoded in P1-P2, plugins in P3)

## Files to Create
- `plugins/claude/main.go` — Claude plugin binary (implements Provider gRPC service)
- `plugins/codex/main.go` — Codex plugin binary
- `plugins/gemini/main.go` — Gemini plugin binary
- `plugins/claude/claude_test.go` — Plugin-level tests
- `plugins/codex/codex_test.go`
- `plugins/gemini/gemini_test.go`

## Files to Modify
- `internal/providers/claude.go` — Extract to plugin, keep thin adapter
- `internal/providers/codex.go` — Extract to plugin, keep thin adapter
- `internal/providers/gemini.go` — Extract to plugin, keep thin adapter
- `Makefile` — Add `build-plugins` target

## Execution Steps

### Step 1: Create plugin binaries
- Each plugin implements the Provider gRPC service from proto/provider.proto
- Reuse parsing/invocation logic from internal/providers/*
- Plugin main.go calls `plugin.Serve()` with gRPC server

### Step 2: Create thin adapters
- `internal/providers/claude.go` → delegates to plugin via plugin manager
- Falls back to direct invocation if plugin not found (graceful)

### Step 3: Build all plugins
- `make build-plugins` → builds 3 plugin binaries into `plugins/`
- `make build` continues to build the host binary

### Step 4: Zero-regression verification
- Run all existing tests — must pass without changes
- Run all L3 smoke tests — same output
- Run L4 visual — same rendering

### Step 5: Unit tests
- Plugin responds to Invoke/HealthCheck/Capabilities RPCs
- Host communicates with plugin via gRPC
- Fallback to direct invocation when plugin missing

### Step 6: Real-world scenario smoke tests (deferred from Task 300)
Create `scripts/test-plugin-smoke.sh` with these scenarios:
- Mock-provider serves Claude-like responses with realistic payloads (session_id, cost, tokens)
- Mock-strategy executes council pipeline end-to-end (3 dispatch, 3 review, synthesis, meta)
- Mock-hook rewrites prompts (PreInvoke) + redacts responses (PostInvoke)
- Full lifecycle: discover → handshake → invoke → shutdown → no orphan processes (pgrep check)
- Realistic payload sizes (10KB prompt, 32KB response — the truncation limit)
- Zero regression: all existing `make test-binary` tests still pass
- Add `make test-plugins` target

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Zero regression + plugin smoke
```bash
make build build-plugins
make test-binary     # 8/8 existing smoke tests
make test-plugins    # 6 real-world scenario tests
./bin/dootsabha consult --agent claude "PONG"
./bin/dootsabha consult --agent codex "PONG"
./bin/dootsabha consult --agent gemini "PONG"
./bin/dootsabha status
./bin/dootsabha council "PONG" --json | python3 -m json.tool
```

## Completion Criteria

1. 3 plugin binaries build and serve gRPC
2. Host communicates with plugins via go-plugin
3. Zero regression — all existing tests pass
4. Fallback to direct invocation when plugins missing
5. 6 real-world scenario smoke tests pass
6. `make check` passes

## Commit

```
feat(plugins): extract claude, codex, gemini into gRPC plugins

- 3 standalone plugin binaries under plugins/
- go-plugin gRPC communication via proto/provider.proto
- Thin adapters with fallback to direct invocation
- Zero regression from extraction
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.3, §5.5
5. Execute steps 1-5
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
