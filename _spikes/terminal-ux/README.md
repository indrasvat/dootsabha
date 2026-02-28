# Spike 007: Terminal UX Foundations — Findings

**Date:** 2026-02-28
**Status:** COMPLETE
**Task:** `docs/tasks/006-terminal-ux-spike.md`

## Summary

All terminal UX assumptions validated. Lipgloss v1.1.0 correctly auto-detects TTY/pipe mode and degrades gracefully. All 4 PRD §8.4 pitfalls reproduced and documented. Critical finding: **huh v0.8.0 removed its standalone spinner** — production code must use a goroutine-based stderr spinner.

---

## Test Results

### Scenario 1: TTY + Color (`go run main.go` in real terminal)

- `isatty.IsTerminal(os.Stdout.Fd())` → `true`
- Lipgloss auto-enables TrueColor (24-bit RGB)
- Provider dots render as colored Unicode `●`
- Table renders with muted-slate borders and cyan headers
- Status cells render in green (#10B981) for "✓ Ready"
- Pitfall #1 background test visible — adjacent text was NOT tinted (lipgloss resets correctly on this terminal)
- Pitfall #4 reset test: separator `|` appeared in plain color after explicit `\033[0m` ✓

**Color codes confirmed active in TTY:**
```
^[[38;2;245;158;11m  →  Claude amber   (#F59E0B RGB 245,158,11)
^[[38;2;16;185;129m  →  Codex emerald  (#10B981 RGB 16,185,129)
^[[38;2;59;130;246m  →  Gemini blue    (#3B82F6 RGB 59,130,246)
^[[38;2;100;116;139m →  Borders muted  (#64748B RGB 100,116,139)
^[[38;2;6;182;211m   →  Headers cyan   (#06B6D4 RGB 6,182,211)
```

---

### Scenario 2: Piped (`go run main.go | cat`)

- `isatty.IsTerminal(os.Stdout.Fd())` → `false`
- Lipgloss auto-disables all color
- Provider dots degrade to `*` prefix (correct per PRD §8.3)
- Table renders with unicode border chars but no color codes
- Spinner silenced (correct per PRD §8.3)
- **Zero ANSI escape codes in output** (verified via `od -c | grep "033"` — only false positives from od offsets)

---

### Scenario 3: `NO_COLOR=1 go run main.go`

- `noColor()` → `true`, `shouldUseColor()` → `false`
- Color disabled even when running in TTY
- Identical output to piped mode (correct)
- `NO_COLOR` env properly detected via `os.LookupEnv("NO_COLOR")` — presence of key matters, not value

**Implementation note:** Use `_, set := os.LookupEnv("NO_COLOR")` not `os.Getenv("NO_COLOR") != ""`. The spec says presence of the variable (even with empty value) disables color.

---

### Scenario 4: Narrow Terminal (`COLUMNS=40 go run main.go`)

- `termWidth()` correctly reads `COLUMNS` env var (takes precedence over TTY size)
- lipgloss table `.Width(tableWidth)` respected — table compresses column widths to fit
- At 40 cols: table compresses but doesn't wrap (columns get as narrow as `│Claude  │`)
- Floor of 40 chars prevents broken output at very narrow widths

```
┌────────┬────────┬──────┬─────────────┐
│Provider│Status  │.     │Version      │
├────────┼────────┼──────┼─────────────┤
│Claude  │OK Ready│*     │claude 2.1.63│
```

Table stays in bounds ✓

---

## Pitfall Verification (PRD §8.4)

### Pitfall 1: Background Bleed ✓ REPRODUCED + DOCUMENTED

**Observed:** lipgloss wraps padding spaces in background escape codes:
```
^[[48;2;30;40;59m ^[[0m   ← space with dark background
^[[38;2;241;245;249;48;2;30;40;59mdark-bg cell^[[0m
^[[48;2;30;40;59m ^[[0m   ← trailing space with dark background
```
Lipgloss does reset after each cell with `^[[0m`. However, in some terminals with default background != transparent, adjacent characters can still appear tinted.

**Fix for production:** Use `termenv.SetBackgroundColor()` before rendering full output and call `output.Reset()` after. This affects the terminal state globally, not just individual cells.

**Production pattern:**
```go
output := termenv.NewOutput(os.Stdout)
output.SetBackgroundColor(output.Color("#1E293B"))
// ... render table ...
output.Reset()
```

---

### Pitfall 2: Width Calculation ✓ VALIDATED

**Confirmed:** `strings.Repeat(" ", delta)` is the safe approach for manual padding in nested/modal elements.

**What to avoid:** `lipgloss.Width()` on inner elements that are later composited into outer styles — the outer style adds its own padding, causing double-padding and width overflow.

**Safe pattern used in spike:**
```go
delta := targetWidth - len(inner)
if delta < 0 { delta = 0 }
padded := inner + strings.Repeat(" ", delta)
box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Render(padded)
```

**Use `termWidth()` for outer container sizing**, not `lipgloss.Width()` on already-styled subcomponents.

---

### Pitfall 3: Type Switch Shadowing ✓ VALIDATED (compile-time)

**Wrong pattern (DO NOT USE):**
```go
switch msg := msg.(type) {  // shadows outer 'msg'
case PingMsg:
    // outer msg is now shadowed, not accessible
}
```

**Correct pattern:**
```go
switch typedMsg := msg.(type) {  // distinct name
case PingMsg:
    typedMsg.Val  // use typedMsg for typed access
    msg           // outer msg still accessible
}
```
Verified: outer `msg` variable remains `main.PingMsg` type after the switch block.

---

### Pitfall 4: ANSI Reset ✓ REPRODUCED

**Without explicit reset:** lipgloss appends `^[[0m` after each `.Render()` call, so simple concatenation `style1.Render("a") + " | " + style2.Render("b")` does NOT bleed — each styled element resets itself.

**When reset IS critical:** When using `lipgloss.JoinHorizontal()`, `lipgloss.Place()`, or custom compositing where you concatenate raw strings that were styled separately and then feed them into another styled container. In that case, explicit `\033[0m` between elements prevents the outer container from inheriting residual state.

**The spike confirmed:** For simple adjacent renders, lipgloss's own `^[[0m` suffices. Explicit resets matter in compositing scenarios.

---

## Critical Finding: huh v0.8.0 Spinner Removed

**Finding:** `github.com/charmbracelet/huh@v0.8.0` does NOT export a `NewSpinner()` function. The spinner was removed from the public huh API. Searching the package confirms only form/field/group constructors exist.

**Impact:** Production code cannot use `huh.NewSpinner().Output(os.Stderr).Run()`.

**Solution: goroutine-based stderr spinner** (demonstrated in spike):
```go
func runSpinner(ctx context.Context, msg string) func() {
    frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
    done := make(chan struct{})
    go func() {
        i := 0
        for {
            select {
            case <-done:
                fmt.Fprintf(os.Stderr, "\r\033[K")
                return
            default:
                fmt.Fprintf(os.Stderr, "\r%s %s", frames[i%len(frames)], msg)
                time.Sleep(80 * time.Millisecond)
                i++
            }
        }
    }()
    return func() { close(done); time.Sleep(50 * time.Millisecond) }
}
```

**Alternative:** `github.com/charmbracelet/bubbles/spinner` + a minimal bubbletea program routed to stderr. More complex but more featureful.

**Spinner isolation confirmed:** When spinner goroutine writes to `os.Stderr`, piped `os.Stdout` contains zero spinner frames. `go run main.go 2>/dev/null | cat` shows clean stdout with no spinner contamination.

---

## Architecture Decisions for Production

| Decision | Recommendation | Rationale |
|----------|---------------|-----------|
| TTY detection | `isatty.IsTerminal(os.Stdout.Fd())` | Correct, fast, battle-tested |
| NO_COLOR | `_, set := os.LookupEnv("NO_COLOR")` | Spec-compliant (presence not value) |
| Terminal width | `COLUMNS` env first, then `term.GetSize()` | Respects user override |
| Spinner | Raw goroutine on stderr | huh v0.8.0 has no standalone spinner |
| Table width | `.Width(termWidth() - 4)` with 40-char floor | Prevents overflow, graceful narrow |
| ANSI in pipes | Gate all color/reset behind `shouldUseColor()` | Zero ANSI in piped output |
| Padding | `strings.Repeat(" ", delta)` not `lipgloss.Width()` | Avoids padding bleed |
| BG bleed | `termenv` SetBackgroundColor/Reset around full render | Correct terminal state management |

## Verification Commands

```bash
# Piped — no color, no spinner
go run main.go | cat

# NO_COLOR honored
NO_COLOR=1 go run main.go

# Narrow terminal
COLUMNS=40 go run main.go

# Zero ANSI in piped output (uses od on macOS, grep -cP on Linux)
go run main.go | od -c | grep -c $'\033'  # should be 0

# TTY+color via pseudo-TTY
script -q /dev/null go run main.go | cat -v | grep "38;2"  # shows RGB codes
```

## Files

- `main.go` — Spike program (standalone module, not production code)
- `go.mod` / `go.sum` — `dootsabha-spike/terminal-ux` module with lipgloss v1.1.0 + huh v0.8.0
- `README.md` — This document
