# Task 0.8: PTY vs Pipe Subprocess Spike

## Status: PENDING

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

### Step 1: Read context
1. Read PRD §4.1 (CLI flags per provider)

### Step 2: Write spike program
- For each CLI (claude, codex, gemini):
  - Spawn via `os/exec` (pipe mode) with YOLO+JSON flags
  - Capture stdout/stderr separately
  - Parse JSON output
- Compare: is JSON output complete and valid?
- Check: any interactive prompts that would block?

### Step 3: Test each CLI
- `claude -p "PONG" --output-format json --dangerously-skip-permissions` via pipe
- `codex exec --json --sandbox danger-full-access "PONG"` via pipe
- `gemini --yolo --output-format json "PONG"` via pipe
- Verify: no prompts, no blocking, valid JSON

### Step 4: Document findings
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
claude -p "PONG" --output-format json --dangerously-skip-permissions 2>/dev/null | python3 -m json.tool
codex exec --json --sandbox danger-full-access "PONG" 2>/dev/null | tail -1 | python3 -m json.tool
gemini --yolo --output-format json "PONG" 2>/dev/null | python3 -m json.tool
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

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4.1
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
