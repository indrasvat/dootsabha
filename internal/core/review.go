package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const maxReviewContentBytes = 32 * 1024 // 32KB truncation per agent output

// ReviewResult holds one agent's peer review output.
type ReviewResult struct {
	Reviewer  string
	Reviewed  []string // names of agents whose output was reviewed
	Content   string
	Duration  time.Duration
	CostUSD   float64
	TokensIn  int
	TokensOut int
	Error     error
}

// PeerReview runs cross-reviews: each agent reviews all OTHER agents' outputs.
// Skips if fewer than 2 successful dispatch results.
func (e *Engine) PeerReview(ctx context.Context, dispatches []DispatchResult, opts InvokeOptions) ([]ReviewResult, error) {
	// Filter to successful dispatches only.
	var good []DispatchResult
	for _, d := range dispatches {
		if d.Error == nil {
			good = append(good, d)
		}
	}
	if len(good) < 2 {
		slog.Info("peer review skipped", "reason", "fewer than 2 successful dispatches", "successful", len(good))
		return nil, nil // skip review with fewer than 2 agents
	}
	slog.Info("peer review starting", "reviewers", len(good))

	results := make([]ReviewResult, len(good))

	if !e.cfg.Council.Parallel {
		for i, reviewer := range good {
			results[i] = e.reviewOne(ctx, reviewer, good, opts)
		}
		return results, nil
	}

	g, gctx := errgroup.WithContext(ctx)
	for i, reviewer := range good {
		g.Go(func() error {
			results[i] = e.reviewOne(gctx, reviewer, good, opts)
			return nil
		})
	}
	_ = g.Wait()

	return results, nil
}

// reviewOne has one agent review all other agents' outputs.
func (e *Engine) reviewOne(ctx context.Context, reviewer DispatchResult, all []DispatchResult, opts InvokeOptions) ReviewResult {
	// Find the matching agent for this reviewer.
	var agent Agent
	for _, a := range e.agents {
		if a.Name() == reviewer.Provider {
			agent = a
			break
		}
	}
	if agent == nil {
		return ReviewResult{
			Reviewer: reviewer.Provider,
			Error:    fmt.Errorf("review: agent %q not found", reviewer.Provider),
		}
	}

	e.notify(reviewer.Provider, ProgressStarted)
	slog.Debug("review starting", "reviewer", reviewer.Provider)

	// Build review prompt with all other agents' outputs.
	var reviewed []string
	var sections []string
	for _, d := range all {
		if d.Provider == reviewer.Provider {
			continue
		}
		reviewed = append(reviewed, d.Provider)
		content := TruncateString(d.Content, maxReviewContentBytes)
		sections = append(sections, fmt.Sprintf("--- %s ---\n%s", d.Provider, content))
	}

	prompt := fmt.Sprintf(
		"Review the following outputs from %s. Identify strengths, weaknesses, errors. Be specific.\n\n%s",
		strings.Join(reviewed, ", "),
		strings.Join(sections, "\n\n"),
	)

	result, err := agent.Invoke(ctx, prompt, opts)
	if err != nil {
		slog.Warn("review failed", "reviewer", reviewer.Provider, "reviewed", reviewed, "error", err)
		e.notify(reviewer.Provider, ProgressFailed)
		return ReviewResult{
			Reviewer: reviewer.Provider,
			Reviewed: reviewed,
			Error:    fmt.Errorf("review by %s: %w", reviewer.Provider, err),
		}
	}

	slog.Info("review complete", "reviewer", reviewer.Provider, "reviewed", reviewed,
		"duration", result.Duration, "content_len", len(result.Content))
	e.notify(reviewer.Provider, ProgressDone)
	return ReviewResult{
		Reviewer:  reviewer.Provider,
		Reviewed:  reviewed,
		Content:   result.Content,
		Duration:  result.Duration,
		CostUSD:   result.CostUSD,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	}
}

// TruncateString truncates s to maxBytes, appending "... [truncated]" if needed.
func TruncateString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n... [truncated]"
}
