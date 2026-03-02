package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

func TestBuildExecuteResponseDispatch(t *testing.T) {
	dispatches := []core.DispatchResult{
		{
			Provider:  "claude",
			Model:     "sonnet-4",
			Content:   "Claude response",
			Duration:  100 * time.Millisecond,
			CostUSD:   0.001,
			TokensIn:  50,
			TokensOut: 100,
		},
		{
			Provider:  "codex",
			Model:     "o4-mini",
			Content:   "Codex response",
			Duration:  200 * time.Millisecond,
			CostUSD:   0.002,
			TokensIn:  60,
			TokensOut: 120,
		},
	}

	resp := buildExecuteResponse(dispatches, nil, nil, 300*time.Millisecond)

	if len(resp.DispatchResults) != 2 {
		t.Fatalf("dispatch count = %d, want 2", len(resp.DispatchResults))
	}
	if resp.DispatchResults[0].Provider != "claude" {
		t.Errorf("dispatch[0] provider = %q", resp.DispatchResults[0].Provider)
	}
	if resp.DispatchResults[1].Provider != "codex" {
		t.Errorf("dispatch[1] provider = %q", resp.DispatchResults[1].Provider)
	}
	if resp.Metadata == nil {
		t.Fatal("metadata is nil")
	}
	if resp.Metadata.TotalCostUsd != 0.003 {
		t.Errorf("total cost = %f, want 0.003", resp.Metadata.TotalCostUsd)
	}
}

func TestBuildExecuteResponseDispatchTokens(t *testing.T) {
	dispatches := []core.DispatchResult{
		{Provider: "claude", TokensIn: 50, TokensOut: 100},
		{Provider: "codex", TokensIn: 60, TokensOut: 120},
	}

	resp := buildExecuteResponse(dispatches, nil, nil, 100*time.Millisecond)

	if resp.Metadata.TotalTokensIn != 110 {
		t.Errorf("total tokens in = %d, want 110", resp.Metadata.TotalTokensIn)
	}
	if resp.Metadata.TotalTokensOut != 220 {
		t.Errorf("total tokens out = %d, want 220", resp.Metadata.TotalTokensOut)
	}
}

func TestBuildExecuteResponseDispatchDuration(t *testing.T) {
	dispatches := []core.DispatchResult{
		{Provider: "claude", Duration: 150 * time.Millisecond},
	}

	resp := buildExecuteResponse(dispatches, nil, nil, 300*time.Millisecond)

	if resp.DispatchResults[0].DurationMs != 150 {
		t.Errorf("dispatch duration = %d, want 150", resp.DispatchResults[0].DurationMs)
	}
	if resp.Metadata.TotalDurationMs != 300 {
		t.Errorf("total duration = %d, want 300", resp.Metadata.TotalDurationMs)
	}
}

func TestBuildExecuteResponseFullPipeline(t *testing.T) {
	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "A", CostUSD: 0.001, TokensIn: 10, TokensOut: 20},
		{Provider: "codex", Content: "B", CostUSD: 0.002, TokensIn: 15, TokensOut: 25},
	}
	reviews := []core.ReviewResult{
		{Reviewer: "claude", Reviewed: []string{"codex"}, Content: "Good", CostUSD: 0.0005},
		{Reviewer: "codex", Reviewed: []string{"claude"}, Content: "Good", CostUSD: 0.0005},
	}
	synthesis := &core.SynthesisResult{
		Chair:   "claude",
		Content: "Synthesized",
		CostUSD: 0.003,
	}

	resp := buildExecuteResponse(dispatches, reviews, synthesis, 500*time.Millisecond)

	if len(resp.DispatchResults) != 2 {
		t.Errorf("dispatch count = %d", len(resp.DispatchResults))
	}
	if len(resp.ReviewResults) != 2 {
		t.Errorf("review count = %d", len(resp.ReviewResults))
	}
	if resp.Synthesis == nil {
		t.Fatal("synthesis is nil")
	}
	if resp.Synthesis.Chair != "claude" {
		t.Errorf("chair = %q", resp.Synthesis.Chair)
	}
	if resp.Synthesis.Content != "Synthesized" {
		t.Errorf("synthesis content = %q", resp.Synthesis.Content)
	}

	expectedCost := 0.001 + 0.002 + 0.0005 + 0.0005 + 0.003
	if resp.Metadata.TotalCostUsd != expectedCost {
		t.Errorf("total cost = %f, want %f", resp.Metadata.TotalCostUsd, expectedCost)
	}
}

func TestBuildExecuteResponseWithErrors(t *testing.T) {
	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "OK", CostUSD: 0.001},
		{Provider: "codex", Error: fmt.Errorf("timeout")},
	}

	resp := buildExecuteResponse(dispatches, nil, nil, 100*time.Millisecond)

	if resp.DispatchResults[0].Error != "" {
		t.Error("claude should have no error")
	}
	if resp.DispatchResults[1].Error != "timeout" {
		t.Errorf("codex error = %q, want timeout", resp.DispatchResults[1].Error)
	}
	if resp.Metadata.ProvidersStatus["claude"] != "healthy" {
		t.Error("claude should be healthy")
	}
	if resp.Metadata.ProvidersStatus["codex"] != "error" {
		t.Error("codex should be error")
	}
}

