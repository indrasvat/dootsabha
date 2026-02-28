# Task 3.1: Proto Definitions + Code Generation

## Status: PENDING

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

## Files to Create
- `proto/provider.proto` — Provider service definition
- `proto/strategy.proto` — Strategy service definition
- `proto/hook.proto` — Hook service definition
- `proto/gen/provider.pb.go` — Generated (or vendored)
- `proto/gen/strategy.pb.go` — Generated (or vendored)
- `proto/gen/hook.pb.go` — Generated (or vendored)
- `Makefile` addition: `proto` target for regeneration

## Execution Steps

### Step 1: Read spike findings
1. Read `_spikes/go-plugin-grpc/README.md` (Spike 0.4 — gRPC patterns)

### Step 2: Define Provider proto
```protobuf
service Provider {
  rpc Invoke(InvokeRequest) returns (InvokeResponse);
  rpc Cancel(CancelRequest) returns (CancelResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Capabilities(CapabilitiesRequest) returns (CapabilitiesResponse);
}
```

### Step 3: Define Strategy proto
```protobuf
service Strategy {
  rpc Execute(ExecuteRequest) returns (ExecuteResponse);
}
```

### Step 4: Define Hook proto
```protobuf
service Hook {
  rpc PreInvoke(HookRequest) returns (HookResponse);
  rpc PostInvoke(HookRequest) returns (HookResponse);
  rpc PreSynthesis(HookRequest) returns (HookResponse);
  rpc PostSession(HookRequest) returns (HookResponse);
}
```

### Step 5: Generate Go code
- Install protoc + protoc-gen-go + protoc-gen-go-grpc
- Add `make proto` target
- Generate and vendor (or gitignore + generate in CI)

### Step 6: Unit tests
- Proto definitions compile
- Generated code is importable

## Verification

### L1: Proto compiles
```bash
make proto
make test
```

## Completion Criteria

1. All 3 proto files define correct service interfaces
2. Generated Go code compiles
3. `make proto` regenerates code
4. Decision on vendoring documented (resolve Q4)
5. `make ci` passes

## Commit

```
feat(proto): add gRPC proto definitions for provider, strategy, hook

- provider.proto: Invoke, Cancel, HealthCheck, Capabilities
- strategy.proto: Execute
- hook.proto: PreInvoke, PostInvoke, PreSynthesis, PostSession
- make proto target for code generation
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
