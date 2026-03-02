// Mock strategy plugin binary for integration testing.
// Implements the Strategy gRPC service with canned council pipeline responses.
package main

import (
	"context"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"

	gen "github.com/indrasvat/dootsabha/proto/gen"

	"github.com/indrasvat/dootsabha/internal/plugin"
)

type mockStrategy struct{}

func (m *mockStrategy) Execute(_ context.Context, req *gen.ExecuteRequest) (*gen.ExecuteResponse, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// Build mock dispatch results — one per agent.
	dispatches := make([]*gen.DispatchResult, len(req.Agents))
	for i, agent := range req.Agents {
		dispatches[i] = &gen.DispatchResult{
			Provider:   agent.Name,
			Model:      agent.Model,
			Content:    fmt.Sprintf("[%s] Response to: %s", agent.Name, req.Prompt),
			DurationMs: 100 + int64(i)*50,
			CostUsd:    0.001 * float64(i+1),
			TokensIn:   50,
			TokensOut:  100,
		}
	}

	// Build mock review results — each agent reviews the others.
	var reviews []*gen.ReviewResult
	for _, agent := range req.Agents {
		reviewed := make([]string, 0, len(req.Agents)-1)
		for _, other := range req.Agents {
			if other.Name != agent.Name {
				reviewed = append(reviewed, other.Name)
			}
		}
		reviews = append(reviews, &gen.ReviewResult{
			Reviewer:   agent.Name,
			Reviewed:   reviewed,
			Content:    fmt.Sprintf("[%s review] All responses are adequate.", agent.Name),
			DurationMs: 80,
			CostUsd:    0.0005,
			TokensIn:   200,
			TokensOut:  50,
		})
	}

	// Chair synthesis.
	chair := "claude"
	if req.Config != nil && req.Config.Chair != "" {
		chair = req.Config.Chair
	}
	synthesis := &gen.SynthesisResult{
		Chair:      chair,
		Content:    fmt.Sprintf("[%s synthesis] Combined analysis of: %s", chair, req.Prompt),
		DurationMs: 200,
		CostUsd:    0.002,
		TokensIn:   500,
		TokensOut:  200,
	}

	// Session metadata.
	var totalCost float64
	var totalIn, totalOut int32
	var totalDur int64
	status := make(map[string]string)
	for _, d := range dispatches {
		totalCost += d.CostUsd
		totalIn += d.TokensIn
		totalOut += d.TokensOut
		totalDur += d.DurationMs
		status[d.Provider] = "healthy"
	}

	return &gen.ExecuteResponse{
		DispatchResults: dispatches,
		ReviewResults:   reviews,
		Synthesis:       synthesis,
		Metadata: &gen.SessionMeta{
			TotalCostUsd:    totalCost + synthesis.CostUsd,
			TotalTokensIn:   totalIn + synthesis.TokensIn,
			TotalTokensOut:  totalOut + synthesis.TokensOut,
			TotalDurationMs: totalDur + synthesis.DurationMs,
			ProvidersStatus: status,
		},
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.StrategyHandshake,
		Plugins: map[string]goplugin.Plugin{
			"strategy": &plugin.StrategyGRPCPlugin{Impl: &mockStrategy{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
