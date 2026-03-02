# Task 3.5: Extension Discovery ($PATH + plugins dir)

## Status: DONE

## Depends On
- Task 3.2 (plugin manager)

## Parallelizable With
- Task 3.3 (extract providers), Task 3.4 (council strategy)

## Problem

Extensions are external binaries on `$PATH` named `dootsabha-{name}` that extend the CLI without gRPC. When a user types an unknown command, दूतसभा should discover the extension, prompt for trust on first run, and exec it.

## PRD Reference
- §5.3 (Extension type: binary, exec transport, PATH + plugins dir discovery)
- §5.4 (Extension context protocol — 3 tiers of context)
- §6.1 (Unknown command → extension discovery → trust prompt → exec)

## Files to Create
- `internal/plugin/extension.go` — PATH-based extension discovery & exec
- `internal/plugin/extension_test.go` — Unit tests

## Files to Modify
- `internal/cli/root.go` — Wire unknown command handler to extension discovery

## Execution Steps

### Step 1: Implement extension discovery
- Scan `$PATH` for `dootsabha-{name}` binaries
- Also scan `plugins/` directory
- Return list of discovered extensions with paths

### Step 2: Implement trust-on-first-run
- First execution of an unknown extension: show binary path, prompt for confirmation
- Store trusted extensions in `~/.dootsabha/trusted-extensions.yaml`
- Subsequent runs: exec immediately if trusted

### Step 3: Implement extension execution
- Exec extension binary with Tier 1 context (env vars: DOOTSABHA_*)
- Pass remaining args to extension
- Forward stdin/stdout/stderr

### Step 4: Wire to unknown command handler
- Cobra's `RunE` on root: check for extension before "unknown command" error
- If found and trusted: exec
- If found and untrusted: prompt
- If not found: standard "unknown command" error

### Step 5: Unit tests
- Extension discovery from PATH mock
- Trust store read/write
- Unknown command → extension exec flow
- Untrusted extension → prompt flow

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Extension discovery
```bash
make build
# Create a dummy extension
echo '#!/bin/bash\necho "hello from extension"' > /tmp/dootsabha-hello && chmod +x /tmp/dootsabha-hello
PATH="/tmp:$PATH" ./bin/dootsabha hello
```

## Completion Criteria

1. Extensions discovered from $PATH + plugins dir
2. Trust-on-first-run with yaml store
3. Unknown command triggers extension lookup
4. Tier 1 context (env vars) passed to extensions
5. `make ci` passes

## Commit

```
feat(extensions): add PATH-based extension discovery and trust system

- Discover dootsabha-{name} on $PATH and in plugins/
- Trust-on-first-run with ~/.dootsabha/trusted-extensions.yaml
- Unknown command handler wired to extension discovery
- Tier 1 DOOTSABHA_* env vars passed to extensions
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.3, §5.4, §6.1
5. Execute steps 1-5
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
