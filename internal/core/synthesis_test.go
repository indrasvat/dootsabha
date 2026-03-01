package core_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

func TestSynthesizeSuccess(t *testing.T) {
	agents := []core.Agent{
		okAgent("claude", "synth-out"),
		okAgent("codex", "x-out"),
	}
	cfg := defaultCfg()
	cfg.Council.Chair = "claude"
	eng := core.NewEngine(agents, cfg)

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "claude output"},
		{Provider: "codex", Content: "codex output"},
	}
	reviews := []core.ReviewResult{
		{Reviewer: "claude", Reviewed: []string{"codex"}, Content: "review by claude"},
		{Reviewer: "codex", Reviewed: []string{"claude"}, Content: "review by codex"},
	}

	result, err := eng.Synthesize(context.Background(), dispatches, reviews, core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Chair != "claude" {
		t.Errorf("Chair = %q, want %q", result.Chair, "claude")
	}
	if result.ChairFallback != "" {
		t.Errorf("ChairFallback = %q, want empty", result.ChairFallback)
	}
	if result.Content == "" {
		t.Error("expected non-empty synthesis content")
	}
}

func TestSynthesizeChairFailureFallback(t *testing.T) {
	agents := []core.Agent{
		failAgent("claude"),
		okAgent("codex", "fallback-synth"),
	}
	cfg := defaultCfg()
	cfg.Council.Chair = "claude"
	eng := core.NewEngine(agents, cfg)

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "claude output"},
		{Provider: "codex", Content: "codex output"},
	}

	result, err := eng.Synthesize(context.Background(), dispatches, nil, core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Chair != "claude" {
		t.Errorf("Chair = %q, want %q", result.Chair, "claude")
	}
	if result.ChairFallback != "codex" {
		t.Errorf("ChairFallback = %q, want %q", result.ChairFallback, "codex")
	}
	if result.Content == "" {
		t.Error("expected non-empty synthesis content from fallback")
	}
}

func TestSynthesizeAllAgentsFail(t *testing.T) {
	agents := []core.Agent{
		failAgent("claude"),
		failAgent("codex"),
	}
	cfg := defaultCfg()
	cfg.Council.Chair = "claude"
	eng := core.NewEngine(agents, cfg)

	dispatches := []core.DispatchResult{
		{Provider: "claude", Error: fmt.Errorf("claude failed")},
		{Provider: "codex", Error: fmt.Errorf("codex failed")},
	}

	_, err := eng.Synthesize(context.Background(), dispatches, nil, core.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error when all agents fail")
	}
}
