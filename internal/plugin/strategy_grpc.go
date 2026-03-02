package plugin

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// StrategyGRPCPlugin is the go-plugin GRPCPlugin implementation for Strategy.
type StrategyGRPCPlugin struct {
	goplugin.Plugin
	// Impl is set on the plugin side (server).
	Impl StrategyPlugin
}

// GRPCServer registers the strategy service on the gRPC server (plugin side).
func (p *StrategyGRPCPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	gen.RegisterStrategyServer(s, &strategyGRPCServer{Impl: p.Impl})
	return nil
}

// GRPCClient returns a StrategyPlugin backed by the gRPC client (host side).
func (p *StrategyGRPCPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &strategyGRPCClient{client: gen.NewStrategyClient(c)}, nil
}

// strategyGRPCServer wraps a StrategyPlugin impl and serves it via gRPC.
type strategyGRPCServer struct {
	gen.UnimplementedStrategyServer
	Impl StrategyPlugin
}

func (s *strategyGRPCServer) Execute(ctx context.Context, req *gen.ExecuteRequest) (*gen.ExecuteResponse, error) {
	return s.Impl.Execute(ctx, req)
}

// strategyGRPCClient wraps a gen.StrategyClient and implements StrategyPlugin.
type strategyGRPCClient struct {
	client gen.StrategyClient
}

func (c *strategyGRPCClient) Execute(ctx context.Context, req *gen.ExecuteRequest) (*gen.ExecuteResponse, error) {
	return c.client.Execute(ctx, req)
}
