// Antigravity (agy) provider plugin binary.
// Wraps the existing AgyProvider and serves it via gRPC.
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

// agyPluginServer wraps AgyProvider and implements plugin.ProviderPlugin.
type agyPluginServer struct {
	provider *providers.AgyProvider
}

func newAgyPluginServer() *agyPluginServer {
	cfg, err := core.LoadConfig("")
	if err != nil {
		cfg = &core.Config{
			Providers: map[string]core.ProviderConfig{},
		}
	}
	return &agyPluginServer{
		provider: providers.NewAgyProvider(cfg, &core.SubprocessRunner{}),
	}
}

func (s *agyPluginServer) Invoke(ctx context.Context, req *gen.InvokeRequest) (*gen.InvokeResponse, error) {
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

func (s *agyPluginServer) Cancel(_ context.Context, _ *gen.CancelRequest) (*gen.CancelResponse, error) {
	return &gen.CancelResponse{Cancelled: false}, nil
}

func (s *agyPluginServer) HealthCheck(ctx context.Context) (*gen.HealthCheckResponse, error) {
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

func (s *agyPluginServer) Capabilities(_ context.Context) (*gen.CapabilitiesResponse, error) {
	return &gen.CapabilitiesResponse{
		SupportsJson:      false,
		SupportsStreaming: false,
		SupportedModels: []string{
			"Gemini 3.5 Flash (Low)",
			"Gemini 3.5 Flash (Medium)",
			"Gemini 3.5 Flash (High)",
			"Gemini 3.1 Pro (Low)",
			"Gemini 3.1 Pro (High)",
			"Claude Sonnet 4.6 (Thinking)",
			"Claude Opus 4.6 (Thinking)",
			"GPT-OSS 120B (Medium)",
		},
		DefaultModel:     "Gemini 3.5 Flash (High)",
		MaxContextTokens: 1000000,
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.ProviderHandshake,
		Plugins: map[string]goplugin.Plugin{
			"provider": &plugin.ProviderGRPCPlugin{Impl: newAgyPluginServer()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
