package providers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/providers"
)

// successGeminiJSON builds a minimal valid gemini JSON response.
// Mirrors the dual-model architecture verified in Spike 0.3.
func successGeminiJSON(t *testing.T, responseText, mainModel string) []byte {
	t.Helper()
	resp := map[string]any{
		"session_id": "gemini-session-abc",
		"response":   responseText,
		"stats": map[string]any{
			"models": map[string]any{
				"gemini-2.5-flash-lite": map[string]any{
					"tokens": map[string]any{
						"input": 50, "prompt": 50, "candidates": 5,
						"total": 55, "cached": 0, "thoughts": 0, "tool": 0,
					},
					"roles": map[string]any{
						"utility_router": map[string]any{
							"tokens": map[string]any{
								"input": 50, "prompt": 50, "candidates": 5,
								"total": 55, "cached": 0, "thoughts": 0, "tool": 0,
							},
						},
					},
				},
				mainModel: map[string]any{
					"tokens": map[string]any{
						"input": 803, "prompt": 803, "candidates": 35,
						"total": 979, "cached": 0, "thoughts": 141, "tool": 0,
					},
					"roles": map[string]any{
						"main": map[string]any{
							"tokens": map[string]any{
								"input": 803, "prompt": 803, "candidates": 35,
								"total": 979, "cached": 0, "thoughts": 141, "tool": 0,
							},
						},
					},
				},
			},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal successGeminiJSON: %v", err)
	}
	return b
}

func TestGeminiProviderName(t *testing.T) {
	p := providers.NewGeminiProvider(defaultConfig(t), &mockRunner{})
	if got := p.Name(); got != "gemini" {
		t.Errorf("Name() = %q, want %q", got, "gemini")
	}
}

func TestGeminiProviderInvokeSuccess(t *testing.T) {
	runner := &mockRunner{stdout: successGeminiJSON(t, "PONG", "gemini-3.1-pro-preview")}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "PONG" {
		t.Errorf("Content = %q, want %q", result.Content, "PONG")
	}
	if result.SessionID != "gemini-session-abc" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "gemini-session-abc")
	}
	if result.Model != "gemini-3.1-pro-preview" {
		t.Errorf("Model = %q, want %q", result.Model, "gemini-3.1-pro-preview")
	}
}

func TestGeminiProviderInvokeMainModelExtracted(t *testing.T) {
	// Primary model is the one with "main" role; tokens from that role.
	runner := &mockRunner{stdout: successGeminiJSON(t, "PONG", "gemini-3.1-pro-preview")}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "gemini-3.1-pro-preview" {
		t.Errorf("Model = %q, want %q", result.Model, "gemini-3.1-pro-preview")
	}
	if result.TokensIn != 803 {
		t.Errorf("TokensIn = %d, want 803 (from main role)", result.TokensIn)
	}
	if result.TokensOut != 35 {
		t.Errorf("TokensOut = %d, want 35 (candidates from main role)", result.TokensOut)
	}
}

func TestGeminiProviderInvokeArgs(t *testing.T) {
	runner := &mockRunner{stdout: successGeminiJSON(t, "ok", "gemini-3.1-pro-preview")}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := runner.capturedArgs
	// Verify --output-format json is present.
	foundFormat := false
	for i, arg := range args {
		if arg == "--output-format" && i+1 < len(args) && args[i+1] == "json" {
			foundFormat = true
			break
		}
	}
	if !foundFormat {
		t.Errorf("--output-format json not found in args: %v", args)
	}
	foundModel := false
	for i, arg := range args {
		if arg == "--model" && i+1 < len(args) && args[i+1] == "gemini-3.1-pro-preview" {
			foundModel = true
			break
		}
	}
	if !foundModel {
		t.Errorf("--model gemini-3.1-pro-preview not found in args: %v", args)
	}
	// Verify prompt is the last arg.
	if args[len(args)-1] != "Say PONG" {
		t.Errorf("last arg = %q, want %q", args[len(args)-1], "Say PONG")
	}
}

