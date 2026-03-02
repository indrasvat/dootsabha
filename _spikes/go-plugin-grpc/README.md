# Spike 003: go-plugin gRPC Handshake Latency

**Date:** 2026-02-28
**Platform:** macOS Darwin 25.3.0 / Apple Silicon
**go-plugin version:** v1.7.0
**gRPC version:** v1.79.1

---

## Result: ✅ PASS — gRPC overhead is acceptable

The 200ms handshake gate is comfortably cleared. Plugin architecture is viable for Phase 3.

---

## Measured Latencies

### Handshake Latency (5 runs — process launch to first usable RPC client)

| Metric | Value |
|--------|-------|
| min    | 6.6 ms |
| median | 7.6 ms |
| p95    | 20.5 ms |
| max    | 20.5 ms |
| **gate** | **<200 ms** ✅ |

> First run is slowest (~20ms) due to OS process loader cold start. Subsequent relaunches are 6–9ms.

### Per-Call Latency (100 calls — steady-state RPC round-trip)

| Metric | Value |
|--------|-------|
| min    | 29 µs |
| median | 45 µs |
| p95    | 102 µs |
| p99    | 281 µs |
| max    | 281 µs |

> Negligible overhead for our use case (AI CLI calls dominate at seconds, not microseconds).

### Memory Overhead (host process)

- Host heap (Go runtime): ~1 MB
- Plugin process: separate OS process — each plugin binary adds ~10–15 MB RSS in practice (Go runtime overhead per process)
- **Architecture implication:** limit plugins to <5 concurrent processes; use health checks to idle-kill unused plugins

---

## Crash Recovery Behavior

1. Plugin killed abruptly via `client.Kill()`
2. Next RPC call returns: `rpc error: code = Canceled desc = grpc: the client connection is closing` — fails fast, no hang
3. Host creates a new `plugin.Client`, re-launches plugin binary in **~7ms**
4. Subsequent calls succeed normally

**Verdict:** Recovery is clean. go-plugin's process model means a crashed plugin cannot corrupt the host. The host just detects the dead connection and re-dials.

---

## Recommended Patterns for Production Use

### 1. HandshakeConfig must match exactly

The `MagicCookieKey`/`MagicCookieValue` pair prevents accidental binary exec of non-plugin binaries. Use distinct values per plugin type (provider vs strategy vs hook).

```go
var HandshakeConfig = plugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "DOOTSABHA_PROVIDER_PLUGIN",
    MagicCookieValue: "dootsabha-provider-v1",
}
```

### 2. Always use `plugin.ProtocolGRPC`; disable net/rpc

```go
AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
```

### 3. Suppress hclog noise in production

go-plugin uses hclog internally. Set `Level: hclog.Error` in the logger to suppress handshake/debug chatter in production output.

### 4. Kill plugins on context cancellation

```go
defer client.Kill()
// OR wire client.Kill to a context cancel
```

go-plugin does **not** automatically kill child processes on host exit via GC. Always call `Kill()` or set up a shutdown hook.

### 5. Re-launch on error, not retry

If `rpcClient.Client()` or `Dispense()` fails, kill the client and create a new one. go-plugin connections are not re-connectable.

### 6. Proto file → generated code → shared package

Keep the `.proto` file and generated `*.pb.go` / `*_grpc.pb.go` in the shared package. Both host and plugin import the same `shared` package — no duplication.

### 7. GRPCServer / GRPCClient wrappers in shared package

Keep the `GRPCServer` (server-side adapter) and `GRPCClient` (client-side adapter) in the same `shared` package as the Go interface. This keeps all transport logic in one place.

---

## go-plugin v1.7.0 Gotchas

1. **`Logger` field requires `hclog.Logger`** — not `*log.Logger`. Import `github.com/hashicorp/go-hclog`.
2. **`UnimplementedGreeterServer` must be embedded** in the gRPC server struct (protoc-gen-go-grpc v1.6+ requirement).
3. **`go build -o plugin/greeter-plugin ./plugin` must run from the module root** — relative paths in `exec.Command` are resolved from the working directory of the host process, not the binary.
4. **`paths=source_relative` with protoc** — run protoc from the module root, not the `shared/` subdirectory, to get correct package output paths.
5. **go-plugin uses `yamux` over a single TCP or unix socket** — the gRPC connection is multiplexed, not a raw TCP port per plugin. No port conflicts.

---

## Architecture Decision for दूतसभा Phase 3

| Question | Decision | Rationale |
|----------|----------|-----------|
| gRPC vs net/rpc | **gRPC** | Typed contracts, forward-compatible with proto evolution |
| Plugin discovery | Scan `~/.dootsabha/plugins/` + `./plugins/` | Same as Terraform/Vault pattern |
| Plugin types | Provider, Strategy, Hook (§5.3) | One HandshakeConfig per type with distinct MagicCookieValue |
| Process limit | ≤5 concurrent plugin processes | Balance isolation vs memory; idle-kill after 30s |
| Crash handling | Re-launch on next call (no retry loops) | Clean, simple, proven pattern |

---

## Files

```
_spikes/go-plugin-grpc/
├── go.mod                      # dootsabha-spike/go-plugin-grpc
├── shared/
│   ├── greeter.proto           # Service definition
│   ├── greeter.pb.go           # Generated protobuf types
│   ├── greeter_grpc.pb.go      # Generated gRPC service
│   └── interface.go            # Go interface + GRPCPlugin wrapper
├── plugin/
│   ├── main.go                 # Plugin server (plugin.Serve)
│   └── greeter-plugin          # Compiled plugin binary
└── host/
    └── main.go                 # Host (plugin.NewClient + measurements)
```
