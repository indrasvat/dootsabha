package output

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Provider and semantic color constants from PRD §8.2.
var (
	ClaudeColor  = lipgloss.Color("#F59E0B") // Amber/gold
	CodexColor   = lipgloss.Color("#10B981") // Emerald
	GeminiColor  = lipgloss.Color("#3B82F6") // Blue
	ErrorColor   = lipgloss.Color("#EF4444") // Red
	SuccessColor = lipgloss.Color("#22C55E") // Green
	WarnColor    = lipgloss.Color("#EAB308") // Yellow
	MutedColor   = lipgloss.Color("#64748B") // Slate
	AccentColor  = lipgloss.Color("#06B6D4") // Cyan
)

// ProviderDot returns the provider indicator glyph per PRD §8.3 degradation rules.
//
// TTY + color → colored "●"
// TTY + NO_COLOR or piped → plain "*"
func ProviderDot(rc *RenderContext, color lipgloss.Color) string {
	if !rc.HasColor {
		return "*"
	}
	// Pitfall #4: explicit reset after styled element to prevent bleed into adjacent text.
	return lipgloss.NewStyle().Foreground(color).Render("●") + "\033[0m"
}

// StatusOK returns the success indicator per PRD §8.3.
//
// TTY → "✓"  Piped → "OK"
func StatusOK(rc *RenderContext) string {
	if rc.IsTTY {
		if rc.HasColor {
			return lipgloss.NewStyle().Foreground(SuccessColor).Render("✓")
		}
		return "✓"
	}
	return "OK"
}

// StatusFail returns the failure indicator per PRD §8.3.
//
// TTY → "✗"  Piped → "FAIL"
func StatusFail(rc *RenderContext) string {
	if rc.IsTTY {
		if rc.HasColor {
			return lipgloss.NewStyle().Foreground(ErrorColor).Render("✗")
		}
		return "✗"
	}
	return "FAIL"
}

// Styled returns a lipgloss-rendered string only when HasColor is true.
// When HasColor is false, the plain text is returned unchanged.
func Styled(rc *RenderContext, style lipgloss.Style, text string) string {
	if !rc.HasColor {
		return text
	}
	return style.Render(text)
}

// CommandHeader renders a rounded-border box with command name and info.
//
// TTY + color: AccentColor border, bold name in top border, muted info.
// TTY + NO_COLOR: plain box-drawing characters.
// Piped: "Name\ninfo" as plain text.
func CommandHeader(rc *RenderContext, name, info string) string {
	if !rc.IsTTY {
		return name + "\n" + info
	}
	w := min(rc.Width, 60)

	nameW := lipgloss.Width(name)
	infoW := lipgloss.Width(info)

	// Truncate info if it overflows the box.
	maxInfo := w - 5 // 5 = "│  " + " " + "│"
	if infoW > maxInfo && maxInfo > 3 {
		// Truncate to fit with "…" suffix.
		runes := []rune(info)
		for len(runes) > 0 && lipgloss.Width(string(runes)) > maxInfo-1 {
			runes = runes[:len(runes)-1]
		}
		info = string(runes) + "…"
		infoW = lipgloss.Width(info)
	}

	// Top: ┌ Name ──────────────────────────────┐
	topFill := max(w-nameW-4, 1) // 4 = "┌ " + " " + "┐"

	// Middle: │  info                             │
	midFill := max(w-infoW-4, 1) // 4 = "│  " + "│" (visual widths: 1+2 + 1)

	// Bottom: └─────────────────────────────────┘
	botFill := max(w-2, 1) // 2 = "└" + "┘"

	if !rc.HasColor {
		top := "┌ " + name + " " + strings.Repeat("─", topFill) + "┐"
		mid := "│  " + info + strings.Repeat(" ", midFill) + "│"
		bot := "└" + strings.Repeat("─", botFill) + "┘"
		return top + "\n" + mid + "\n" + bot
	}

	b := lipgloss.NewStyle().Foreground(AccentColor)
	n := lipgloss.NewStyle().Bold(true).Foreground(AccentColor)
	inf := lipgloss.NewStyle().Foreground(MutedColor)

	top := b.Render("┌") + " " + n.Render(name) + " " + b.Render(strings.Repeat("─", topFill)+"┐")
	mid := b.Render("│") + "  " + inf.Render(info) + strings.Repeat(" ", midFill) + b.Render("│")
	bot := b.Render("└" + strings.Repeat("─", botFill) + "┘")

	return top + "\n" + mid + "\n" + bot
}

// SectionDivider renders a labeled section divider that fills the terminal width.
//
// TTY + color: "── Label ── info ─────" with AccentColor label, MutedColor lines.
// TTY + NO_COLOR: same but plain.
// Piped: "--- Label --- info ---" or "--- Label ---".
func SectionDivider(rc *RenderContext, label, info string) string {
	if !rc.IsTTY {
		if info != "" {
			return fmt.Sprintf("--- %s --- %s ---", label, info)
		}
		return fmt.Sprintf("--- %s ---", label)
	}
	w := min(rc.Width, 60)
	labelW := lipgloss.Width(label)

	var fill int
	if info != "" {
		infoW := lipgloss.Width(info)
		// "── LABEL ── INFO ─────"
		fill = w - 8 - labelW - infoW // 8 = "── " + " ── " + " "
	} else {
		// "── LABEL ─────"
		fill = w - 4 - labelW // 4 = "── " + " "
	}
	fill = max(fill, 3)

	if !rc.HasColor {
		if info != "" {
			return "── " + label + " ── " + info + " " + strings.Repeat("─", fill)
		}
		return "── " + label + " " + strings.Repeat("─", fill)
	}

	m := lipgloss.NewStyle().Foreground(MutedColor)
	a := lipgloss.NewStyle().Foreground(AccentColor)

	if info != "" {
		return m.Render("──") + " " + a.Render(label) + " " + m.Render("── "+info+" "+strings.Repeat("─", fill))
	}
	return m.Render("──") + " " + a.Render(label) + " " + m.Render(strings.Repeat("─", fill))
}

// ContentSeparator renders a full-width thin line.
//
// TTY: MutedColor "────────".
// Piped: empty string (omitted).
func ContentSeparator(rc *RenderContext) string {
	if !rc.IsTTY {
		return ""
	}
	w := min(rc.Width, 60)
	line := strings.Repeat("─", w)
	if !rc.HasColor {
		return line
	}
	return lipgloss.NewStyle().Foreground(MutedColor).Render(line)
}

// FooterMetrics renders indented, pipe-delimited metric values.
//
// Output: "  val1 │ val2 │ val3" with MutedColor when available.
func FooterMetrics(rc *RenderContext, parts ...string) string {
	line := "  " + strings.Join(parts, " │ ")
	if !rc.HasColor {
		return line
	}
	return lipgloss.NewStyle().Foreground(MutedColor).Render(line)
}
