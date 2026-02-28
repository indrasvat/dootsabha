# Task 0.7: Terminal UX Foundations Spike

## Status: PENDING

## Depends On
- None

## Parallelizable With
- All other spikes (0.1–0.6, 0.8)

## Problem

दूतसभा uses lipgloss for all rendering and huh for spinners. We must validate: lipgloss behavior under pipe/NO_COLOR/narrow terminals, huh spinner on stderr only, color palette rendering, and the known lipgloss pitfalls from gh-ghent (background bleed, width calculation, ANSI reset).

## PRD Reference
- §8 (Terminal UX standards — full section)
- §8.2 (Color palette)
- §8.3 (Graceful degradation matrix)
- §8.4 (Lipgloss pitfalls from gh-ghent)

## Files to Create
- `_spikes/terminal-ux/main.go` — Spike program
- `_spikes/terminal-ux/README.md` — Findings doc

## Execution Steps

### Step 1: Read context
1. Read PRD §8 (all subsections)

### Step 2: Write spike program
- Render a sample provider status table using lipgloss/table
- Render colored provider dots (Claude amber, Codex emerald, Gemini blue)
- Run huh spinner on stderr while printing to stdout
- Test graceful degradation: TTY vs pipe vs NO_COLOR

### Step 3: Test scenarios
- `go run main.go` (TTY with color)
- `go run main.go | cat` (piped — no color, no spinner, no Unicode)
- `NO_COLOR=1 go run main.go` (TTY without color)
- Terminal width 40 cols — verify no ugly wrapping
- Verify: no ANSI codes in piped output (`| grep -P '\x1b\['`)

### Step 4: Verify lipgloss pitfalls
- Background bleed: test `lipgloss.Background()` rendering
- Width calculation: test `lipgloss.Width()` on nested elements
- ANSI reset: verify explicit `\033[0m` between styled elements

### Step 5: Document findings
- Which lipgloss pitfalls from §8.4 were reproduced
- huh spinner stderr isolation confirmed
- Degradation behavior documented per matrix in §8.3

## Verification

### L1: Spike runs in all modes
```bash
cd _spikes/terminal-ux
go run main.go                          # TTY + color
go run main.go | cat                    # Piped
NO_COLOR=1 go run main.go              # NO_COLOR
COLUMNS=40 go run main.go              # Narrow
go run main.go | grep -cP '\x1b\['     # Should be 0
```

## Completion Criteria

1. Lipgloss renders correctly in TTY, degrades in pipe/NO_COLOR
2. Huh spinner stays on stderr, doesn't pollute stdout
3. All 4 lipgloss pitfalls from §8.4 tested and documented
4. No ANSI in piped output confirmed

## Commit

```
spike(terminal-ux): validate lipgloss degradation and huh spinner isolation

- lipgloss rendering across TTY/pipe/NO_COLOR/narrow
- huh spinner stderr isolation confirmed
- Lipgloss pitfalls (background bleed, width calc) documented
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §8
5. Execute steps 1-5
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit
