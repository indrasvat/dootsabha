# Task 3.2: Plugin Manager (Discovery, Loading, Registry)

## Status: DONE

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

### Step 6: Build mock plugin binaries
- `testdata/mock-plugins/mock-provider/main.go` — implements Provider gRPC, canned responses
- `testdata/mock-plugins/mock-strategy/main.go` — implements Strategy gRPC, mock pipeline
- `testdata/mock-plugins/mock-hook/main.go` — implements Hook gRPC, prompt rewrite + redaction
- Add `make build-test-plugins` target

### Step 7: GRPCServer/GRPCClient wrappers
- `internal/plugin/provider_grpc.go` — GRPCServer/GRPCClient for Provider (like spike shared/interface.go)
- `internal/plugin/strategy_grpc.go` — GRPCServer/GRPCClient for Strategy
- `internal/plugin/hook_grpc.go` — GRPCServer/GRPCClient for Hook

### Step 8: Unit tests + go-plugin integration tests

**Registry & Discovery tests:**
- Plugin discovery from directory
- Registry operations (add, get, list, remove)
- Graceful shutdown

**go-plugin Integration tests (20 tests, deferred from Task 300):**

Provider plugin (8 tests):
- `TestProviderPluginHandshake` — basic connectivity, HandshakeConfig match
- `TestProviderPluginInvokeRoundtrip` — full RPC with prompt, model, response
- `TestProviderPluginHealthCheckRoundtrip` — health status via gRPC
- `TestProviderPluginCapabilitiesRoundtrip` — supported_models, max_context_tokens
- `TestProviderPluginCancelRoundtrip` — cancel with session_id
- `TestProviderPluginErrorPropagation` — gRPC error boundary (MOCK_ERROR=true)
- `TestProviderPluginContextCancellation` — deadline propagation through go-plugin
- `TestProviderPluginConcurrentCalls` — 5 concurrent Invoke RPCs via errgroup

Strategy plugin (3 tests):
- `TestStrategyPluginHandshake` — separate MagicCookieValue (dootsabha-strategy-v1)
- `TestStrategyPluginExecuteRoundtrip` — 3 agents, full pipeline response
- `TestStrategyPluginExecuteErrorPropagation` — 0 agents triggers error

Hook plugin (5 tests):
- `TestHookPluginHandshake` — MagicCookieValue dootsabha-hook-v1
- `TestHookPluginPreInvokeModifiesRequest` — prompt rewriting hook
- `TestHookPluginPostInvokeModifiesResponse` — PII redaction hook
- `TestHookPluginPreSynthesisPassthrough` — response list passthrough
- `TestHookPluginPostSessionRecords` — notification-only hook

Plugin lifecycle (4 tests):
- `TestPluginCrashRecovery` — kill + relaunch succeeds
- `TestConcurrentPluginConnections` — 3 simultaneous plugin processes
- `TestPluginBinaryMissing` — clean error, no panic, no hang
- `TestPluginHandshakeMismatch` — wrong MagicCookieValue rejected

## Verification

### L1: Builds
```bash
make build-test-plugins
make build
```

### L2: All tests pass
```bash
make test
```

### L3: No regression
```bash
make test-binary
```

## Completion Criteria

1. Plugin discovery scans plugins/ directory
2. Loading via go-plugin gRPC handshake works
3. Registry tracks active plugins
4. GRPCServer/GRPCClient wrappers for all 3 plugin types
5. 3 mock plugin binaries built and tested
6. 20+ go-plugin integration tests pass
7. Graceful shutdown stops all plugin processes
8. `make check` passes

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
