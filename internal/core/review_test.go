package core_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

func TestPeerReviewThreeAgents(t *testing.T) {
	agents := []core.Agent{
		okAgent("claude", "c-out"),
		okAgent("codex", "x-out"),
		okAgent("gemini", "g-out"),
	}
	eng := core.NewEngine(agents, defaultCfg())

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "claude output"},
		{Provider: "codex", Content: "codex output"},
		{Provider: "gemini", Content: "gemini output"},
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
		okAgent("gemini", "g-out"),
	}
	eng := core.NewEngine(agents, defaultCfg())

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "claude output"},
		{Provider: "codex", Error: fmt.Errorf("codex failed")},
		{Provider: "gemini", Content: "gemini output"},
	}

	reviews, err := eng.PeerReview(context.Background(), dispatches, core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only claude and gemini succeeded → 2 reviews
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