func TestGeminiProviderModelOverride(t *testing.T) {
	const overrideModel = "gemini-3-flash-preview"
	runner := &mockRunner{stdout: successGeminiJSON(t, "ok", overrideModel)}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{
		Model: overrideModel,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != overrideModel {
		t.Errorf("Model = %q, want %q", result.Model, overrideModel)
	}

	found := false
	for i, arg := range runner.capturedArgs {
		if arg == "--model" && i+1 < len(runner.capturedArgs) && runner.capturedArgs[i+1] == overrideModel {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("--model %s not found in args: %v", overrideModel, runner.capturedArgs)
	}
}

func TestGeminiProviderStripsModelFlagsFromConfig(t *testing.T) {
	cfg := defaultConfig(t)
	pc := cfg.Providers["gemini"]
	pc.Flags = append([]string{"--model", "legacy-model", "-m=older-model"}, pc.Flags...)
	cfg.Providers["gemini"] = pc

	runner := &mockRunner{stdout: successGeminiJSON(t, "ok", "gemini-3.1-pro-preview")}
	p := providers.NewGeminiProvider(cfg, runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "gemini-3.1-pro-preview" {
		t.Errorf("Model = %q, want %q", result.Model, "gemini-3.1-pro-preview")
	}

	modelFlags := 0
	for i, arg := range runner.capturedArgs {
		switch {
		case arg == "--model":
			modelFlags++
			if i+1 >= len(runner.capturedArgs) || runner.capturedArgs[i+1] != "gemini-3.1-pro-preview" {
				t.Fatalf("expected --model gemini-3.1-pro-preview in args, got %v", runner.capturedArgs)
			}
		case arg == "-m" || strings.HasPrefix(arg, "--model=") || strings.HasPrefix(arg, "-m="):
			t.Fatalf("legacy model flag %q should have been removed from args: %v", arg, runner.capturedArgs)
		}
	}
	if modelFlags != 1 {
		t.Fatalf("expected exactly one --model flag, got %d in args: %v", modelFlags, runner.capturedArgs)
	}
}

func TestGeminiProviderInvokeNonZeroExit(t *testing.T) {
	runner := &mockRunner{
		stderr:   []byte("Error: authentication required"),
		exitCode: 1,
	}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("error %q should contain stderr message", err.Error())
	}
}

func TestGeminiProviderInvokeNonZeroExitEmptyStderr(t *testing.T) {
	runner := &mockRunner{exitCode: 2}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	if !strings.Contains(err.Error(), "exit code") {
		t.Errorf("error %q should mention exit code when stderr is empty", err.Error())
	}
}

func TestGeminiProviderInvokeEmptyStdout(t *testing.T) {
	runner := &mockRunner{stdout: []byte{}, exitCode: 0}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for empty stdout, got nil")
	}
}

func TestGeminiProviderInvokeRunnerError(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found")}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "binary not found") {
		t.Errorf("error %q should contain %q", err.Error(), "binary not found")
	}
}

func TestGeminiProviderInvokeTimeout(t *testing.T) {
	p := providers.NewGeminiProvider(defaultConfig(t), &mockRunner{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invoke

	_, err := p.Invoke(ctx, "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestGeminiProviderHealthCheck(t *testing.T) {
	runner := &mockRunner{stdout: []byte("gemini 0.30.0\n")}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Healthy {
		t.Errorf("expected Healthy=true, got error: %s", status.Error)
	}
	if status.CLIVersion != "0.30.0" {
		t.Errorf("CLIVersion = %q, want %q", status.CLIVersion, "0.30.0")
	}
	if !status.AuthValid {
		t.Error("expected AuthValid=true")
	}
}

func TestGeminiProviderHealthCheckBinaryMissing(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found: no such file or directory")}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

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

func TestGeminiProviderHealthCheckNonZeroExit(t *testing.T) {
	runner := &mockRunner{
		stderr:   []byte("unknown flag: --version"),
		exitCode: 2,
	}
	p := providers.NewGeminiProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false on non-zero exit")
	}
}
