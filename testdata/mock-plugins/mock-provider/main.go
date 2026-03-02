// Mock provider plugin binary for integration testing.
// Implements the Provider gRPC service with canned responses.
package main

import (
	"context"
	"fmt"
	"strings"

	goplugin "github.com/hashicorp/go-plugin"

	gen "github.com/indrasvat/dootsabha/proto/gen"

	"github.com/indrasvat/dootsabha/internal/plugin"
)

type mockProvider struct{}

func (m *mockProvider) Invoke(_ context.Context, req *gen.InvokeRequest) (*gen.InvokeResponse, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	content := fmt.Sprintf("Mock response to: %s", req.Prompt)
	model := req.Model
	if model == "" {
		model = "mock-model-v1"
	}
	return &gen.InvokeResponse{
		Content:    content,
		Provider:   "mock-provider",
		Model:      model,
		SessionId:  "mock-session-001",
		CostUsd:    0.001,
		TokensIn:   int32(len(strings.Fields(req.Prompt))),
		TokensOut:  int32(len(strings.Fields(content))),
		DurationMs: 42,
	}, nil
}

func (m *mockProvider) Cancel(_ context.Context, req *gen.CancelRequest) (*gen.CancelResponse, error) {
	return &gen.CancelResponse{Cancelled: true}, nil
}

func (m *mockProvider) HealthCheck(_ context.Context) (*gen.HealthCheckResponse, error) {
	return &gen.HealthCheckResponse{
		Healthy:    true,
		CliVersion: "mock-1.0.0",
		Model:      "mock-model-v1",
		AuthValid:  true,
	}, nil
}

func (m *mockProvider) Capabilities(_ context.Context) (*gen.CapabilitiesResponse, error) {
	return &gen.CapabilitiesResponse{
		SupportsJson:      true,
		SupportsStreaming: false,
		SupportedModels:   []string{"mock-model-v1", "mock-model-v2"},
		DefaultModel:      "mock-model-v1",
		MaxContextTokens:  128000,
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.ProviderHandshake,
		Plugins: map[string]goplugin.Plugin{
			"provider": &plugin.ProviderGRPCPlugin{Impl: &mockProvider{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
