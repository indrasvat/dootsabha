// Spike 007: Terminal UX Foundations
// Validates: lipgloss rendering under TTY/pipe/NO_COLOR/narrow, huh spinner stderr isolation,
// and all 4 lipgloss pitfalls from PRD §8.4.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/term"
	"github.com/mattn/go-isatty"
)

// ── Color palette from PRD §8.2 ─────────────────────────────────────────────

var (
	ClaudeColor  = lipgloss.Color("#F59E0B") // Amber/gold
	CodexColor   = lipgloss.Color("#10B981") // Emerald
	GeminiColor  = lipgloss.Color("#3B82F6") // Blue
	ErrorColor   = lipgloss.Color("#EF4444") // Red
	SuccessColor = lipgloss.Color("#10B981") // Green
	MutedColor   = lipgloss.Color("#64748B") // Slate
	AccentColor  = lipgloss.Color("#06B6D4") // Cyan
)

// ── Environment detection ───────────────────────────────────────────────────

func isTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

func noColor() bool {
	_, set := os.LookupEnv("NO_COLOR")
	return set
}

func shouldUseColor() bool {
	return isTTY() && !noColor()
}

func termWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var w int
		fmt.Sscanf(cols, "%d", &w)
		if w > 0 {
			return w
		}
	}
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// ── Section 1: Environment report ──────────────────────────────────────────

func printEnvironment() {
	fmt.Printf("=== Environment ===\n")
	fmt.Printf("  TTY:      %v\n", isTTY())
	fmt.Printf("  NO_COLOR: %v\n", noColor())
	fmt.Printf("  Width:    %d\n", termWidth())
	fmt.Printf("  Color:    %v\n", shouldUseColor())
}

// ── Section 2: Provider dots ────────────────────────────────────────────────
// PRD §8.3: TTY+color → colored ●, TTY+NO_COLOR or pipe → *

func renderDot(color lipgloss.Color, label string) string {
	if !shouldUseColor() {
		return "* " + label
	}
	// Pitfall #4: explicit \033[0m reset after each styled element
	dot := lipgloss.NewStyle().Foreground(color).Render("●")
	name := lipgloss.NewStyle().Bold(true).Render(label)
	return dot + " " + name + "\033[0m"
}

func renderProviderDots() {
	fmt.Println("\n=== Provider Dots (PRD §8.3) ===")
	fmt.Println(renderDot(ClaudeColor, "Claude"))
	fmt.Println(renderDot(CodexColor, "Codex"))
	fmt.Println(renderDot(GeminiColor, "Gemini"))
}

// ── Section 3: Provider status table ───────────────────────────────────────
// Uses lipgloss/table. Degrades to plain borders without color.

func renderProviderTable() {
	fmt.Println("\n=== Provider Status Table (PRD §8.3) ===")

	// Pitfall #2: avoid lipgloss.Width() on inner modal elements.
	// Use termWidth() directly and limit table width manually.
	width := termWidth()
	tableWidth := width - 4
	if tableWidth < 40 {
		tableWidth = 40
	}

	var t *table.Table

	if shouldUseColor() {
		headerStyle := lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

		rowStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0"))

		t = table.New().
			Border(lipgloss.NormalBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(MutedColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				return rowStyle
			}).
			Width(tableWidth).
			Headers("Provider", "Status", "●", "Version").
			Row("Claude",
				lipgloss.NewStyle().Foreground(SuccessColor).Render("✓ Ready"),
				lipgloss.NewStyle().Foreground(ClaudeColor).Render("●"),
				"claude 2.1.63").
			Row("Codex",
				lipgloss.NewStyle().Foreground(SuccessColor).Render("✓ Ready"),
				lipgloss.NewStyle().Foreground(CodexColor).Render("●"),
				"codex 0.106.0").
			Row("Gemini",
				lipgloss.NewStyle().Foreground(SuccessColor).Render("✓ Ready"),
				lipgloss.NewStyle().Foreground(GeminiColor).Render("●"),
				"gemini 0.30.0")
	} else {
		// Degraded: no color, use ASCII-safe status markers per §8.3
		t = table.New().
			Border(lipgloss.NormalBorder()).
			Width(tableWidth).
			Headers("Provider", "Status", ".", "Version").
			Row("Claude", "OK Ready", "*", "claude 2.1.63").
			Row("Codex", "OK Ready", "*", "codex 0.106.0").
			Row("Gemini", "OK Ready", "*", "gemini 0.30.0")
	}

	rendered := t.Render()
	// Pitfall #4: explicit ANSI reset after table rendering to prevent color bleed.
	// ONLY emit the reset when color is active — never inject ANSI into piped output.
	if shouldUseColor() {
		fmt.Println(rendered + "\033[0m")
	} else {
		fmt.Println(rendered)
	}
}

// ── Section 4: Spinner on stderr ────────────────────────────────────────────
// PRD §8.1: spinners go to stderr, stdout stays clean (parseable).
// PRD §8.3: when piped (not TTY), silence the spinner entirely.
//
// huh v0.8.0 removed the standalone spinner; we demonstrate the pattern
// using a raw goroutine writing to stderr — same isolation contract.

func runSpinnerOnStderr() {
	fmt.Println("\n=== Spinner Isolation (PRD §8.1: stderr only) ===")

	if !isTTY() {
		// §8.3: piped context → silence spinner, nothing on stderr either
		fmt.Println("(non-TTY: spinner suppressed — stdout-only mode)")
		return
	}

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan struct{})

	// Spinner goroutine writes exclusively to stderr
	go func() {
		i := 0
		for {
			select {
			case <-done:
				// Clear spinner line on stderr
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			default:
				frame := frames[i%len(frames)]
				fmt.Fprintf(os.Stderr, "\r%s Consulting council...", frame)
				time.Sleep(80 * time.Millisecond)
				i++
			}
		}
	}()

	// Simulate 600ms of work
	time.Sleep(600 * time.Millisecond)
	close(done)
	time.Sleep(50 * time.Millisecond) // let goroutine clear line

	// This stdout line should appear cleanly, with no spinner contamination
	fmt.Println("Spinner done — this stdout line appeared after spinner cleared.")
	fmt.Println("Verify: piped output should have ZERO ANSI codes from spinner.")
}

