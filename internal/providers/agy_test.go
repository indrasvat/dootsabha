package providers_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/providers"
)

func TestAgyProviderName(t *testing.T) {
	p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{})
	if got := p.Name(); got != "agy" {
		t.Errorf("Name() = %q, want %q", got, "agy")
	}
}

func TestAgyProviderInvokeSuccess(t *testing.T) {
	// Antigravity print mode emits plain text — Content is the trimmed stdout.
	runner := &mockRunner{stdout: []byte("PONG\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "PONG" {
		t.Errorf("Content = %q, want %q", result.Content, "PONG")
	}
	// Plain-text mode: no token/cost/session data is available.
	if result.TokensIn != 0 || result.TokensOut != 0 {
		t.Errorf("tokens = (%d,%d), want (0,0) — print mode has no token data", result.TokensIn, result.TokensOut)
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0 — print mode has no cost data", result.CostUSD)
	}
	if result.Model != "Gemini 3.5 Flash (High)" {
		t.Errorf("Model = %q, want default %q", result.Model, "Gemini 3.5 Flash (High)")
	}
}

func TestAgyProviderInvokeMultilineContent(t *testing.T) {
	runner := &mockRunner{stdout: []byte("  line one\nline two\n\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "explain", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Leading/trailing whitespace trimmed; interior preserved.
	if result.Content != "line one\nline two" {
		t.Errorf("Content = %q, want %q", result.Content, "line one\nline two")
	}
}

func TestAgyProviderInvokeArgs(t *testing.T) {
	runner := &mockRunner{stdout: []byte("ok")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := runner.capturedArgs

	// Dangerous mode (yolo equivalent) must be present by default.
	foundYolo := slices.Contains(args, "--dangerously-skip-permissions")
	if !foundYolo {
		t.Errorf("--dangerously-skip-permissions not found in args: %v", args)
	}

	// Default model flag.
	foundModel := false
	for i, arg := range args {
		if arg == "--model" && i+1 < len(args) && args[i+1] == "Gemini 3.5 Flash (High)" {
			foundModel = true
			break
		}
	}
	if !foundModel {
		t.Errorf("--model default not found in args: %v", args)
	}

	// agy has NO --output-format flag.
	for _, arg := range args {
		if arg == "--output-format" {
			t.Errorf("agy must not receive --output-format (no JSON mode): %v", args)
		}
	}

	// Prompt passed via -p and kept last.
	if len(args) < 2 || args[len(args)-2] != "-p" || args[len(args)-1] != "Say PONG" {
		t.Errorf("expected trailing `-p \"Say PONG\"`, got %v", args)
	}
}

func TestAgyProviderModelOverride(t *testing.T) {
	const overrideModel = "Gemini 3.1 Pro (High)"
	runner := &mockRunner{stdout: []byte("ok")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

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
		t.Errorf("--model %q not found in args: %v", overrideModel, runner.capturedArgs)
	}
}

func TestAgyProviderStripsModelFlagsFromConfig(t *testing.T) {
	cfg := defaultConfig(t)
	pc := cfg.Providers["agy"]
	pc.Flags = append([]string{"--model", "legacy-model", "-m=older-model"}, pc.Flags...)
	cfg.Providers["agy"] = pc

	runner := &mockRunner{stdout: []byte("ok")}
	p := providers.NewAgyProvider(cfg, runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "Gemini 3.5 Flash (High)" {
		t.Errorf("Model = %q, want %q", result.Model, "Gemini 3.5 Flash (High)")
	}

	modelFlags := 0
	for i, arg := range runner.capturedArgs {
		switch {
		case arg == "--model":
			modelFlags++
			if i+1 >= len(runner.capturedArgs) || runner.capturedArgs[i+1] != "Gemini 3.5 Flash (High)" {
				t.Fatalf("expected --model \"Gemini 3.5 Flash (High)\" in args, got %v", runner.capturedArgs)
			}
		case arg == "-m" || strings.HasPrefix(arg, "--model=") || strings.HasPrefix(arg, "-m="):
			t.Fatalf("legacy model flag %q should have been removed from args: %v", arg, runner.capturedArgs)
		}
	}
	if modelFlags != 1 {
		t.Fatalf("expected exactly one --model flag, got %d in args: %v", modelFlags, runner.capturedArgs)
	}
}

func TestAgyProviderInvokeNonZeroExit(t *testing.T) {
	runner := &mockRunner{
		stderr:   []byte("Error: authentication required"),
		exitCode: 1,
	}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("error %q should contain stderr message", err.Error())
	}
}

func TestAgyProviderInvokeNonZeroExitEmptyStderr(t *testing.T) {
	runner := &mockRunner{exitCode: 2}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	if !strings.Contains(err.Error(), "exit code") {
		t.Errorf("error %q should mention exit code when stderr is empty", err.Error())
	}
}

func TestAgyProviderInvokeEmptyStdout(t *testing.T) {
	runner := &mockRunner{stdout: []byte("   \n"), exitCode: 0}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for empty/whitespace stdout, got nil")
	}
}

func TestAgyProviderInvokeRunnerError(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "binary not found") {
		t.Errorf("error %q should contain %q", err.Error(), "binary not found")
	}
}

func TestAgyProviderInvokeTimeout(t *testing.T) {
	p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invoke

	_, err := p.Invoke(ctx, "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestAgyProviderHealthCheck(t *testing.T) {
	runner := &mockRunner{stdout: []byte("1.0.8\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Healthy {
		t.Errorf("expected Healthy=true, got error: %s", status.Error)
	}
	if status.CLIVersion != "1.0.8" {
		t.Errorf("CLIVersion = %q, want %q", status.CLIVersion, "1.0.8")
	}
	if !status.AuthValid {
		t.Error("expected AuthValid=true")
	}
}

func TestAgyProviderHealthCheckBinaryMissing(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found: no such file or directory")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

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

func TestAgyProviderHealthCheckNonZeroExit(t *testing.T) {
	runner := &mockRunner{
		stderr:   []byte("unknown flag: --version"),
		exitCode: 2,
	}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false on non-zero exit")
	}
}
