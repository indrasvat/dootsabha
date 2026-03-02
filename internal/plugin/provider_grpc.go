package plugin

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// ProviderGRPCPlugin is the go-plugin GRPCPlugin implementation for Provider.
type ProviderGRPCPlugin struct {
	goplugin.Plugin
	// Impl is set on the plugin side (server).
	Impl ProviderPlugin
}

// GRPCServer registers the provider service on the gRPC server (plugin side).
func (p *ProviderGRPCPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	gen.RegisterProviderServer(s, &providerGRPCServer{Impl: p.Impl})
	return nil
}

// GRPCClient returns a ProviderPlugin backed by the gRPC client (host side).
func (p *ProviderGRPCPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &providerGRPCClient{client: gen.NewProviderClient(c)}, nil
}

// providerGRPCServer wraps a ProviderPlugin impl and serves it via gRPC.
type providerGRPCServer struct {
	gen.UnimplementedProviderServer
	Impl ProviderPlugin
}

func (s *providerGRPCServer) Invoke(ctx context.Context, req *gen.InvokeRequest) (*gen.InvokeResponse, error) {
	return s.Impl.Invoke(ctx, req)
}

func (s *providerGRPCServer) Cancel(ctx context.Context, req *gen.CancelRequest) (*gen.CancelResponse, error) {
	return s.Impl.Cancel(ctx, req)
}

func (s *providerGRPCServer) HealthCheck(ctx context.Context, _ *gen.HealthCheckRequest) (*gen.HealthCheckResponse, error) {
	return s.Impl.HealthCheck(ctx)
}

func (s *providerGRPCServer) Capabilities(ctx context.Context, _ *gen.CapabilitiesRequest) (*gen.CapabilitiesResponse, error) {
	return s.Impl.Capabilities(ctx)
}

// providerGRPCClient wraps a gen.ProviderClient and implements ProviderPlugin.
type providerGRPCClient struct {
	client gen.ProviderClient
}

func (c *providerGRPCClient) Invoke(ctx context.Context, req *gen.InvokeRequest) (*gen.InvokeResponse, error) {
	return c.client.Invoke(ctx, req)
}

func (c *providerGRPCClient) Cancel(ctx context.Context, req *gen.CancelRequest) (*gen.CancelResponse, error) {
	return c.client.Cancel(ctx, req)
}

func (c *providerGRPCClient) HealthCheck(ctx context.Context) (*gen.HealthCheckResponse, error) {
	return c.client.HealthCheck(ctx, &gen.HealthCheckRequest{})
}

func (c *providerGRPCClient) Capabilities(ctx context.Context) (*gen.CapabilitiesResponse, error) {
	return c.client.Capabilities(ctx, &gen.CapabilitiesRequest{})
}
