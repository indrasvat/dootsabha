package plugin

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// HookGRPCPlugin is the go-plugin GRPCPlugin implementation for Hook.
type HookGRPCPlugin struct {
	goplugin.Plugin
	// Impl is set on the plugin side (server).
	Impl HookPlugin
}

// GRPCServer registers the hook service on the gRPC server (plugin side).
func (p *HookGRPCPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	gen.RegisterHookServer(s, &hookGRPCServer{Impl: p.Impl})
	return nil
}

// GRPCClient returns a HookPlugin backed by the gRPC client (host side).
func (p *HookGRPCPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &hookGRPCClient{client: gen.NewHookClient(c)}, nil
}

// hookGRPCServer wraps a HookPlugin impl and serves it via gRPC.
type hookGRPCServer struct {
	gen.UnimplementedHookServer
	Impl HookPlugin
}

func (s *hookGRPCServer) PreInvoke(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	return s.Impl.PreInvoke(ctx, req)
}

func (s *hookGRPCServer) PostInvoke(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	return s.Impl.PostInvoke(ctx, req)
}

func (s *hookGRPCServer) PreSynthesis(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	return s.Impl.PreSynthesis(ctx, req)
}

func (s *hookGRPCServer) PostSession(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	return s.Impl.PostSession(ctx, req)
}

// hookGRPCClient wraps a gen.HookClient and implements HookPlugin.
type hookGRPCClient struct {
	client gen.HookClient
}

func (c *hookGRPCClient) PreInvoke(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	return c.client.PreInvoke(ctx, req)
}

func (c *hookGRPCClient) PostInvoke(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	return c.client.PostInvoke(ctx, req)
}

func (c *hookGRPCClient) PreSynthesis(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	return c.client.PreSynthesis(ctx, req)
}

func (c *hookGRPCClient) PostSession(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	return c.client.PostSession(ctx, req)
}
