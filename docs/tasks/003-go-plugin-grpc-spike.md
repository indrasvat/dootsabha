# Task 0.4: go-plugin gRPC Handshake Spike

## Status: PENDING

## Depends On
- None

## Parallelizable With
- All other spikes (0.1–0.3, 0.5–0.8)

## Problem

hashicorp/go-plugin v1.7.0 is the plugin foundation for Phase 3. We must validate: gRPC handshake latency, process isolation behavior, plugin crash recovery, and the developer ergonomics of writing a plugin. If overhead is >200ms, we may need in-process providers instead.

## PRD Reference
- §4 (go-plugin v1.7.0 in tech stack)
- §5.3 (Plugin types: Provider, Strategy, Hook)
- §11 (Risk: gRPC overhead too high)

## Files to Create
- `_spikes/go-plugin-grpc/host/main.go` — Host program
- `_spikes/go-plugin-grpc/plugin/main.go` — Minimal gRPC plugin
- `_spikes/go-plugin-grpc/shared/interface.go` — Shared interface
- `_spikes/go-plugin-grpc/README.md` — Findings doc

## Execution Steps

### Step 1: Initialize spike module
- **No top-level `go.mod` exists yet** (created in Task 1.1). Each spike is a standalone module.
- `mkdir -p _spikes/go-plugin-grpc && cd _spikes/go-plugin-grpc`
- `go mod init dootsabha-spike/go-plugin-grpc`
- `go get github.com/hashicorp/go-plugin@v1.7.0 google.golang.org/grpc`

### Step 2: Read context
1. Read PRD §5.3 (plugin types and interfaces)
2. Read hashicorp/go-plugin examples

### Step 3: Write minimal plugin pair
- Define a simple `Greeter` interface with one `Greet(string) string` method
- Implement gRPC server (plugin side) and client (host side)
- Use `plugin.Serve()` in plugin, `plugin.NewClient()` in host

### Step 4: Measure
- Handshake latency (time from NewClient to first RPC call)
- Per-call latency (subsequent RPC calls)
- Memory overhead (host + plugin processes)
- Plugin crash → host recovery behavior

### Step 5: Document findings
- Measured latencies (median, p95)
- Plugin crash recovery mechanism
- Recommended patterns for production use
- Go-plugin version-specific gotchas

## Verification

### L1: Spike runs
```bash
cd _spikes/go-plugin-grpc
go build -o plugin/greeter-plugin ./plugin
go run ./host
```

### Latency check
```bash
# Should see handshake <100ms, per-call <5ms
```

## Completion Criteria

1. Host successfully communicates with plugin via gRPC
2. Handshake latency measured and documented (<200ms gate)
3. Plugin crash recovery verified
4. README.md with latency data and recommended patterns

## Commit

```
spike(go-plugin): validate gRPC handshake latency and crash recovery

- Minimal host+plugin pair using go-plugin v1.7.0
- Measures handshake latency, per-call latency, memory
- Documents crash recovery and production patterns
```

## Session Protocol

1. Read CLAUDE.md — **skip if it doesn't exist yet (created in Task 1.1)**
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4, §5.3
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md` — **if it doesn't exist, create it with a Phase 0 header and this spike's entry**
9. Commit