// ── Section 5: Lipgloss pitfall tests ──────────────────────────────────────
// All 4 pitfalls from PRD §8.4.

func testPitfalls() {
	fmt.Println("\n=== Lipgloss Pitfall Tests (PRD §8.4) ===")

	// ── Pitfall 1: Background bleed ─────────────────────────────────────
	// lipgloss.Background() only affects rendered chars; empty cells bleed.
	// Fix: use termenv.SetBackgroundColor() before render and output.Reset() after.
	fmt.Println("\n[Pitfall 1] Background bleed — check if adjacent text is tinted:")
	if shouldUseColor() {
		bgStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#1E293B")).
			Foreground(lipgloss.Color("#F1F5F9")).
			Padding(0, 1)
		bgCell := bgStyle.Render("dark-bg cell")
		// Without Reset(), adjacent text may inherit background on some terminals
		fmt.Printf("  %s adjacent text (should be normal color) \033[0m\n", bgCell)
		fmt.Println("  OBSERVATION: Does 'adjacent text' look tinted? That's bleed.")
		fmt.Println("  FIX: termenv.SetBackgroundColor() + output.Reset() around render.")
	} else {
		fmt.Println("  (color disabled — bleed test N/A)")
	}

	// ── Pitfall 2: Width calculation ────────────────────────────────────
	// AVOID lipgloss.Width() on inner/modal elements — causes padding bleed.
	// Use strings.Repeat(" ", delta) for manual padding instead.
	fmt.Println("\n[Pitfall 2] Width calculation — manual padding (safe approach):")
	inner := "inner content"
	targetWidth := 20
	// SAFE: manual padding, not lipgloss.Width()
	delta := targetWidth - len(inner)
	if delta < 0 {
		delta = 0
	}
	padded := inner + strings.Repeat(" ", delta)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Render(padded)
	fmt.Println(box)
	fmt.Println("  (Used strings.Repeat, not lipgloss.Width — avoids padding bleed)")

	// ── Pitfall 3: Switch shadowing ─────────────────────────────────────
	// WRONG: msg := msg.(type)   — shadows outer var, modifications don't propagate
	// RIGHT: typedMsg := msg.(type) — distinct name, safe to use
	fmt.Println("\n[Pitfall 3] Type-switch shadowing — compile-time pattern check:")
	type Msg interface{}
	type PingMsg struct{ Val string }
	var msg Msg = PingMsg{Val: "hello from spike"}

	// Correct pattern
	switch typedMsg := msg.(type) {
	case PingMsg:
		fmt.Printf("  CORRECT: typedMsg.Val = %q (outer 'msg' still intact)\n", typedMsg.Val)
	}
	// Demonstrate outer var is still accessible
	fmt.Printf("  outer msg type: %T\n", msg)

	// ── Pitfall 4: ANSI reset between styled elements ───────────────────
	// Without explicit \033[0m, color can bleed into adjacent unstyled text.
	fmt.Println("\n[Pitfall 4] ANSI reset between styled elements:")
	if shouldUseColor() {
		style1 := lipgloss.NewStyle().Foreground(ClaudeColor).Bold(true)
		style2 := lipgloss.NewStyle().Foreground(GeminiColor).Bold(true)
		muted := lipgloss.NewStyle().Foreground(MutedColor)

		// WITH explicit resets (correct)
		with := style1.Render("amber") + "\033[0m" + " | " + style2.Render("blue") + "\033[0m"
		// WITHOUT explicit resets (may bleed)
		without := style1.Render("amber") + " | " + style2.Render("blue")

		fmt.Println("  WITH explicit resets:", with)
		fmt.Println("  WITHOUT resets:      ", without)
		fmt.Println(muted.Render("  (check if ' | ' separator above inherited any color)"))
	} else {
		fmt.Println("  (color disabled — reset test N/A)")
	}
}

// ── Section 6: Degradation summary ─────────────────────────────────────────

func printDegradationSummary() {
	fmt.Println("\n=== Degradation Matrix Summary (PRD §8.3) ===")
	rows := [][]string{
		{"Provider dot", "colored ●", "* (plain)", "* (plain)"},
		{"Status OK", "✓ ready (green)", "OK ready", "OK ready"},
		{"Spinner", "animated stderr", "static stderr", "silent"},
		{"Table", "lipgloss+color", "lipgloss no-color", "tab-sep"},
	}
	headers := []string{"Element", "TTY+Color", "TTY+NO_COLOR", "Piped"}
	_ = rows
	_ = headers

	// In a real implementation this would render a table.
	// For spike: just describe current mode.
	mode := "Piped (no TTY)"
	if isTTY() && shouldUseColor() {
		mode = "TTY + Color"
	} else if isTTY() && !shouldUseColor() {
		mode = "TTY + NO_COLOR"
	}
	fmt.Printf("  Current mode: %s\n", mode)
	fmt.Println("  Run with '| cat' and 'NO_COLOR=1' to verify other modes.")
}

// ── main ────────────────────────────────────────────────────────────────────

func main() {
	printEnvironment()
	renderProviderDots()
	renderProviderTable()
	testPitfalls()
	runSpinnerOnStderr()
	printDegradationSummary()

	fmt.Println("\n=== Spike complete. ===")
}
