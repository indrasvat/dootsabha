# Task 3.2: Plugin Manager (Discovery, Loading, Registry)

## Status: PENDING

## Depends On
- Task 3.1 (proto definitions)

## Parallelizable With
- None

## Problem

The plugin manager discovers, loads, and manages gRPC plugin binaries using hashicorp/go-plugin. It must handle: plugin directory scanning, handshake protocol, health monitoring, and graceful shutdown of plugin processes.

## PRD Reference
- §5.2 (Plugin Manager component)
- §5.3 (Plugin types: Provider, Strategy, Hook — discovery mechanisms)
- §4 (hashicorp/go-plugin v1.7.0)

## Files to Create
- `internal/plugin/manager.go` — Plugin discovery, loading, registry
- `internal/plugin/interfaces.go` — Go interfaces matching proto definitions
- `internal/plugin/grpc.go` — gRPC server/client implementations for go-plugin
- `internal/plugin/manager_test.go` — Unit tests

## Execution Steps

### Step 1: Read spike findings
1. Read `_spikes/go-plugin-grpc/README.md` (Spike 0.4 — handshake, latency)

### Step 2: Implement plugin discovery
- Scan `plugins/` directory for plugin binaries
- Each subdirectory = one plugin (e.g., `plugins/claude/claude-plugin`)
- Detect plugin type from handshake metadata

### Step 3: Implement plugin loading
- go-plugin `NewClient()` with gRPC protocol
- Handshake config with magic cookie
- Manage client lifecycle (start, health check, stop)

### Step 4: Implement registry
- `Registry` — thread-safe map of loaded plugins by name
- Methods: `Get(name)`, `List()`, `HealthCheck(name)`, `Shutdown()`
- Periodic health checks (configurable interval)

### Step 5: Implement graceful shutdown
- On Ctrl+C: stop all plugin clients
- On plugin crash: remove from registry, log error

### Step 6: Unit tests
- Plugin discovery from directory
- Plugin loading with mock plugin binary
- Registry operations (add, get, list, remove)
- Graceful shutdown

## Verification

### L1: Unit tests
```bash
make test
```

## Completion Criteria

1. Plugin discovery scans plugins/ directory
2. Loading via go-plugin gRPC handshake works
3. Registry tracks active plugins
4. Graceful shutdown stops all plugin processes
5. `make ci` passes

## Commit

```
feat(plugin): add plugin manager with discovery, loading, registry

- Directory-based plugin discovery
- go-plugin gRPC handshake and client management
- Thread-safe registry with health checks
- Graceful shutdown of plugin processes
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.2, §5.3
5. Read spike 0.4 findings
6. Execute steps 1-6
7. Run verification
8. **Change status to `DONE`**
9. Update `docs/PROGRESS.md`
10. Commit
