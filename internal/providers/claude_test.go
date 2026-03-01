package providers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// mockRunner implements providers.Runner for unit tests.
// It records the last binary and args passed to Run.
type mockRunner struct {
	stdout       []byte
	stderr       []byte
	exitCode     int
	err          error
	capturedBin  string
	capturedArgs []string
}

func (m *mockRunner) Run(ctx context.Context, binary string, args []string, opts ...core.RunOption) (*core.SubprocessResult, error) {
	m.capturedBin = binary
	m.capturedArgs = args
	if ctx.Err() != nil {
		return &core.SubprocessResult{ExitCode: -1}, ctx.Err()
	}
	if m.err != nil {
		return nil, m.err
	}
	return &core.SubprocessResult{
		Stdout:   m.stdout,
		Stderr:   m.stderr,
		ExitCode: m.exitCode,
	}, nil
}

// defaultConfig returns a Config loaded with defaults only (no YAML file).
func defaultConfig(t *testing.T) *core.Config {
	t.Helper()
	cfg, err := core.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

// successJSON returns a minimal valid claude JSON response for the given content and model.
func successJSON(t *testing.T, content, model string) []byte {
	t.Helper()
	resp := map[string]any{
		"is_error":       false,
		"result":         content,
		"session_id":     "test-session-abc",
		"total_cost_usd": 0.001,
		"duration_ms":    150,
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 5,
		},
		"modelUsage": map[string]any{
			model: map[string]any{
				"inputTokens":  10,
				"outputTokens": 5,
				"costUSD":      0.001,
			},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal successJSON: %v", err)
	}
	return b
}

// errorJSON returns a minimal claude JSON error response.
func errorJSON(t *testing.T, msg string) []byte {
	t.Helper()
	resp := map[string]any{
		"is_error":       true,
		"result":         msg,
		"session_id":     "",
		"total_cost_usd": 0.0,
		"usage": map[string]any{
			"input_tokens":  0,
			"output_tokens": 0,
		},
		"modelUsage": map[string]any{},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal errorJSON: %v", err)
	}
	return b
}

func TestClaudeProviderName(t *testing.T) {
	p := providers.NewClaudeProvider(defaultConfig(t), &mockRunner{})
	if got := p.Name(); got != "claude" {
		t.Errorf("Name() = %q, want %q", got, "claude")
	}
}

func TestClaudeProviderInvokeSuccess(t *testing.T) {
	runner := &mockRunner{
		stdout: successJSON(t, "PONG", "claude-sonnet-4-6"),
	}
	p := providers.NewClaudeProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "PONG" {
		t.Errorf("Content = %q, want %q", result.Content, "PONG")
	}
	if result.SessionID != "test-session-abc" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "test-session-abc")
	}
	if result.CostUSD != 0.001 {
		t.Errorf("CostUSD = %f, want 0.001", result.CostUSD)
	}
	if result.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", result.Model, "claude-sonnet-4-6")
	}
	if result.TokensIn != 10 {
		t.Errorf("TokensIn = %d, want 10", result.TokensIn)
	}
	if result.TokensOut != 5 {
		t.Errorf("TokensOut = %d, want 5", result.TokensOut)
	}
}

func TestClaudeProviderInvokeError(t *testing.T) {
	runner := &mockRunner{
		stdout:   errorJSON(t, "Invalid model xyz"),
		exitCode: 1,
	}
	p := providers.NewClaudeProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid model xyz") {
		t.Errorf("error %q should contain %q", err.Error(), "Invalid model xyz")
	}
}

func TestClaudeProviderInvokeTimeout(t *testing.T) {
	p := providers.NewClaudeProvider(defaultConfig(t), &mockRunner{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invoke

	_, err := p.Invoke(ctx, "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestClaudeProviderModelOverride(t *testing.T) {
	const overrideModel = "claude-haiku-4-5-20251001"
	runner := &mockRunner{
		stdout: successJSON(t, "PONG", overrideModel),
	}
	p := providers.NewClaudeProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{
		Model: overrideModel,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != overrideModel {
		t.Errorf("Model = %q, want %q", result.Model, overrideModel)
	}

	// Verify --model flag was passed to the subprocess.
	found := false
	for i, arg := range runner.capturedArgs {
		if arg == "--model" && i+1 < len(runner.capturedArgs) && runner.capturedArgs[i+1] == overrideModel {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("--model %s not found in subprocess args: %v", overrideModel, runner.capturedArgs)
	}
}

func TestClaudeProviderInvokeDoubleJSON(t *testing.T) {
	// Spike 0.2 §2: claude sometimes emits the JSON object twice on error.
	// Verify that the first object is parsed and the duplicate is ignored.
	single := errorJSON(t, "model not found")
	doubled := make([]byte, 0, 2*len(single))
	doubled = append(doubled, single...)
	doubled = append(doubled, single...)
	runner := &mockRunner{stdout: doubled, exitCode: 1}
	p := providers.NewClaudeProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error from is_error response, got nil")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("error %q should contain %q", err.Error(), "model not found")
	}
}

func TestClaudeProviderHealthCheck(t *testing.T) {
	runner := &mockRunner{stdout: []byte("claude 2.1.63\n")}
	p := providers.NewClaudeProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Healthy {
		t.Errorf("expected Healthy=true, got error: %s", status.Error)
	}
	if status.CLIVersion != "2.1.63" {
		t.Errorf("CLIVersion = %q, want %q", status.CLIVersion, "2.1.63")
	}
	if !status.AuthValid {
		t.Error("expected AuthValid=true")
	}
	if status.Model == "" {
		t.Error("expected non-empty Model")
	}
}

func TestClaudeProviderHealthCheckBinaryMissing(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found: no such file or directory")}
	p := providers.NewClaudeProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false when binary is missing")
	}
	if status.Error == "" {
		t.Error("expected non-empty Error field")
	}
}

func TestClaudeProviderHealthCheckNonZeroExit(t *testing.T) {
	runner := &mockRunner{
		stderr:   []byte("unknown flag: --version"),
		exitCode: 2,
	}
	p := providers.NewClaudeProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false on non-zero exit")
	}
}
