package plugin_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/indrasvat/dootsabha/internal/plugin"
	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// testLogger returns a silent logger to suppress go-plugin's internal noise.
func testLogger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:   "test",
		Output: os.Stderr,
		Level:  hclog.Error,
	})
}

// mockPluginBinDir resolves the directory containing pre-built mock plugin binaries.
// Tests must be run from the repo root or use -count=1 to avoid caching issues.
func mockPluginBinDir(t *testing.T) string {
	t.Helper()
	// Walk up from this test file's location to find the repo root.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	binDir := filepath.Join(repoRoot, "testdata", "mock-plugins", "bin")
	if _, err := os.Stat(binDir); err != nil {
		t.Fatalf("mock plugin binaries not found at %s — run: make build-mock-plugins", binDir)
	}
	return binDir
}

// newProviderClient launches the mock-provider plugin and returns a client.
func newProviderClient(t *testing.T) (*goplugin.Client, plugin.ProviderPlugin) {
	t.Helper()
	binDir := mockPluginBinDir(t)
	pluginPath := filepath.Join(binDir, "mock-provider")

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  plugin.ProviderHandshake,
		Plugins:          plugin.ProviderPluginMap,
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           testLogger(),
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		t.Fatalf("provider rpc client: %v", err)
	}

	raw, err := rpcClient.Dispense("provider")
	if err != nil {
		client.Kill()
		t.Fatalf("dispense provider: %v", err)
	}

	provider, ok := raw.(plugin.ProviderPlugin)
	if !ok {
		client.Kill()
		t.Fatalf("unexpected type: %T", raw)
	}
	return client, provider
}

// newStrategyClient launches the mock-strategy plugin and returns a client.
func newStrategyClient(t *testing.T) (*goplugin.Client, plugin.StrategyPlugin) {
	t.Helper()
	binDir := mockPluginBinDir(t)
	pluginPath := filepath.Join(binDir, "mock-strategy")

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  plugin.StrategyHandshake,
		Plugins:          plugin.StrategyPluginMap,
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           testLogger(),
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		t.Fatalf("strategy rpc client: %v", err)
	}

	raw, err := rpcClient.Dispense("strategy")
	if err != nil {
		client.Kill()
		t.Fatalf("dispense strategy: %v", err)
	}

	strategy, ok := raw.(plugin.StrategyPlugin)
	if !ok {
		client.Kill()
		t.Fatalf("unexpected type: %T", raw)
	}
	return client, strategy
}

// newHookClient launches the mock-hook plugin and returns a client.
func newHookClient(t *testing.T) (*goplugin.Client, plugin.HookPlugin) {
	t.Helper()
	binDir := mockPluginBinDir(t)
	pluginPath := filepath.Join(binDir, "mock-hook")

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  plugin.HookHandshake,
		Plugins:          plugin.HookPluginMap,
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           testLogger(),
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		t.Fatalf("hook rpc client: %v", err)
	}

	raw, err := rpcClient.Dispense("hook")
	if err != nil {
		client.Kill()
		t.Fatalf("dispense hook: %v", err)
	}

	hook, ok := raw.(plugin.HookPlugin)
	if !ok {
		client.Kill()
		t.Fatalf("unexpected type: %T", raw)
	}
	return client, hook
}

// ── Provider Plugin Tests ───────────────────────────────────────────────────

func TestProviderPluginHandshake(t *testing.T) {
	client, _ := newProviderClient(t)
	defer client.Kill()
	// If we get here, handshake succeeded.
}

