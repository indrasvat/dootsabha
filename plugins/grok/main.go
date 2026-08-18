// Grok provider plugin binary.
// Wraps the existing GrokProvider and serves it via gRPC.
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

// grokPluginServer wraps GrokProvider and implements plugin.ProviderPlugin.
type grokPluginServer struct {
	provider *providers.GrokProvider
}

func newGrokPluginServer() *grokPluginServer {
	cfg, err := core.LoadConfig("")
	if err != nil {
		cfg = &core.Config{
			Providers: map[string]core.ProviderConfig{},
		}
	}
	return &grokPluginServer{
		provider: providers.NewGrokProvider(cfg, &core.SubprocessRunner{}),
	}
}

func (s *grokPluginServer) Invoke(ctx context.Context, req *gen.InvokeRequest) (*gen.InvokeResponse, error) {
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

func (s *grokPluginServer) Cancel(_ context.Context, _ *gen.CancelRequest) (*gen.CancelResponse, error) {
	return &gen.CancelResponse{Cancelled: false}, nil
}

func (s *grokPluginServer) HealthCheck(ctx context.Context) (*gen.HealthCheckResponse, error) {
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

func (s *grokPluginServer) Capabilities(_ context.Context) (*gen.CapabilitiesResponse, error) {
	return &gen.CapabilitiesResponse{
		SupportsJson: true,
		// The grok CLI streams, but this plugin cannot: the gRPC Invoke RPC is
		// unary and GrokProvider.Invoke buffers the whole subprocess output before
		// returning. Advertising streaming would promise consumers a mode that is
		// unreachable through this interface.
		SupportsStreaming: false,
		// grok-4.5 is NOT retired — `grok models` still lists it, so it stays
		// selectable. The default is read from the provider, never repeated.
		SupportedModels:  []string{providers.GrokDefaultModel, "grok-4.5"},
		DefaultModel:     providers.GrokDefaultModel,
		MaxContextTokens: 500000,
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.ProviderHandshake,
		Plugins: map[string]goplugin.Plugin{
			"provider": &plugin.ProviderGRPCPlugin{Impl: newGrokPluginServer()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
