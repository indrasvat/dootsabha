# Task 3.1: Proto Definitions + Code Generation

## Status: IN PROGRESS

## Depends On
- Phase 2 complete (council pipeline working with hardcoded providers)

## Parallelizable With
- None (proto definitions are foundation for all P3 tasks)

## Problem

The plugin system uses gRPC for communication between host and plugins. We need .proto definitions for Provider, Strategy, and Hook interfaces, plus code generation setup. This defines the contract that plugins implement.

## PRD Reference
- §5.3 (Plugin types — Provider, Strategy, Hook interfaces with methods)
- §5.2 (Provider interface: Invoke, Cancel, HealthCheck, Capabilities)
- §12 (Q4: vendor proto-generated code?)

## Files Created
- `proto/provider.proto` — Provider service (Invoke, Cancel, HealthCheck, Capabilities)
- `proto/strategy.proto` — Strategy service (Execute) with AgentConfig, StrategyConfig, DispatchResult, ReviewResult, SynthesisResult, SessionMeta
- `proto/hook.proto` — Hook service (PreInvoke, PostInvoke, PreSynthesis, PostSession) with EventType enum, oneof payloads
- `proto/gen/*.pb.go` — Generated and vendored (6 files)
- `proto/gen/gen_test.go` — 35 L2 tests (message, serialization, edge cases)
- `internal/plugin/convert.go` — Go type ↔ proto conversion helpers
- `internal/plugin/convert_test.go` — 13 L2 tests (roundtrip, field coverage, arch doc)
- `Makefile` — Added `proto` target + `proto/...` in GO_DIRS

## Design Decisions
1. **Vendor generated code** — resolves PRD Q4: contributors don't need protoc installed
2. **int64 for duration_ms** — matches all existing JSON output (not google.protobuf.Duration)
3. **oneof for HookRequest/HookResponse payloads** — 4 event types carry different data
4. **Architecture-doc fields in proto** — temperature, output_format, extra_args, env, work_dir included for third-party plugin extensibility
5. **Single Go package** — `go_package = "github.com/indrasvat/dootsabha/proto/gen"` for all 3 protos

## Test Summary (48 tests)

### proto/gen/gen_test.go (35 tests)
| Category | Tests | What |
|----------|-------|------|
| Message construction | 5 | InvokeRequest, InvokeResponse, HealthCheck, Capabilities, SessionSummary |
| Serialization roundtrips | 6 | All major messages through marshal/unmarshal/proto.Equal |
| Proto3 semantics | 4 | Default values, zero-value behavior, repeated fields, map fields |
| Enum & oneof | 3 | EventType values, HookRequest oneof variants, HookResponse oneof |
| Edge cases: strings & sizes | 6 | Empty strings, 32KB, 128KB, Devanagari, emoji, CJK |
| Edge cases: numbers & collections | 6 | Negative cost, 100 extra_args, 50 env entries, MaxAgents boundary |
| Strategy specifics | 5 | DispatchResult error, review order, synthesis fallback, session meta map |

### internal/plugin/convert_test.go (13 tests)
| Category | Tests | What |
|----------|-------|------|
| Roundtrip conversions | 6 | ProviderResult, HealthStatus, InvokeOptions, DispatchResult, ReviewResult, SynthesisResult |
| Duration edge cases | 1 | 0ms through 30min, sub-ms truncation |
| Field coverage (reflection) | 4 | ProviderResult, DispatchResult, ReviewResult, SynthesisResult |
| Architecture doc extension points | 2 | InvokeRequest extension fields, CapabilitiesResponse fields |

## Verification

### L1: Proto compiles + binary builds
```bash
make proto
make build
```

### L2: All 48 tests pass
```bash
make test
```

### L3: No regression (8/8 smoke tests)
```bash
make test-binary
```

### Full suite
```bash
make check   # 0 lint issues, all tests, 8/8 smoke
```

## Completion Criteria

1. All 3 proto files define correct service interfaces
2. Generated Go code compiles and is vendored
3. `make proto` regenerates identical code
4. 48 L2 tests pass covering messages, roundtrips, edge cases, field coverage
5. Go ↔ proto conversion helpers with full roundtrip verification
6. Architecture doc extension fields present for future third-party plugins
7. `make check` passes (0 lint issues, all tests, 8/8 L3 smoke)

## Commit

```
feat(proto): add gRPC proto definitions for provider, strategy, hook

- provider.proto: Invoke, Cancel, HealthCheck, Capabilities
- strategy.proto: Execute with full council pipeline types
- hook.proto: 4 lifecycle hooks with EventType enum and oneof payloads
- Go ↔ proto conversion helpers (internal/plugin/convert.go)
- 48 L2 tests: messages, serialization, edge cases, field coverage
- make proto target, generated code vendored (resolves Q4)
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.3
5. Read spike 0.4 findings
6. Execute steps 1-6
7. Run verification
8. **Change status to `DONE`**
9. Update `docs/PROGRESS.md`
10. Commit
