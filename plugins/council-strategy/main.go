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
		agents = append(agents, &agentAdapter{provider: provider})
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("no valid agents configured")
	}

	// Create engine and run pipeline.
	engine := core.NewEngine(agents, cfg)
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
	case "gemini":
		return providers.NewGeminiProvider(cfg, runner)
	default:
		return nil
	}
}

// agentAdapter wraps providers.Provider to satisfy core.Agent.
type agentAdapter struct {
	provider providers.Provider
}

func (a *agentAdapter) Name() string { return a.provider.Name() }

func (a *agentAdapter) Invoke(ctx context.Context, prompt string, opts core.InvokeOptions) (*core.InvokeResult, error) {
	result, err := a.provider.Invoke(ctx, prompt, providers.InvokeOptions{
		Model:    opts.Model,
		MaxTurns: opts.MaxTurns,
		Timeout:  opts.Timeout,
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
func buildExecuteResponse(dispatches []core.DispatchResult, reviews []core.ReviewResult, synthesis *core.SynthesisResult, totalDuration time.Duration) *gen.ExecuteResponse {
	resp := &gen.ExecuteResponse{}

	var totalCost float64
	var totalIn, totalOut int32
	status := make(map[string]string)

	for _, d := range dispatches {
		dr := &gen.DispatchResult{
			Provider:   d.Provider,
			Model:      d.Model,
			Content:    d.Content,
			DurationMs: d.Duration.Milliseconds(),
			CostUsd:    d.CostUSD,
			TokensIn:   int32(d.TokensIn),
			TokensOut:  int32(d.TokensOut),
		}
		if d.Error != nil {
			dr.Error = d.Error.Error()
			status[d.Provider] = "error"
		} else {
			status[d.Provider] = "healthy"
		}
		totalCost += d.CostUSD
		totalIn += int32(d.TokensIn)
		totalOut += int32(d.TokensOut)
		resp.DispatchResults = append(resp.DispatchResults, dr)
	}

	for _, r := range reviews {
		rr := &gen.ReviewResult{
			Reviewer:   r.Reviewer,
			Reviewed:   r.Reviewed,
			Content:    r.Content,
			DurationMs: r.Duration.Milliseconds(),
			CostUsd:    r.CostUSD,
			TokensIn:   int32(r.TokensIn),
			TokensOut:  int32(r.TokensOut),
		}
		if r.Error != nil {
			rr.Error = r.Error.Error()
		}
		totalCost += r.CostUSD
		totalIn += int32(r.TokensIn)
		totalOut += int32(r.TokensOut)
		resp.ReviewResults = append(resp.ReviewResults, rr)
	}

	if synthesis != nil {
		resp.Synthesis = &gen.SynthesisResult{
			Chair:         synthesis.Chair,
			ChairFallback: synthesis.ChairFallback,
			Content:       synthesis.Content,
			DurationMs:    synthesis.Duration.Milliseconds(),
			CostUsd:       synthesis.CostUSD,
			TokensIn:      int32(synthesis.TokensIn),
			TokensOut:     int32(synthesis.TokensOut),
		}
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
