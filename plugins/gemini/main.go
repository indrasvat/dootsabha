// Gemini provider plugin binary.
// Wraps the existing GeminiProvider and serves it via gRPC.
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

// geminiPluginServer wraps GeminiProvider and implements plugin.ProviderPlugin.
type geminiPluginServer struct {
	provider *providers.GeminiProvider
}

func newGeminiPluginServer() *geminiPluginServer {
	cfg, err := core.LoadConfig("")
	if err != nil {
		cfg = &core.Config{
			Providers: map[string]core.ProviderConfig{},
		}
	}
	return &geminiPluginServer{
		provider: providers.NewGeminiProvider(cfg, &core.SubprocessRunner{}),
	}
}

func (s *geminiPluginServer) Invoke(ctx context.Context, req *gen.InvokeRequest) (*gen.InvokeResponse, error) {
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

func (s *geminiPluginServer) Cancel(_ context.Context, _ *gen.CancelRequest) (*gen.CancelResponse, error) {
	return &gen.CancelResponse{Cancelled: false}, nil
}

func (s *geminiPluginServer) HealthCheck(ctx context.Context) (*gen.HealthCheckResponse, error) {
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

func (s *geminiPluginServer) Capabilities(_ context.Context) (*gen.CapabilitiesResponse, error) {
	return &gen.CapabilitiesResponse{
		SupportsJson:      true,
		SupportsStreaming: false,
		SupportedModels:   []string{"gemini-3.1-pro-preview", "gemini-3-flash-preview", "gemini-2.5-flash"},
		DefaultModel:      "gemini-3.1-pro-preview",
		MaxContextTokens:  1000000,
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.ProviderHandshake,
		Plugins: map[string]goplugin.Plugin{
			"provider": &plugin.ProviderGRPCPlugin{Impl: newGeminiPluginServer()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
