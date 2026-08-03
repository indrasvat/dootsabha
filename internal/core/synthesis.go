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
	// ChairError is why the chair was replaced, kept even though the fallback
	// succeeded: a chair that timed out is still "an agent timed out", and
	// dropping it here let a council exit 0 on a run that hit a deadline.
	ChairError error
	Content    string
	Duration   time.Duration
	CostUSD    float64
	TokensIn   int
	TokensOut  int
}

// Synthesize invokes the chair agent with all dispatch outputs and reviews.
// On chair failure, re-invokes the first healthy non-chair agent.
func (e *Engine) Synthesize(ctx context.Context, dispatches []DispatchResult, reviews []ReviewResult, opts InvokeOptions) (*SynthesisResult, error) {
	prompt := buildSynthesisPrompt(dispatches, reviews)
	var chairErr error

	// Find the chair agent.
	chairName := e.cfg.Council.Chair
	slog.Info("synthesis starting", "chair", chairName, "dispatches", len(dispatches), "reviews", len(reviews))
	chair := e.findAgent(chairName)

	if chair != nil {
		e.notify(chairName, ProgressStarted)
		slog.Debug("invoking chair", "chair", chairName)
		// Synthesis is the LAST stage, so it was the one starved by a shared
		// deadline (issue #20). It gets its own window like every other call.
		//
		// The closure scopes `defer cancel` to the call itself: the fallback
		// below must not inherit the chair's spent window, and a provider that
		// panics must not leak the timer.
		result, err := func() (*InvokeResult, error) {
			chairCtx, cancel := StepContext(ctx, opts.Timeout)
			defer cancel()
			return chair.Invoke(chairCtx, prompt, opts)
		}()
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
		chairErr = err
		// Chair failed — try fallback.
	}

	// Fallback: first healthy non-chair agent.
	fallback := e.findFallbackAgent(chairName, dispatches)
	if fallback == nil {
		// Carry the chair's failure. It is the reason there is nothing to fall
		// back to, and when it was a deadline the caller's exit code depends on
		// seeing it — 4 ("an agent timed out") outranks 5.
		if chairErr != nil {
			return nil, fmt.Errorf("synthesize: no healthy agent left after chair %s failed: %w", chairName, chairErr)
		}
		return nil, fmt.Errorf("synthesize: no healthy agents available for synthesis")
	}

	fallbackName := fallback.Name()
	slog.Info("synthesis fallback", "fallback", fallbackName)
	e.notify(fallbackName, ProgressStarted)
	// The chair may have burned its whole window before failing; the fallback
	// still gets a full one, or it would be reported broken for being second.
	fallbackCtx, cancelFallback := StepContext(ctx, opts.Timeout)
	defer cancelFallback()
	result, err := fallback.Invoke(fallbackCtx, prompt, opts)
	if err != nil {
		e.notify(fallbackName, ProgressFailed)
		// Same reasoning: a chair that timed out before a fallback that failed
		// some other way is still a run that timed out.
		if chairErr != nil {
			return nil, fmt.Errorf("synthesize fallback %s: %w (after chair %s: %w)", fallbackName, err, chairName, chairErr)
		}
		return nil, fmt.Errorf("synthesize fallback %s: %w", fallbackName, err)
	}

	e.notify(fallbackName, ProgressDone)
	return &SynthesisResult{
		Chair:         chairName,
		ChairFallback: fallbackName,
		ChairError:    chairErr,
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
