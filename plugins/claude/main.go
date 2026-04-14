// Claude provider plugin binary.
// Wraps the existing ClaudeProvider and serves it via gRPC.
package main

import (
	"context"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/plugin"
	"github.com/indrasvat/dootsabha/internal/providers"
	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// claudePluginServer wraps ClaudeProvider and implements plugin.ProviderPlugin.
type claudePluginServer struct {
	provider *providers.ClaudeProvider
}

func newClaudePluginServer() *claudePluginServer {
	cfg, err := core.LoadConfig("")
	if err != nil {
		// If config fails, use a minimal config with defaults.
		// The provider's providerConfig() method has its own defaults.
		cfg = &core.Config{
			Providers: map[string]core.ProviderConfig{},
		}
	}
	return &claudePluginServer{
		provider: providers.NewClaudeProvider(cfg, &core.SubprocessRunner{}),
	}
}

func (s *claudePluginServer) Invoke(ctx context.Context, req *gen.InvokeRequest) (*gen.InvokeResponse, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	opts := providers.InvokeOptions{
		Model: req.Model,
	}
	if req.MaxTurns > 0 {
		opts.MaxTurns = int(req.MaxTurns)
	}

	result, err := s.provider.Invoke(ctx, req.Prompt, opts)
	if err != nil {
		return nil, err
	}

	return &gen.InvokeResponse{
		Content:    result.Content,
		Provider:   s.provider.Name(),
		Model:      result.Model,
		SessionId:  result.SessionID,
		CostUsd:    result.CostUSD,
		TokensIn:   int32(result.TokensIn),
		TokensOut:  int32(result.TokensOut),
		DurationMs: result.Duration.Milliseconds(),
	}, nil
}

func (s *claudePluginServer) Cancel(_ context.Context, _ *gen.CancelRequest) (*gen.CancelResponse, error) {
	return &gen.CancelResponse{Cancelled: false}, nil
}

func (s *claudePluginServer) HealthCheck(ctx context.Context) (*gen.HealthCheckResponse, error) {
	status, err := s.provider.HealthCheck(ctx)
	if err != nil {
		return nil, err
	}
	return &gen.HealthCheckResponse{
		Healthy:    status.Healthy,
		CliVersion: status.CLIVersion,
		Model:      status.Model,
		AuthValid:  status.AuthValid,
		Error:      status.Error,
	}, nil
}

func (s *claudePluginServer) Capabilities(_ context.Context) (*gen.CapabilitiesResponse, error) {
	return &gen.CapabilitiesResponse{
		SupportsJson:      true,
		SupportsStreaming: false,
		SupportedModels:   []string{"claude-sonnet-4-6", "claude-opus-4-6", "claude-haiku-4-5-20251001"},
		DefaultModel:      "claude-opus-4-6",
		MaxContextTokens:  200000,
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.ProviderHandshake,
		Plugins: map[string]goplugin.Plugin{
			"provider": &plugin.ProviderGRPCPlugin{Impl: newClaudePluginServer()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