func TestProviderPluginInvokeRoundtrip(t *testing.T) {
	client, provider := newProviderClient(t)
	defer client.Kill()

	ctx := context.Background()
	resp, err := provider.Invoke(ctx, &gen.InvokeRequest{
		Prompt: "What is the capital of India?",
		Model:  "test-model",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp.Content != "Mock response to: What is the capital of India?" {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.Provider != "mock-provider" {
		t.Errorf("provider = %q, want mock-provider", resp.Provider)
	}
	if resp.Model != "test-model" {
		t.Errorf("model = %q, want test-model", resp.Model)
	}
	if resp.SessionId != "mock-session-001" {
		t.Errorf("session_id = %q", resp.SessionId)
	}
	if resp.CostUsd != 0.001 {
		t.Errorf("cost_usd = %f, want 0.001", resp.CostUsd)
	}
	if resp.TokensIn == 0 {
		t.Error("tokens_in should be > 0")
	}
	if resp.TokensOut == 0 {
		t.Error("tokens_out should be > 0")
	}
	if resp.DurationMs != 42 {
		t.Errorf("duration_ms = %d, want 42", resp.DurationMs)
	}
}

func TestProviderPluginInvokeDefaultModel(t *testing.T) {
	client, provider := newProviderClient(t)
	defer client.Kill()

	resp, err := provider.Invoke(context.Background(), &gen.InvokeRequest{
		Prompt: "test prompt",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp.Model != "mock-model-v1" {
		t.Errorf("model = %q, want mock-model-v1 (default)", resp.Model)
	}
}

func TestProviderPluginInvokeEmptyPromptError(t *testing.T) {
	client, provider := newProviderClient(t)
	defer client.Kill()

	_, err := provider.Invoke(context.Background(), &gen.InvokeRequest{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestProviderPluginHealthCheckRoundtrip(t *testing.T) {
	client, provider := newProviderClient(t)
	defer client.Kill()

	resp, err := provider.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if !resp.Healthy {
		t.Error("expected healthy = true")
	}
	if resp.CliVersion != "mock-1.0.0" {
		t.Errorf("cli_version = %q", resp.CliVersion)
	}
	if resp.Model != "mock-model-v1" {
		t.Errorf("model = %q", resp.Model)
	}
	if !resp.AuthValid {
		t.Error("expected auth_valid = true")
	}
}

func TestProviderPluginCapabilitiesRoundtrip(t *testing.T) {
	client, provider := newProviderClient(t)
	defer client.Kill()

	resp, err := provider.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if !resp.SupportsJson {
		t.Error("expected supports_json = true")
	}
	if resp.SupportsStreaming {
		t.Error("expected supports_streaming = false")
	}
	if len(resp.SupportedModels) != 2 {
		t.Errorf("supported_models count = %d, want 2", len(resp.SupportedModels))
	}
	if resp.DefaultModel != "mock-model-v1" {
		t.Errorf("default_model = %q", resp.DefaultModel)
	}
	if resp.MaxContextTokens != 128000 {
		t.Errorf("max_context_tokens = %d, want 128000", resp.MaxContextTokens)
	}
}

func TestProviderPluginCancelRoundtrip(t *testing.T) {
	client, provider := newProviderClient(t)
	defer client.Kill()

	resp, err := provider.Cancel(context.Background(), &gen.CancelRequest{
		SessionId: "session-123",
	})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !resp.Cancelled {
		t.Error("expected cancelled = true")
	}
}

func TestProviderPluginContextCancellation(t *testing.T) {
	client, provider := newProviderClient(t)
	defer client.Kill()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := provider.Invoke(ctx, &gen.InvokeRequest{
		Prompt: "this should fail",
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestProviderPluginConcurrentCalls(t *testing.T) {
	client, provider := newProviderClient(t)
	defer client.Kill()

	const n = 5
	errs := make(chan error, n)
	for i := range n {
		go func(idx int) {
			_, err := provider.Invoke(context.Background(), &gen.InvokeRequest{
				Prompt: "concurrent test",
				Model:  "concurrent-model",
			})
			errs <- err
		}(i)
	}
	for range n {
		if err := <-errs; err != nil {
			t.Errorf("concurrent invoke: %v", err)
		}
	}
}

// ── Strategy Plugin Tests ───────────────────────────────────────────────────

func TestStrategyPluginHandshake(t *testing.T) {
	client, _ := newStrategyClient(t)
	defer client.Kill()
}

func TestStrategyPluginExecuteRoundtrip(t *testing.T) {
	client, strategy := newStrategyClient(t)
	defer client.Kill()

	resp, err := strategy.Execute(context.Background(), &gen.ExecuteRequest{
		Prompt: "Compare React vs Vue",
		Agents: []*gen.AgentConfig{
			{Name: "claude", Model: "sonnet-4"},
			{Name: "codex", Model: "o4-mini"},
			{Name: "gemini", Model: "gemini-2.5-pro"},
		},
		Config: &gen.StrategyConfig{
			Parallel:     true,
			Rounds:       1,
			Chair:        "claude",
			StrategyName: "council",
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// 3 dispatch results.
	if len(resp.DispatchResults) != 3 {
		t.Fatalf("dispatch_results count = %d, want 3", len(resp.DispatchResults))
	}
	for i, d := range resp.DispatchResults {
		if d.Content == "" {
			t.Errorf("dispatch[%d] content empty", i)
		}
		if d.Provider == "" {
			t.Errorf("dispatch[%d] provider empty", i)
		}
	}

	// 3 review results (one per agent).
	if len(resp.ReviewResults) != 3 {
		t.Fatalf("review_results count = %d, want 3", len(resp.ReviewResults))
	}
	for i, r := range resp.ReviewResults {
		if r.Reviewer == "" {
			t.Errorf("review[%d] reviewer empty", i)
		}
		if len(r.Reviewed) != 2 {
			t.Errorf("review[%d] reviewed count = %d, want 2", i, len(r.Reviewed))
		}
	}

	// Synthesis.
	if resp.Synthesis == nil {
		t.Fatal("synthesis is nil")
	}
	if resp.Synthesis.Chair != "claude" {
		t.Errorf("synthesis chair = %q, want claude", resp.Synthesis.Chair)
	}
	if resp.Synthesis.Content == "" {
		t.Error("synthesis content empty")
	}

	// Session metadata.
	if resp.Metadata == nil {
		t.Fatal("metadata is nil")
	}
	if resp.Metadata.TotalCostUsd <= 0 {
		t.Errorf("total_cost_usd = %f, want > 0", resp.Metadata.TotalCostUsd)
	}
	if len(resp.Metadata.ProvidersStatus) != 3 {
		t.Errorf("providers_status count = %d, want 3", len(resp.Metadata.ProvidersStatus))
	}
}

func TestStrategyPluginExecuteEmptyPromptError(t *testing.T) {
	client, strategy := newStrategyClient(t)
	defer client.Kill()

	_, err := strategy.Execute(context.Background(), &gen.ExecuteRequest{
		Agents: []*gen.AgentConfig{{Name: "claude"}},
	})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestStrategyPluginExecuteSingleAgent(t *testing.T) {
	client, strategy := newStrategyClient(t)
	defer client.Kill()

	resp, err := strategy.Execute(context.Background(), &gen.ExecuteRequest{
		Prompt: "simple consult",
		Agents: []*gen.AgentConfig{
			{Name: "claude", Model: "sonnet-4"},
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(resp.DispatchResults) != 1 {
		t.Errorf("dispatch count = %d, want 1", len(resp.DispatchResults))
	}
	// Single agent reviews nobody.
	if len(resp.ReviewResults) != 1 {
		t.Errorf("review count = %d, want 1", len(resp.ReviewResults))
	}
	if len(resp.ReviewResults[0].Reviewed) != 0 {
		t.Errorf("reviewed count = %d, want 0 (no peers)", len(resp.ReviewResults[0].Reviewed))
	}
}

// ── Hook Plugin Tests ───────────────────────────────────────────────────────

func TestHookPluginHandshake(t *testing.T) {
	client, _ := newHookClient(t)
	defer client.Kill()
}

func TestHookPluginPreInvokeModifiesRequest(t *testing.T) {
	client, hook := newHookClient(t)
	defer client.Kill()

	resp, err := hook.PreInvoke(context.Background(), &gen.HookRequest{
		EventType: gen.EventType_PRE_INVOKE,
		Payload: &gen.HookRequest_InvokeRequest{
			InvokeRequest: &gen.InvokeRequest{
				Prompt: "original prompt",
				Model:  "sonnet-4",
			},
		},
	})
	if err != nil {
		t.Fatalf("pre_invoke: %v", err)
	}
	if !resp.Proceed {
		t.Error("expected proceed = true")
	}
	modified := resp.GetModifiedInvokeRequest()
	if modified == nil {
		t.Fatal("expected modified_invoke_request")
	}
	if modified.Prompt != "[hook] original prompt" {
		t.Errorf("modified prompt = %q", modified.Prompt)
	}
	if modified.Model != "sonnet-4" {
		t.Errorf("model changed unexpectedly: %q", modified.Model)
	}
}

func TestHookPluginPostInvokeModifiesResponse(t *testing.T) {
	client, hook := newHookClient(t)
	defer client.Kill()

	resp, err := hook.PostInvoke(context.Background(), &gen.HookRequest{
		EventType: gen.EventType_POST_INVOKE,
		Payload: &gen.HookRequest_InvokeResponse{
			InvokeResponse: &gen.InvokeResponse{
				Content:  "sensitive output",
				Provider: "claude",
			},
		},
	})
	if err != nil {
		t.Fatalf("post_invoke: %v", err)
	}
	if !resp.Proceed {
		t.Error("expected proceed = true")
	}
	modified := resp.GetModifiedInvokeResponse()
	if modified == nil {
		t.Fatal("expected modified_invoke_response")
	}
	if modified.Content != "sensitive output [redacted]" {
		t.Errorf("modified content = %q", modified.Content)
	}
}

func TestHookPluginPreSynthesisPassthrough(t *testing.T) {
	client, hook := newHookClient(t)
	defer client.Kill()

	resp, err := hook.PreSynthesis(context.Background(), &gen.HookRequest{
		EventType: gen.EventType_PRE_SYNTHESIS,
		Payload: &gen.HookRequest_InvokeResponses{
			InvokeResponses: &gen.InvokeResponseList{
				Responses: []*gen.InvokeResponse{
					{Content: "resp1", Provider: "claude"},
					{Content: "resp2", Provider: "codex"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("pre_synthesis: %v", err)
	}
	if !resp.Proceed {
		t.Error("expected proceed = true")
	}
	// No modification expected.
	if resp.GetModifiedInvokeRequest() != nil || resp.GetModifiedInvokeResponse() != nil || resp.GetModifiedInvokeResponses() != nil {
		t.Error("expected no modified payload from passthrough")
	}
}

func TestHookPluginPostSessionRecords(t *testing.T) {
	client, hook := newHookClient(t)
	defer client.Kill()

	resp, err := hook.PostSession(context.Background(), &gen.HookRequest{
		EventType: gen.EventType_POST_SESSION,
		Payload: &gen.HookRequest_SessionSummary{
			SessionSummary: &gen.SessionSummary{
				SessionId:    "sess-001",
				Strategy:     "council",
				Providers:    []string{"claude", "codex", "gemini"},
				TotalCostUsd: 0.015,
				TotalTokens:  5000,
				DurationMs:   3200,
			},
		},
	})
	if err != nil {
		t.Fatalf("post_session: %v", err)
	}
	if !resp.Proceed {
		t.Error("expected proceed = true")
	}
}

// ── Cross-Plugin Tests ──────────────────────────────────────────────────────

func TestConcurrentPluginConnections(t *testing.T) {
	// Launch all 3 plugin types simultaneously.
	pClient, provider := newProviderClient(t)
	defer pClient.Kill()
	sClient, strategy := newStrategyClient(t)
	defer sClient.Kill()
	hClient, hook := newHookClient(t)
	defer hClient.Kill()

	ctx := context.Background()

	// All three should work concurrently.
	errs := make(chan error, 3)
	go func() {
		_, err := provider.Invoke(ctx, &gen.InvokeRequest{Prompt: "concurrent provider"})
		errs <- err
	}()
	go func() {
		_, err := strategy.Execute(ctx, &gen.ExecuteRequest{
			Prompt: "concurrent strategy",
			Agents: []*gen.AgentConfig{{Name: "claude"}},
		})
		errs <- err
	}()
	go func() {
		_, err := hook.PreInvoke(ctx, &gen.HookRequest{
			EventType: gen.EventType_PRE_INVOKE,
			Payload: &gen.HookRequest_InvokeRequest{
				InvokeRequest: &gen.InvokeRequest{Prompt: "concurrent hook"},
			},
		})
		errs <- err
	}()

	for range 3 {
		if err := <-errs; err != nil {
			t.Errorf("concurrent call: %v", err)
		}
	}
}

func TestPluginCrashRecovery(t *testing.T) {
	// Launch provider, kill it, verify error, relaunch.
	client1, provider1 := newProviderClient(t)

	// Verify it works.
	_, err := provider1.Invoke(context.Background(), &gen.InvokeRequest{Prompt: "before crash"})
	if err != nil {
		t.Fatalf("pre-crash invoke: %v", err)
	}

	// Kill abruptly.
	client1.Kill()

	// Call on dead client should fail.
	_, err = provider1.Invoke(context.Background(), &gen.InvokeRequest{Prompt: "after crash"})
	if err == nil {
		t.Fatal("expected error after killing plugin")
	}

	// Relaunch — should work.
	client2, provider2 := newProviderClient(t)
	defer client2.Kill()

	resp, err := provider2.Invoke(context.Background(), &gen.InvokeRequest{Prompt: "after relaunch"})
	if err != nil {
		t.Fatalf("post-relaunch invoke: %v", err)
	}
	if resp.Content != "Mock response to: after relaunch" {
		t.Errorf("content = %q", resp.Content)
	}
}

func TestPluginBinaryMissing(t *testing.T) {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  plugin.ProviderHandshake,
		Plugins:          plugin.ProviderPluginMap,
		Cmd:              exec.Command("/nonexistent/path/to/plugin"),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           testLogger(),
	})
	defer client.Kill()

	_, err := client.Client()
	if err == nil {
		t.Fatal("expected error for missing plugin binary")
	}
}

func TestPluginHandshakeMismatch(t *testing.T) {
	// Try to connect to mock-provider with the strategy handshake — should fail.
	binDir := mockPluginBinDir(t)
	pluginPath := filepath.Join(binDir, "mock-provider")

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  plugin.StrategyHandshake, // wrong handshake!
		Plugins:          plugin.StrategyPluginMap,
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           testLogger(),
	})
	defer client.Kill()

	_, err := client.Client()
	if err == nil {
		t.Fatal("expected handshake mismatch error")
	}
}

// ── Full Pipeline Test ──────────────────────────────────────────────────────

func TestFullPipelineHookProviderRoundtrip(t *testing.T) {
	// Simulate: hook rewrites prompt → provider invokes → hook redacts response.
	pClient, provider := newProviderClient(t)
	defer pClient.Kill()
	hClient, hook := newHookClient(t)
	defer hClient.Kill()

	ctx := context.Background()
	originalPrompt := "explain quantum computing"

	// Step 1: PreInvoke hook rewrites prompt.
	hookResp, err := hook.PreInvoke(ctx, &gen.HookRequest{
		EventType: gen.EventType_PRE_INVOKE,
		Payload: &gen.HookRequest_InvokeRequest{
			InvokeRequest: &gen.InvokeRequest{Prompt: originalPrompt},
		},
	})
	if err != nil {
		t.Fatalf("pre_invoke: %v", err)
	}
	modifiedReq := hookResp.GetModifiedInvokeRequest()
	if modifiedReq == nil {
		t.Fatal("expected modified request")
	}

	// Step 2: Provider invokes with modified prompt.
	invokeResp, err := provider.Invoke(ctx, modifiedReq)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if invokeResp.Content != "Mock response to: [hook] explain quantum computing" {
		t.Errorf("invoke content = %q", invokeResp.Content)
	}

	// Step 3: PostInvoke hook redacts response.
	postResp, err := hook.PostInvoke(ctx, &gen.HookRequest{
		EventType: gen.EventType_POST_INVOKE,
		Payload: &gen.HookRequest_InvokeResponse{
			InvokeResponse: invokeResp,
		},
	})
	if err != nil {
		t.Fatalf("post_invoke: %v", err)
	}
	redacted := postResp.GetModifiedInvokeResponse()
	if redacted == nil {
		t.Fatal("expected modified response")
	}
	expected := "Mock response to: [hook] explain quantum computing [redacted]"
	if redacted.Content != expected {
		t.Errorf("redacted content = %q, want %q", redacted.Content, expected)
	}
}
