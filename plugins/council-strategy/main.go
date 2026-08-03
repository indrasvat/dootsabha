// Council strategy plugin binary.
// Implements the Strategy gRPC service using the existing 3-stage engine
// (dispatch → peer review → synthesis). Serves as the default strategy
// and demonstrates the Strategy plugin interface.
package main

import (
	"context"
	"fmt"
	"time"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/indrasvat/dootsabha/internal/core"
	internalPlugin "github.com/indrasvat/dootsabha/internal/plugin"
	"github.com/indrasvat/dootsabha/internal/providers"
	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// councilStrategy implements plugin.StrategyPlugin using the core engine.
type councilStrategy struct{}

func (s *councilStrategy) Execute(ctx context.Context, req *gen.ExecuteRequest) (*gen.ExecuteResponse, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if len(req.Agents) == 0 {
		return nil, fmt.Errorf("at least one agent is required")
	}

	// Build config from request.
	cfg, err := core.LoadConfig("")
	if err != nil {
		cfg = &core.Config{
			Providers: map[string]core.ProviderConfig{},
		}
	}
	if req.Config != nil {
		cfg.Council.Parallel = req.Config.Parallel
		cfg.Council.Rounds = int(req.Config.Rounds)
		if req.Config.Chair != "" {
			cfg.Council.Chair = req.Config.Chair
		}
	}
	if cfg.Council.Chair == "" && len(req.Agents) > 0 {
		cfg.Council.Chair = req.Agents[0].Name
	}

	// Create agents from config.
	runner := &core.SubprocessRunner{}
	agents := make([]core.Agent, 0, len(req.Agents))
	for _, ac := range req.Agents {
		provider := createProvider(ac.Name, cfg, runner)
		if provider == nil {
			continue
		}
		// AgentConfig.timeout_ms is documented as the PER-AGENT timeout and was
		// being dropped entirely, so every call in a plugin-run council was
		// unbounded — GitHub issue #20 in the out-of-process pipeline.
		timeout := time.Duration(ac.TimeoutMs) * time.Millisecond
		if timeout <= 0 {
			timeout = cfg.Timeout // proto: "0 = use config default"
		}
		agents = append(agents, &agentAdapter{provider: provider, timeout: timeout})
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("no valid agents configured")
	}

	// Create engine and run pipeline.
	engine := core.NewEngine(agents, cfg)
	// Each agent carries its own timeout (see agentAdapter), so the engine adds
	// no window of its own here.
	opts := core.InvokeOptions{}

	start := time.Now()

	// Stage 1: Dispatch.
	dispatches, err := engine.Dispatch(ctx, req.Prompt, opts)
	if err != nil {
		return nil, fmt.Errorf("dispatch: %w", err)
	}

	// Stage 2: Peer Review.
	reviews, err := engine.PeerReview(ctx, dispatches, opts)
	if err != nil {
		return nil, fmt.Errorf("peer review: %w", err)
	}

	// Stage 3: Synthesis.
	synthesis, err := engine.Synthesize(ctx, dispatches, reviews, opts)
	if err != nil {
		return nil, fmt.Errorf("synthesis: %w", err)
	}

	totalDuration := time.Since(start)

	// Convert to proto response.
	return buildExecuteResponse(dispatches, reviews, synthesis, totalDuration), nil
}

// createProvider creates a provider by name.
func createProvider(name string, cfg *core.Config, runner providers.Runner) providers.Provider {
	switch name {
	case "claude":
		return providers.NewClaudeProvider(cfg, runner)
	case "codex":
		return providers.NewCodexProvider(cfg, runner)
	case "agy":
		return providers.NewAgyProvider(cfg, runner)
	case "grok":
		return providers.NewGrokProvider(cfg, runner)
	default:
		return nil
	}
}

// agentAdapter wraps providers.Provider to satisfy core.Agent, applying that
// agent's own per-invocation timeout.
type agentAdapter struct {
	provider providers.Provider
	timeout  time.Duration
}

func (a *agentAdapter) Name() string { return a.provider.Name() }

func (a *agentAdapter) Invoke(ctx context.Context, prompt string, opts core.InvokeOptions) (*core.InvokeResult, error) {
	// A fresh window for THIS call, clipped by whatever the caller's context
	// already allows. Without it every call in the pipeline shared one deadline
	// and a slow agent starved the ones after it (issue #20).
	if a.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = core.StepContext(ctx, a.timeout)
		defer cancel()
	}
	result, err := a.provider.Invoke(ctx, prompt, providers.InvokeOptions{
		Model:    opts.Model,
		MaxTurns: opts.MaxTurns,
		Timeout:  a.timeout,
	})
	if err != nil {
		return nil, err
	}
	return &core.InvokeResult{
		Content:   result.Content,
		Model:     result.Model,
		Duration:  result.Duration,
		CostUSD:   result.CostUSD,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	}, nil
}

// buildExecuteResponse converts engine results to a proto ExecuteResponse.
// buildExecuteResponse assembles the strategy response.
//
// Serialisation goes through internal/plugin's converters rather than being
// written out again here. This function used to hand-build every message, and
// drifted: it never set SynthesisResult.ChairError, so a chair that timed out
// and was replaced by a healthy fallback crossed the wire looking like a clean
// run, and the host exited 0 on a run that hit a deadline. Two serialisers for
// one format is one too many.
func buildExecuteResponse(dispatches []core.DispatchResult, reviews []core.ReviewResult, synthesis *core.SynthesisResult, totalDuration time.Duration) *gen.ExecuteResponse {
	resp := &gen.ExecuteResponse{}

	var totalCost float64
	var totalIn, totalOut int32
	status := make(map[string]string)

	for _, d := range dispatches {
		if d.Error != nil {
			status[d.Provider] = "error"
		} else {
			status[d.Provider] = "healthy"
		}
		totalCost += d.CostUSD
		totalIn += int32(d.TokensIn)
		totalOut += int32(d.TokensOut)
		resp.DispatchResults = append(resp.DispatchResults, internalPlugin.DispatchResultToProto(&d))
	}

	for _, r := range reviews {
		totalCost += r.CostUSD
		totalIn += int32(r.TokensIn)
		totalOut += int32(r.TokensOut)
		resp.ReviewResults = append(resp.ReviewResults, internalPlugin.ReviewResultToProto(&r))
	}

	if synthesis != nil {
		resp.Synthesis = internalPlugin.SynthesisResultToProto(synthesis)
		totalCost += synthesis.CostUSD
		totalIn += int32(synthesis.TokensIn)
		totalOut += int32(synthesis.TokensOut)
	}

	resp.Metadata = &gen.SessionMeta{
		TotalCostUsd:    totalCost,
		TotalTokensIn:   totalIn,
		TotalTokensOut:  totalOut,
		TotalDurationMs: totalDuration.Milliseconds(),
		ProvidersStatus: status,
	}

	return resp
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: internalPlugin.StrategyHandshake,
		Plugins: map[string]goplugin.Plugin{
			"strategy": &internalPlugin.StrategyGRPCPlugin{Impl: &councilStrategy{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
