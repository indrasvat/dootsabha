package output

import (
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
