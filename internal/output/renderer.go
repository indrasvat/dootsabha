// Package output provides render context and output helpers for दूतसभा.
// All terminal output flows through RenderContext to ensure correct TTY/pipe/color behaviour.
package output

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/mattn/go-isatty"
)

// RenderContext carries detected terminal capabilities for the current invocation.
// Construct once at startup and pass through the call stack.
type RenderContext struct {
	// IsTTY is true when stdout is an interactive terminal.
	IsTTY bool
	// HasColor is true when TTY is active AND NO_COLOR is not set.
	HasColor bool
	// Width is the usable terminal column count (min 40, default 80 when unknown).
	Width int
	// Format is "text" (default) or "json" (from --json flag).
	Format string
}

// NewRenderContext detects terminal capabilities and builds a RenderContext.
//
// stdout is typically os.Stdout. jsonFlag enables JSON-only output mode.
func NewRenderContext(stdout *os.File, jsonFlag bool) *RenderContext {
	isTTY := isatty.IsTerminal(stdout.Fd())
	_, noColorSet := os.LookupEnv("NO_COLOR")
	hasColor := isTTY && !noColorSet

	width := termWidth(stdout)

	format := "text"
	if jsonFlag {
		format = "json"
	}

	return &RenderContext{
		IsTTY:    isTTY,
		HasColor: hasColor,
		Width:    width,
		Format:   format,
	}
}

// IsJSON reports whether the render context is in JSON output mode.
func (rc *RenderContext) IsJSON() bool {
	return rc.Format == "json"
}

// termWidth reads terminal width from COLUMNS env var first, then via syscall.
// Returns a value in the range [40, ∞) with a fallback of 80.
func termWidth(stdout *os.File) int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var w int
		if _, err := fmt.Sscanf(cols, "%d", &w); err == nil && w > 0 {
			if w < 40 {
				return 40
			}
			return w
		}
	}
	w, _, err := term.GetSize(stdout.Fd())
	if err != nil || w <= 0 {
		return 80
	}
	if w < 40 {
		return 40
	}
	return w
}
