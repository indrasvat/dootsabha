package output_test

import (
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/output"
)

// --- CommandHeader ---

func TestCommandHeader_Piped(t *testing.T) {
	rc := &output.RenderContext{IsTTY: false, HasColor: false, Width: 80}
	got := output.CommandHeader(rc, "Refine", "author: claude · reviewers: codex")
	if !strings.HasPrefix(got, "Refine\n") {
		t.Errorf("piped header should start with plain name + newline, got %q", got)
	}
	if !strings.Contains(got, "author: claude") {
		t.Error("piped header should contain info text")
	}
	// No box-drawing characters in piped mode.
	if strings.Contains(got, "┌") || strings.Contains(got, "│") {
		t.Error("piped header should not contain box-drawing characters")
	}
}

func TestCommandHeader_TTY_NoColor(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: false, Width: 60}
	got := output.CommandHeader(rc, "Council", "agents: claude, codex, gemini")
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("TTY header should have 3 lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "┌ Council ") {
		t.Errorf("top border should start with '┌ Council ', got %q", lines[0])
	}
	if !strings.HasSuffix(lines[0], "┐") {
		t.Errorf("top border should end with '┐', got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "│  agents:") {
		t.Errorf("middle should start with '│  agents:', got %q", lines[1])
	}
	if !strings.HasSuffix(lines[1], "│") {
		t.Errorf("middle should end with '│', got %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "└") || !strings.HasSuffix(lines[2], "┘") {
		t.Errorf("bottom should be └───┘, got %q", lines[2])
	}
}

func TestCommandHeader_TTY_Color(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: true, Width: 60}
	got := output.CommandHeader(rc, "Review", "author: codex · reviewer: claude")
	// Should contain the command name and box structure.
	if !strings.Contains(got, "Review") {
		t.Error("colored header should contain command name")
	}
	if !strings.Contains(got, "author: codex") {
		t.Error("colored header should contain info text")
	}
	// Should be 3 lines.
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("colored header should have 3 lines, got %d", len(lines))
	}
}

func TestCommandHeader_Alignment(t *testing.T) {
	tests := []struct {
		name  string
		info  string
		width int
	}{
		{"Refine", "author: claude · reviewers: codex, gemini", 60},
		{"Council", "agents: claude, codex, gemini · chair: claude", 60},
		{"Review", "author: codex · reviewer: claude", 60},
		{"R", "a", 40}, // minimal
		{"LongCommandName", "some info here that is moderately long", 60},
		{"X", "this info is extremely long and will exceed the box width limit easily", 40},                // overflow → truncated
		{"Council", "agents: claude, codex, gemini · chair: claude", 80},                                   // wide terminal
		{"Refine", "author: claude · reviewers: codex, gemini, claude, gemini, codex, claude, gemini", 60}, // many reviewers
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &output.RenderContext{IsTTY: true, HasColor: false, Width: tt.width}
			got := output.CommandHeader(rc, tt.name, tt.info)
			lines := strings.Split(got, "\n")
			if len(lines) != 3 {
				t.Fatalf("expected 3 lines, got %d", len(lines))
			}
			// All lines should have the same visual width (= min(width, 60)).
			expectedW := min(tt.width, 60)
			for i, line := range lines {
				// Count runes for visual width (all ASCII + box-drawing = 1 col each).
				rw := runeCount(line)
				if rw != expectedW {
					t.Errorf("line %d: visual width %d, want %d\n  line: %q", i, rw, expectedW, line)
				}
			}
		})
	}
}

// runeCount returns the number of runes (visual columns for ASCII + box-drawing chars).
func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// --- SectionDivider ---

func TestSectionDivider_Piped(t *testing.T) {
	rc := &output.RenderContext{IsTTY: false, HasColor: false, Width: 80}
	got := output.SectionDivider(rc, "Dispatch", "3 agents")
	want := "--- Dispatch --- 3 agents ---"
	if got != want {
		t.Errorf("piped divider = %q, want %q", got, want)
	}
}

func TestSectionDivider_Piped_NoInfo(t *testing.T) {
	rc := &output.RenderContext{IsTTY: false, HasColor: false, Width: 80}
	got := output.SectionDivider(rc, "Peer Review", "")
	want := "--- Peer Review ---"
	if got != want {
		t.Errorf("piped divider = %q, want %q", got, want)
	}
}

func TestSectionDivider_TTY_NoColor(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: false, Width: 60}
	got := output.SectionDivider(rc, "Dispatch", "3 agents · parallel")
	if !strings.HasPrefix(got, "── Dispatch ── 3 agents") {
		t.Errorf("TTY divider should start with '── Dispatch ── 3 agents', got %q", got)
	}
	if !strings.Contains(got, "─") {
		t.Error("TTY divider should contain fill dashes")
	}
}

func TestSectionDivider_TTY_Color(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: true, Width: 60}
	got := output.SectionDivider(rc, "Synthesis", "chair: claude")
	if !strings.Contains(got, "Synthesis") {
		t.Error("colored divider should contain label")
	}
	if !strings.Contains(got, "chair: claude") {
		t.Error("colored divider should contain info")
	}
}

// --- ContentSeparator ---

func TestContentSeparator_Piped(t *testing.T) {
	rc := &output.RenderContext{IsTTY: false, HasColor: false, Width: 80}
	got := output.ContentSeparator(rc)
	if got != "" {
		t.Errorf("piped separator should be empty, got %q", got)
	}
}

func TestContentSeparator_TTY_NoColor(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: false, Width: 60}
	got := output.ContentSeparator(rc)
	if len(got) == 0 {
		t.Fatal("TTY separator should not be empty")
	}
	// Should be all dashes (no ANSI).
	cleaned := strings.ReplaceAll(got, "─", "")
	if cleaned != "" {
		t.Errorf("NO_COLOR separator should only contain '─', got residual: %q", cleaned)
	}
}

func TestContentSeparator_TTY_Color(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: true, Width: 60}
	got := output.ContentSeparator(rc)
	if len(got) == 0 {
		t.Error("colored separator should not be empty")
	}
	// Should contain dash characters.
	if !strings.Contains(got, "─") {
		t.Error("colored separator should contain '─' characters")
	}
}

// --- FooterMetrics ---

func TestFooterMetrics_Formatting(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: false, Width: 80}
	got := output.FooterMetrics(rc, "1.5s", "$0.050", "512 in · 1,024 out")
	want := "  1.5s │ $0.050 │ 512 in · 1,024 out"
	if got != want {
		t.Errorf("FooterMetrics = %q, want %q", got, want)
	}
}

func TestFooterMetrics_SinglePart(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: false, Width: 80}
	got := output.FooterMetrics(rc, "1.5s")
	want := "  1.5s"
	if got != want {
		t.Errorf("FooterMetrics single = %q, want %q", got, want)
	}
}

func TestFooterMetrics_Color(t *testing.T) {
	rc := &output.RenderContext{IsTTY: true, HasColor: true, Width: 80}
	got := output.FooterMetrics(rc, "1.5s", "$0.050")
	if !strings.Contains(got, "1.5s") {
		t.Error("colored FooterMetrics should contain metric values")
	}
	if !strings.Contains(got, "$0.050") {
		t.Error("colored FooterMetrics should contain cost")
	}
}