func TestBuildExecuteResponseWithFallback(t *testing.T) {
	synthesis := &core.SynthesisResult{
		Chair:         "claude",
		ChairFallback: "codex",
		Content:       "Fallback synthesis",
	}

	resp := buildExecuteResponse(nil, nil, synthesis, 100*time.Millisecond)

	if resp.Synthesis.Chair != "claude" {
		t.Errorf("chair = %q", resp.Synthesis.Chair)
	}
	if resp.Synthesis.ChairFallback != "codex" {
		t.Errorf("fallback = %q", resp.Synthesis.ChairFallback)
	}
}

func TestBuildExecuteResponseNilSynthesis(t *testing.T) {
	resp := buildExecuteResponse(nil, nil, nil, 100*time.Millisecond)

	if resp.Synthesis != nil {
		t.Error("synthesis should be nil when not provided")
	}
	if resp.Metadata == nil {
		t.Fatal("metadata should never be nil")
	}
	if resp.Metadata.TotalCostUsd != 0 {
		t.Errorf("total cost = %f, want 0", resp.Metadata.TotalCostUsd)
	}
}

func TestBuildExecuteResponseEmptyDispatches(t *testing.T) {
	resp := buildExecuteResponse([]core.DispatchResult{}, nil, nil, 50*time.Millisecond)

	if len(resp.DispatchResults) != 0 {
		t.Errorf("dispatch count = %d, want 0", len(resp.DispatchResults))
	}
	if resp.Metadata.TotalDurationMs != 50 {
		t.Errorf("total duration = %d, want 50", resp.Metadata.TotalDurationMs)
	}
}

func TestBuildExecuteResponseReviewTokens(t *testing.T) {
	reviews := []core.ReviewResult{
		{Reviewer: "claude", Reviewed: []string{"codex"}, TokensIn: 100, TokensOut: 200, CostUSD: 0.01},
		{Reviewer: "codex", Reviewed: []string{"claude"}, TokensIn: 150, TokensOut: 250, CostUSD: 0.02},
	}

	resp := buildExecuteResponse(nil, reviews, nil, 100*time.Millisecond)

	if resp.Metadata.TotalTokensIn != 250 {
		t.Errorf("total tokens in = %d, want 250", resp.Metadata.TotalTokensIn)
	}
	if resp.Metadata.TotalTokensOut != 450 {
		t.Errorf("total tokens out = %d, want 450", resp.Metadata.TotalTokensOut)
	}
	if resp.Metadata.TotalCostUsd != 0.03 {
		t.Errorf("total cost = %f, want 0.03", resp.Metadata.TotalCostUsd)
	}
}

func TestBuildExecuteResponseReviewErrors(t *testing.T) {
	reviews := []core.ReviewResult{
		{Reviewer: "claude", Error: fmt.Errorf("review failed")},
	}

	resp := buildExecuteResponse(nil, reviews, nil, 100*time.Millisecond)

	if resp.ReviewResults[0].Error != "review failed" {
		t.Errorf("review error = %q, want 'review failed'", resp.ReviewResults[0].Error)
	}
}

func TestBuildExecuteResponseSynthesisTokens(t *testing.T) {
	synthesis := &core.SynthesisResult{
		Chair:     "claude",
		Content:   "Final answer",
		TokensIn:  500,
		TokensOut: 1000,
		CostUSD:   0.05,
		Duration:  250 * time.Millisecond,
	}

	resp := buildExecuteResponse(nil, nil, synthesis, 300*time.Millisecond)

	if resp.Synthesis.TokensIn != 500 {
		t.Errorf("synthesis tokens in = %d, want 500", resp.Synthesis.TokensIn)
	}
	if resp.Synthesis.TokensOut != 1000 {
		t.Errorf("synthesis tokens out = %d, want 1000", resp.Synthesis.TokensOut)
	}
	if resp.Synthesis.DurationMs != 250 {
		t.Errorf("synthesis duration = %d, want 250", resp.Synthesis.DurationMs)
	}
	if resp.Metadata.TotalCostUsd != 0.05 {
		t.Errorf("total cost = %f, want 0.05", resp.Metadata.TotalCostUsd)
	}
}

func TestBuildExecuteResponseProvidersStatusMap(t *testing.T) {
	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "OK"},
		{Provider: "codex", Error: fmt.Errorf("fail")},
		{Provider: "gemini", Content: "OK"},
	}

	resp := buildExecuteResponse(dispatches, nil, nil, 100*time.Millisecond)

	if len(resp.Metadata.ProvidersStatus) != 3 {
		t.Fatalf("providers status count = %d, want 3", len(resp.Metadata.ProvidersStatus))
	}
	for _, tc := range []struct {
		provider string
		want     string
	}{
		{"claude", "healthy"},
		{"codex", "error"},
		{"gemini", "healthy"},
	} {
		if got := resp.Metadata.ProvidersStatus[tc.provider]; got != tc.want {
			t.Errorf("provider %s status = %q, want %q", tc.provider, got, tc.want)
		}
	}
}
