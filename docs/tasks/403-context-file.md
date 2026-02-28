# Task 4.4: Tier 2 Context File for Extensions

## Status: PENDING

## Depends On
- Task 4.1 (structured logging — for trace IDs)

## Parallelizable With
- Task 4.3 (edge cases)

## Problem

Extensions need rich context beyond env vars. Tier 2 provides a JSON context file with full config, provider registry, workspace info, and capabilities. Extensions read it via `$DOOTSABHA_CONTEXT_FILE`.

## PRD Reference
- §5.4 (Extension context protocol — Tier 2: context file)

## Files to Create
- `internal/plugin/context_file.go` — Context file generator
- `internal/plugin/context_file_test.go` — Unit tests

## Files to Modify
- `internal/plugin/extension.go` — Set DOOTSABHA_CONTEXT_FILE env var before exec

## Execution Steps

### Step 1: Define context file schema
```json
{
  "version": "0.1.0",
  "session_id": "ds_x7k2m",
  "workspace": "/path/to/workspace",
  "config": { ... },
  "providers": {
    "claude": {"healthy": true, "model": "sonnet-4-6"},
    "codex": {"healthy": true, "model": "gpt-5.3-codex"}
  },
  "capabilities": {"council": true, "review": true, "plugins": true},
  "tty": true,
  "terminal_width": 120
}
```

### Step 2: Implement context file writer
- Write to temp file (`os.CreateTemp`)
- Set `DOOTSABHA_CONTEXT_FILE` env var pointing to it
- Clean up after extension exits

### Step 3: Wire into extension exec
- Before exec: generate context file, set env var
- After exec: remove temp file

### Step 4: Unit tests
- Context file is valid JSON
- Contains all required fields
- Temp file cleaned up after extension exits
- Works with mock extension that reads the file

## Verification

### L1: Unit tests
```bash
make test
```

### L3: Extension reads context
```bash
make build
# Create extension that reads context file
cat > /tmp/dootsabha-ctx <<'EOF'
#!/bin/bash
echo "Context file: $DOOTSABHA_CONTEXT_FILE"
python3 -m json.tool "$DOOTSABHA_CONTEXT_FILE"
EOF
chmod +x /tmp/dootsabha-ctx
PATH="/tmp:$PATH" ./bin/dootsabha ctx
```

## Completion Criteria

1. Context file generated with full schema
2. DOOTSABHA_CONTEXT_FILE env var set before extension exec
3. Temp file cleaned up after execution
4. JSON is valid and complete
5. `make ci` passes

## Commit

```
feat(context): add Tier 2 context file for extensions

- JSON context file with config, providers, capabilities
- Written to temp file, cleaned up after extension exec
- DOOTSABHA_CONTEXT_FILE env var set for extensions
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §5.4
5. Execute steps 1-4
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
