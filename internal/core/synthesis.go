package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// SynthesisResult holds the chair's synthesized output.
type SynthesisResult struct {
	Chair         string
	ChairFallback string // non-empty if fallback chair was used
	Content       string
	Duration      time.Duration
	CostUSD       float64
	TokensIn      int
	TokensOut     int
}

// Synthesize invokes the chair agent with all dispatch outputs and reviews.
// On chair failure, re-invokes the first healthy non-chair agent.
func (e *Engine) Synthesize(ctx context.Context, dispatches []DispatchResult, reviews []ReviewResult, opts InvokeOptions) (*SynthesisResult, error) {
	prompt := buildSynthesisPrompt(dispatches, reviews)

	// Find the chair agent.
	chairName := e.cfg.Council.Chair
	slog.Info("synthesis starting", "chair", chairName, "dispatches", len(dispatches), "reviews", len(reviews))
	chair := e.findAgent(chairName)

	if chair != nil {
		e.notify(chairName, ProgressStarted)
		slog.Debug("invoking chair", "chair", chairName)
		result, err := chair.Invoke(ctx, prompt, opts)
		if err == nil {
			slog.Info("synthesis complete", "chair", chairName, "duration", result.Duration, "content_len", len(result.Content))
			e.notify(chairName, ProgressDone)
			return &SynthesisResult{
				Chair:     chairName,
				Content:   result.Content,
				Duration:  result.Duration,
				CostUSD:   result.CostUSD,
				TokensIn:  result.TokensIn,
				TokensOut: result.TokensOut,
			}, nil
		}
		slog.Warn("chair failed, trying fallback", "chair", chairName, "error", err)
		e.notify(chairName, ProgressFailed)
		// Chair failed — try fallback.
	}

	// Fallback: first healthy non-chair agent.
	fallback := e.findFallbackAgent(chairName, dispatches)
	if fallback == nil {
		return nil, fmt.Errorf("synthesize: no healthy agents available for synthesis")
	}

	fallbackName := fallback.Name()
	slog.Info("synthesis fallback", "fallback", fallbackName)
	e.notify(fallbackName, ProgressStarted)
	result, err := fallback.Invoke(ctx, prompt, opts)
	if err != nil {
		e.notify(fallbackName, ProgressFailed)
		return nil, fmt.Errorf("synthesize fallback %s: %w", fallbackName, err)
	}

	e.notify(fallbackName, ProgressDone)
	return &SynthesisResult{
		Chair:         chairName,
		ChairFallback: fallbackName,
		Content:       result.Content,
		Duration:      result.Duration,
		CostUSD:       result.CostUSD,
		TokensIn:      result.TokensIn,
		TokensOut:     result.TokensOut,
	}, nil
}

// findAgent returns the named agent or nil if not found.
func (e *Engine) findAgent(name string) Agent {
	for _, a := range e.agents {
		if a.Name() == name {
			return a
		}
	}
	return nil
}

// findFallbackAgent returns the first non-chair agent that had a successful dispatch.
func (e *Engine) findFallbackAgent(chairName string, dispatches []DispatchResult) Agent {
	for _, d := range dispatches {
		if d.Provider == chairName || d.Error != nil {
			continue
		}
		if a := e.findAgent(d.Provider); a != nil {
			return a
		}
	}
	return nil
}

// buildSynthesisPrompt constructs the synthesis prompt from dispatches and reviews.
func buildSynthesisPrompt(dispatches []DispatchResult, reviews []ReviewResult) string {
	var parts []string

	parts = append(parts, "Synthesize these agent responses and reviews into a unified answer:")
	parts = append(parts, "")

	for _, d := range dispatches {
		if d.Error != nil {
			continue
		}
		content := TruncateString(d.Content, maxReviewContentBytes)
		parts = append(parts, fmt.Sprintf("--- %s ---\n%s", d.Provider, content))
	}

	if len(reviews) > 0 {
		parts = append(parts, "")
		parts = append(parts, "--- Reviews ---")
		for _, r := range reviews {
			if r.Error != nil {
				continue
			}
			content := TruncateString(r.Content, maxReviewContentBytes)
			parts = append(parts, fmt.Sprintf("\n[%s reviewing %s]\n%s",
				r.Reviewer, strings.Join(r.Reviewed, ", "), content))
		}
	}

	return strings.Join(parts, "\n")
}
