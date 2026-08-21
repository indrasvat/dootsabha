package core_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/indrasvat/dootsabha/internal/core"
)

func TestPeerReviewThreeAgents(t *testing.T) {
	agents := []core.Agent{
		okAgent("claude", "c-out"),
		okAgent("codex", "x-out"),
		okAgent("agy", "g-out"),
	}
	eng := core.NewEngine(agents, defaultCfg())

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "claude output"},
		{Provider: "codex", Content: "codex output"},
		{Provider: "agy", Content: "agy output"},
	}

	reviews, err := eng.PeerReview(context.Background(), dispatches, core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 3 {
		t.Fatalf("got %d reviews, want 3", len(reviews))
	}
	for _, r := range reviews {
		if r.Error != nil {
			t.Errorf("review by %s has error: %v", r.Reviewer, r.Error)
		}
		if len(r.Reviewed) != 2 {
			t.Errorf("review by %s reviewed %d agents, want 2", r.Reviewer, len(r.Reviewed))
		}
	}
}

func TestPeerReviewTwoAgents(t *testing.T) {
	agents := []core.Agent{
		okAgent("claude", "c-out"),
		okAgent("codex", "x-out"),
	}
	eng := core.NewEngine(agents, defaultCfg())

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "claude output"},
		{Provider: "codex", Content: "codex output"},
	}

	reviews, err := eng.PeerReview(context.Background(), dispatches, core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("got %d reviews, want 2", len(reviews))
	}
	for _, r := range reviews {
		if len(r.Reviewed) != 1 {
			t.Errorf("review by %s reviewed %d agents, want 1", r.Reviewer, len(r.Reviewed))
		}
	}
}

func TestPeerReviewSkipsWithOneAgent(t *testing.T) {
	agents := []core.Agent{okAgent("claude", "out")}
	eng := core.NewEngine(agents, defaultCfg())

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "only one"},
	}

	reviews, err := eng.PeerReview(context.Background(), dispatches, core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reviews != nil {
		t.Errorf("expected nil reviews for single agent, got %d", len(reviews))
	}
}

func TestPeerReviewExcludesFailedDispatches(t *testing.T) {
	agents := []core.Agent{
		okAgent("claude", "c-out"),
		okAgent("codex", "x-out"),
		okAgent("agy", "g-out"),
	}
	eng := core.NewEngine(agents, defaultCfg())

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "claude output"},
		{Provider: "codex", Error: fmt.Errorf("codex failed")},
		{Provider: "agy", Content: "agy output"},
	}

	reviews, err := eng.PeerReview(context.Background(), dispatches, core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only claude and agy succeeded → 2 reviews
	if len(reviews) != 2 {
		t.Fatalf("got %d reviews, want 2", len(reviews))
	}
}

func TestPeerReviewTruncation(t *testing.T) {
	// Create content larger than 32KB
	bigContent := strings.Repeat("x", 40*1024)

	// Mock agent that captures the prompt it receives.
	captured := &capturingAgent{
		name: "claude",
		result: &core.InvokeResult{
			Content:  "review output",
			Duration: 100 * time.Millisecond,
		},
	}
	agents := []core.Agent{
		captured,
		okAgent("codex", "small"),
	}
	eng := core.NewEngine(agents, defaultCfg())

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "claude output"},
		{Provider: "codex", Content: bigContent},
	}

	reviews, err := eng.PeerReview(context.Background(), dispatches, core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("got %d reviews, want 2", len(reviews))
	}

	// Claude should have received a truncated version of codex's output
	if !strings.Contains(captured.lastPrompt, "[truncated]") {
		t.Error("expected truncation marker in review prompt")
	}
}

// capturingAgent records the last prompt passed to Invoke.
type capturingAgent struct {
	name       string
	result     *core.InvokeResult
	lastPrompt string
}

func (m *capturingAgent) Name() string { return m.name }
func (m *capturingAgent) Invoke(_ context.Context, prompt string, _ core.InvokeOptions) (*core.InvokeResult, error) {
	m.lastPrompt = prompt
	return m.result, nil
}

// TruncateString cut on a raw byte offset, stranding a lead byte and emitting
// invalid UTF-8. It feeds council/refine prompts (32KB) and provider error text,
// so a mid-rune cut sends broken bytes into another agent's prompt — or renders a
// Devanagari error as mojibake. Found by adversarial review.
func TestTruncateStringCutsOnRuneBoundary(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"devanagari", strings.Repeat("क", 500)}, // 3 bytes/rune — 400 is not a multiple
		{"emoji", strings.Repeat("🙏", 300)},      // 4 bytes/rune
		{"mixed", strings.Repeat("aक", 300)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := core.TruncateString(tc.in, 400)
			if !utf8.ValidString(got) {
				t.Errorf("truncated output is not valid UTF-8: %q", got)
			}
			if !strings.HasSuffix(got, "... [truncated]") {
				t.Errorf("missing truncation marker: %q", got)
			}
			if !strings.HasPrefix(tc.in, strings.TrimSuffix(got, "\n... [truncated]")) {
				t.Error("truncated output is not a prefix of the input")
			}
		})
	}

	// Pure ASCII still cuts at exactly maxBytes, and a short string is untouched.
	if got := core.TruncateString(strings.Repeat("a", 500), 400); len(got) != 400+len("\n... [truncated]") {
		t.Errorf("ASCII cut moved: len=%d", len(got))
	}
	if got := core.TruncateString("short", 400); got != "short" {
		t.Errorf("short string was modified: %q", got)
	}
}
