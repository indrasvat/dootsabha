// Plugin binary: implements the Greeter interface and serves it via gRPC.
// Run as a subprocess managed by go-plugin in the host process.
package main

import (
	"fmt"

	"github.com/hashicorp/go-plugin"

	"dootsabha-spike/go-plugin-grpc/shared"
)

// GreeterImpl is the concrete implementation of the Greeter interface.
type GreeterImpl struct{}

func (g *GreeterImpl) Greet(name string) (string, error) {
	return fmt.Sprintf("Namaste, %s! — from plugin process", name), nil
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shared.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"greeter": &shared.GreeterPlugin{Impl: &GreeterImpl{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
