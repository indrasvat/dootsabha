# Task 0.8: PTY vs Pipe Subprocess Spike

## Status: DONE

## Depends On
- None

## Parallelizable With
- All other spikes (0.1–0.7)

## Problem

Some CLIs behave differently when spawned via pipe vs PTY (buffering, interactive prompts, JSON output differences). We must verify that claude/codex/gemini with their YOLO+JSON flags produce identical output in pipe mode. If not, we may need `creack/pty`.

## PRD Reference
- §4.1 (CLI YOLO flags that prevent interactive prompts)
- §11 (Risk: CLIs need PTY, not pipe)

## Files to Create
- `_spikes/pty-pipe/main.go` — Spike program (runs each CLI in pipe mode)
- `_spikes/pty-pipe/README.md` — Findings doc

## Execution Steps

### Step 1: Initialize spike module
- **No top-level `go.mod` exists yet** (created in Task 1.1). Each spike is a standalone module.
- `mkdir -p _spikes/pty-pipe && cd _spikes/pty-pipe`
- `go mod init dootsabha-spike/pty-pipe`

### Step 2: Read context
1. Read PRD §4.1 (CLI flags per provider)

### Step 3: Write spike program
- For each CLI (claude, codex, gemini):
  - Spawn via `os/exec` (pipe mode) with YOLO+JSON flags
  - Capture stdout/stderr separately
  - Parse JSON output
- Compare: is JSON output complete and valid?
- Check: any interactive prompts that would block?

### Step 4: Test each CLI
- `claude -p "PONG" --output-format json --dangerously-skip-permissions` via pipe
- `codex exec --json --sandbox danger-full-access "PONG"` via pipe
- `gemini --yolo --output-format json "PONG"` via pipe
- Verify: no prompts, no blocking, valid JSON

### Step 5: Document findings
- Which CLIs work perfectly via pipe
- Any differences vs interactive terminal execution
- Whether `creack/pty` is needed (ideally not)
- Recommended subprocess spawn pattern per CLI

## Verification

### L1: Spike runs
```bash
cd _spikes/pty-pipe && go run main.go
```

### L3: Direct CLI test
```bash
# Each should produce valid JSON without hanging
claude -p "PONG" --output-format json --dangerously-skip-permissions | python3 -m json.tool
codex exec --json --sandbox danger-full-access "PONG" | tail -1 | python3 -m json.tool
gemini --yolo --output-format json "PONG" | python3 -m json.tool
```

## Completion Criteria

1. All 3 CLIs tested in pipe mode with YOLO+JSON flags
2. JSON output validity confirmed for each
3. No interactive blocking in pipe mode
4. Decision documented: plain pipe is sufficient OR creack/pty needed

## Commit

```
spike(pty-pipe): verify CLI behavior with plain pipes vs PTY

- Tests claude/codex/gemini JSON output via os/exec pipe
- Confirms YOLO flags prevent interactive prompts
- Documents per-CLI spawn pattern recommendation
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
