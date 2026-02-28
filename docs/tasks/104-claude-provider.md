# Task 1.5: Claude Provider (Hardcoded)

## Status: PENDING

## Depends On
- Task 1.2 (render context), Task 1.3 (config manager), Task 1.4 (subprocess runner)

## Parallelizable With
- None (first provider, establishes patterns for 1.6)

## Problem

Implement the first hardcoded provider (claude) that invokes `claude -p` with JSON output, parses the response, runs health checks, and produces styled output. This establishes the provider pattern that codex and gemini will follow.

## PRD Reference
- §4.1 (Claude CLI flags, version, CLAUDECODE gotcha)
- §5.2 (Provider interface: Invoke, Cancel, HealthCheck, Capabilities)
- §6.3 (Consult command output — what provider must return)
- §6.5 (Status command — health check format)

## Files to Create
- `internal/providers/claude.go` — Claude provider implementation
- `internal/providers/types.go` — Shared provider types (Result, HealthStatus, Capabilities)
- `internal/providers/claude_test.go` — Unit tests with mock subprocess

## Execution Steps

### Step 1: Read spike findings
1. Read `_spikes/claude-json/README.md` (Spike 0.2 findings — JSON schema)

### Step 2: Define provider types
- `ProviderResult{Content, Model, Duration, CostUSD, TokensIn, TokensOut, SessionID}`
- `HealthStatus{Healthy, CLIVersion, Model, AuthValid, Error}`
- `Capabilities{Models []string, SupportsJSON, SupportsCost, SupportsTokens}`

### Step 3: Implement Claude provider
- `Invoke(ctx, prompt, opts)` → build args from config, run subprocess, parse JSON
- `HealthCheck(ctx)` → run `claude --version`, check auth
- `Cancel(ctx)` → kill subprocess (uses SubprocessRunner context cancellation)
- Unset `CLAUDECODE` in subprocess env

### Step 4: Implement `--model` override
- Config default + CLI flag override
- Pass `--model` to claude CLI if specified

### Step 5: Unit tests
- Mock subprocess runner returns canned JSON
- Verify JSON parsing extracts all fields
- Health check with mock version output
- Error handling (auth failure, timeout)

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Real CLI (tiny prompt)
```bash
make build
./bin/dootsabha consult --agent claude "Say PONG" 2>/dev/null
# Should show styled output with provider dot
```

## Completion Criteria

1. Claude provider invokes CLI with correct flags
2. JSON response parsed into ProviderResult
3. CLAUDECODE env var removed in subprocess
4. Health check works
5. `--model` override works
6. `make ci` passes

## Commit

```
feat(claude): add hardcoded claude provider with JSON parsing

- Invokes claude -p with --output-format json
- Parses JSON response into ProviderResult
- CLAUDECODE env var removal, --model override
- HealthCheck via claude --version
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4.1, §5.2, §6.3, §6.5
5. Read spike 0.2 findings
6. Execute steps 1-5
7. Run verification (L1 → L3)
8. **Change status to `DONE`**
9. Update `docs/PROGRESS.md`
10. Commit
