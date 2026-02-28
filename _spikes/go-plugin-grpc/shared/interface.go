// Package shared contains the Greeter interface and go-plugin gRPC wrappers
// used by both host and plugin processes.
package shared

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

// HandshakeConfig is shared between host and plugin — must match exactly.
var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "GREETER_PLUGIN",
	MagicCookieValue: "dootsabha-spike",
}

// PluginMap is the map of plugins we can dispense.
var PluginMap = map[string]plugin.Plugin{
	"greeter": &GreeterPlugin{},
}

// Greeter is the interface implemented by plugins.
type Greeter interface {
	Greet(name string) (string, error)
}

// GreeterPlugin is the go-plugin GRPCPlugin implementation.
type GreeterPlugin struct {
	plugin.Plugin
	// Impl is set on the plugin side (not the host side).
	Impl Greeter
}

func (p *GreeterPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	RegisterGreeterServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *GreeterPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: NewGreeterClient(c)}, nil
}

// GRPCServer is the server-side implementation of Greeter over gRPC.
// It wraps the concrete Impl and translates between the Go interface and gRPC.
type GRPCServer struct {
	UnimplementedGreeterServer
	Impl Greeter
}

func (s *GRPCServer) Greet(ctx context.Context, req *GreetRequest) (*GreetResponse, error) {
	msg, err := s.Impl.Greet(req.Name)
	if err != nil {
		return nil, err
	}
	return &GreetResponse{Message: msg}, nil
}

// GRPCClient is the client-side implementation of Greeter over gRPC.
// It wraps the generated GreeterClient and translates to the Go interface.
type GRPCClient struct {
	client GreeterClient
}

func (c *GRPCClient) Greet(name string) (string, error) {
	resp, err := c.client.Greet(context.Background(), &GreetRequest{Name: name})
	if err != nil {
		return "", err
	}
	return resp.Message, nil
}
