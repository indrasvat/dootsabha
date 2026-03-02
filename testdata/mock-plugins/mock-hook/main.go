// Mock hook plugin binary for integration testing.
// Implements the Hook gRPC service: prepends "[hook] " to prompts,
// appends "[redacted]" to responses, passes through synthesis, records sessions.
package main

import (
	"context"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"

	gen "github.com/indrasvat/dootsabha/proto/gen"

	"github.com/indrasvat/dootsabha/internal/plugin"
)

type mockHook struct{}

func (m *mockHook) PreInvoke(_ context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	invokeReq := req.GetInvokeRequest()
	if invokeReq == nil {
		return &gen.HookResponse{Proceed: true}, nil
	}
	// Rewrite prompt: prepend "[hook] ".
	modified := *invokeReq
	modified.Prompt = fmt.Sprintf("[hook] %s", invokeReq.Prompt)
	return &gen.HookResponse{
		Proceed: true,
		ModifiedPayload: &gen.HookResponse_ModifiedInvokeRequest{
			ModifiedInvokeRequest: &modified,
		},
	}, nil
}

func (m *mockHook) PostInvoke(_ context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	invokeResp := req.GetInvokeResponse()
	if invokeResp == nil {
		return &gen.HookResponse{Proceed: true}, nil
	}
	// Redact response: append " [redacted]".
	modified := *invokeResp
	modified.Content = invokeResp.Content + " [redacted]"
	return &gen.HookResponse{
		Proceed: true,
		ModifiedPayload: &gen.HookResponse_ModifiedInvokeResponse{
			ModifiedInvokeResponse: &modified,
		},
	}, nil
}

func (m *mockHook) PreSynthesis(_ context.Context, _ *gen.HookRequest) (*gen.HookResponse, error) {
	// Pass through — no modification.
	return &gen.HookResponse{Proceed: true}, nil
}

func (m *mockHook) PostSession(_ context.Context, req *gen.HookRequest) (*gen.HookResponse, error) {
	summary := req.GetSessionSummary()
	if summary == nil {
		return &gen.HookResponse{Proceed: true}, nil
	}
	// Record session — in a real hook this might write to a file or webhook.
	// For testing, we just acknowledge.
	return &gen.HookResponse{
		Proceed: true,
	}, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: plugin.HookHandshake,
		Plugins: map[string]goplugin.Plugin{
			"hook": &plugin.HookGRPCPlugin{Impl: &mockHook{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
