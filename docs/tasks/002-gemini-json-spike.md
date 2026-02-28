# Task 0.3: Gemini JSON Output Spike

## Status: DONE

## Depends On
- None

## Parallelizable With
- All other spikes (0.1–0.2, 0.4–0.8)

## Problem

Gemini CLI v0.30.0 has `--output-format json` and `--yolo` flag, but the exact JSON schema is not documented. We need to verify: positional prompt vs `-p`, `--yolo` vs `--approval-mode yolo`, and the JSON response structure.

## PRD Reference
- §4.1 (Gemini CLI version, flags, verified behavior)
- §11 (Risk: Gemini JSON schema undocumented)

## Files to Create
- `_spikes/gemini-json/main.go` — Spike program
- `_spikes/gemini-json/README.md` — Findings doc

## Execution Steps

### Step 1: Initialize spike module
- **No top-level `go.mod` exists yet** (created in Task 1.1). Each spike is a standalone module.
- `mkdir -p _spikes/gemini-json && cd _spikes/gemini-json`
- `go mod init dootsabha-spike/gemini-json`

### Step 2: Read context
1. Read PRD §4.1 (Gemini CLI flags, verified v0.30.0 behavior)

### Step 3: Write spike program
- Run `gemini --yolo --output-format json "Say PONG"` and parse response
- Also test: `gemini --yolo -p "Say PONG" --output-format json`
- Also test: `gemini --approval-mode yolo --output-format json "Say PONG"`
- Extract: content, model, duration, tokens

### Step 4: Test variations
- Positional prompt vs `-p` flag — compare JSON output
- `--yolo` vs `--approval-mode yolo` — confirm identical behavior
- Error cases: no auth, invalid model, timeout

### Step 5: Document findings
- Exact JSON schema
- Which prompt mechanism is preferred (positional or `-p`)
- Differences between `--yolo` variants (if any)
- Error format

## Verification

### L1: Spike runs
```bash
cd _spikes/gemini-json && go run main.go
```

### L3: Real CLI output
```bash
gemini --yolo --output-format json "Say PONG" 2>/dev/null | python3 -m json.tool
```

## Completion Criteria

1. JSON schema captured with Go types
2. Positional vs `-p` behavior documented
3. `--yolo` vs `--approval-mode yolo` confirmed equivalent
4. Error format documented

## Commit

```
spike(gemini-json): validate JSON output schema and flag variants

- Captures exact JSON schema from gemini --output-format json
- Confirms --yolo and --approval-mode yolo are equivalent
- Documents positional prompt vs -p behavior
```

## Session Protocol

1. Read CLAUDE.md — **skip if it doesn't exist yet (created in Task 1.1)**
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4.1
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md` — **if it doesn't exist, create it with a Phase 0 header and this spike's entry**
9. Commit
