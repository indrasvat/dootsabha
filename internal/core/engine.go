package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

// MaxAgents is the hard cap on concurrent agents in a council.
const MaxAgents = 5

// ProgressEvent describes a stage transition for a single provider.
type ProgressEvent int

const (
	ProgressStarted ProgressEvent = iota
	ProgressDone
	ProgressFailed
)

// ProgressFunc receives per-provider progress updates during dispatch/review.
type ProgressFunc func(provider string, event ProgressEvent)

// Agent abstracts a provider for engine use, avoiding an import cycle with
// the providers package. The CLI layer adapts providers.Provider → core.Agent.
type Agent interface {
	Name() string
	Invoke(ctx context.Context, prompt string, opts InvokeOptions) (*InvokeResult, error)
}

// InvokeOptions configures a single agent invocation (mirrors providers.InvokeOptions).
type InvokeOptions struct {
	Model    string
	MaxTurns int
	Timeout  time.Duration
}

// InvokeResult holds the output of a successful agent invocation (mirrors providers.ProviderResult).
type InvokeResult struct {
	Content   string
	Model     string
	Duration  time.Duration
	CostUSD   float64
	TokensIn  int
	TokensOut int
}

// DispatchResult holds one agent's dispatch output.
type DispatchResult struct {
	Provider  string
	Model     string
	Content   string
	Duration  time.Duration
	CostUSD   float64
	TokensIn  int
	TokensOut int
	Error     error // non-nil if this agent failed
}

// Engine orchestrates the council pipeline: dispatch → peer review → synthesis.
type Engine struct {
	agents   []Agent
	cfg      *Config
	progress ProgressFunc
}

// NewEngine creates a council engine with the given agents and config.
func NewEngine(agents []Agent, cfg *Config) *Engine {
	return &Engine{
		agents: agents,
		cfg:    cfg,
	}
}

// SetProgress registers a callback for dispatch/review progress events.
func (e *Engine) SetProgress(fn ProgressFunc) {
	e.progress = fn
}

func (e *Engine) notify(provider string, event ProgressEvent) {
	if e.progress != nil {
		e.progress(provider, event)
	}
}

// Dispatch sends the prompt to all agents and collects results.
// Parallel mode uses errgroup; sequential mode uses a simple loop.
// Returns results for all agents (failed ones have Error set).
func (e *Engine) Dispatch(ctx context.Context, prompt string, opts InvokeOptions) ([]DispatchResult, error) {
	n := len(e.agents)
	if n == 0 {
		return nil, fmt.Errorf("dispatch: no providers configured")
	}
	if n > MaxAgents {
		return nil, fmt.Errorf("dispatch: %d agents exceeds maximum of %d", n, MaxAgents)
	}

	results := make([]DispatchResult, n)

	mode := "parallel"
	if !e.cfg.Council.Parallel {
		mode = "sequential"
	}
	slog.Info("dispatch starting", "agents", n, "mode", mode, "prompt_len", len(prompt))

	if !e.cfg.Council.Parallel {
		for i, agent := range e.agents {
			results[i] = e.invokeOne(ctx, agent, prompt, opts)
		}
		slog.Info("dispatch complete", "agents", n, "mode", mode)
		return results, nil
	}

	// Parallel dispatch with errgroup.
	g, gctx := errgroup.WithContext(ctx)
	for i, agent := range e.agents {
		g.Go(func() error {
			results[i] = e.invokeOne(gctx, agent, prompt, opts)
			return nil // never fail the group; errors are captured per-result
		})
	}
	_ = g.Wait() // always nil since goroutines never return errors

	slog.Info("dispatch complete", "agents", n, "mode", mode)
	return results, nil
}

// invokeOne calls a single agent and wraps the result.
func (e *Engine) invokeOne(ctx context.Context, agent Agent, prompt string, opts InvokeOptions) DispatchResult {
	name := agent.Name()
	e.notify(name, ProgressStarted)
	slog.Debug("invoking agent", "provider", name, "model", opts.Model, "timeout", opts.Timeout)

	result, err := agent.Invoke(ctx, prompt, opts)
	if err != nil {
		slog.Warn("agent invocation failed", "provider", name, "error", err)
		e.notify(name, ProgressFailed)
		return DispatchResult{
			Provider: name,
			Error:    fmt.Errorf("invoke %s: %w", name, err),
		}
	}

	slog.Info("agent invocation succeeded", "provider", name, "model", result.Model,
		"duration", result.Duration, "tokens_in", result.TokensIn, "tokens_out", result.TokensOut,
		"cost_usd", result.CostUSD, "content_len", len(result.Content))
	e.notify(name, ProgressDone)
	return DispatchResult{
		Provider:  name,
		Model:     result.Model,
		Content:   result.Content,
		Duration:  result.Duration,
		CostUSD:   result.CostUSD,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	}
}
