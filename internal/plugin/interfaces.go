package plugin

import (
	"context"

	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// ProviderPlugin is the Go interface that provider plugins implement.
// It maps 1:1 to the Provider gRPC service in proto/provider.proto.
type ProviderPlugin interface {
	Invoke(ctx context.Context, req *gen.InvokeRequest) (*gen.InvokeResponse, error)
	Cancel(ctx context.Context, req *gen.CancelRequest) (*gen.CancelResponse, error)
	HealthCheck(ctx context.Context) (*gen.HealthCheckResponse, error)
	Capabilities(ctx context.Context) (*gen.CapabilitiesResponse, error)
}

// StrategyPlugin is the Go interface that strategy plugins implement.
// It maps 1:1 to the Strategy gRPC service in proto/strategy.proto.
type StrategyPlugin interface {
	Execute(ctx context.Context, req *gen.ExecuteRequest) (*gen.ExecuteResponse, error)
}

// HookPlugin is the Go interface that hook plugins implement.
// It maps 1:1 to the Hook gRPC service in proto/hook.proto.
type HookPlugin interface {
	PreInvoke(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error)
	PostInvoke(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error)
	PreSynthesis(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error)
	PostSession(ctx context.Context, req *gen.HookRequest) (*gen.HookResponse, error)
}
