package providers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/providers"
)

// agyEnvelope builds an `agy --output-format json` document. Field names and the
// token relationship (total == in+out, thinking ⊆ out) mirror agy 1.1.17.
func agyEnvelope(t *testing.T, status, response, errMsg string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"conversation_id":  "3963cf7a-446b-479b-846f-1693117031f9",
		"status":           status,
		"response":         response,
		"error":            errMsg,
		"duration_seconds": 1.5,
		"num_turns":        1,
		"usage": map[string]int{
			"input_tokens": 1801, "output_tokens": 27, "thinking_tokens": 25,
			"cache_read_tokens": 0, "total_tokens": 1828,
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func agyOK(t *testing.T, response string) []byte {
	t.Helper()
	return agyEnvelope(t, "SUCCESS", response, "")
}

func TestAgyProviderName(t *testing.T) {
	p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{})
	if got := p.Name(); got != "agy" {
		t.Errorf("Name() = %q, want %q", got, "agy")
	}
}

func TestAgyProviderInvokeSuccess(t *testing.T) {
	runner := &mockRunner{stdout: agyOK(t, "PONG\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "PONG" {
		t.Errorf("Content = %q, want %q", result.Content, "PONG")
	}
	if result.SessionID != "3963cf7a-446b-479b-846f-1693117031f9" {
		t.Errorf("SessionID = %q, want the conversation_id", result.SessionID)
	}
	if result.TokensIn != 1801 || result.TokensOut != 27 {
		t.Errorf("tokens = (%d,%d), want (1801,27)", result.TokensIn, result.TokensOut)
	}
	// thinking_tokens is a SUBSET of output_tokens; adding it would double-count.
	if result.TokensOut == 27+25 {
		t.Error("TokensOut added thinking_tokens — they are already inside output_tokens")
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0 — agy reports no cost", result.CostUSD)
	}
	if result.Model != providers.AgyDefaultModel {
		t.Errorf("Model = %q, want %q", result.Model, providers.AgyDefaultModel)
	}
}

func TestAgyProviderInvokeMultilineContent(t *testing.T) {
	runner := &mockRunner{stdout: agyOK(t, "  line one\nline two\n\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "explain", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "line one\nline two" {
		t.Errorf("Content = %q, want interior newlines preserved", result.Content)
	}
}

func TestAgyProviderInvokeArgs(t *testing.T) {
	runner := &mockRunner{stdout: agyOK(t, "ok")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	if _, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := runner.capturedArgs

	if !slices.Contains(args, "--dangerously-skip-permissions") {
		t.Errorf("--dangerously-skip-permissions not found in args: %v", args)
	}
	if got := flagValue(args, "--model"); got != providers.AgyDefaultModel {
		t.Errorf("--model = %q, want %q: %v", got, providers.AgyDefaultModel, args)
	}
	if got := flagValue(args, "--output-format"); got != "json" {
		t.Errorf("--output-format = %q, want json: %v", got, args)
	}
	if n := countFlag(args, "--output-format"); n != 1 {
		t.Errorf("--output-format appears %d times, want 1: %v", n, args)
	}
	// agy rejects --effort for the lower tiers; the model suffix carries effort.
	if slices.Contains(args, "--effort") {
		t.Errorf("दूतसभा must not emit --effort: %v", args)
	}
	if len(args) < 2 || args[len(args)-2] != "-p" || args[len(args)-1] != "Say PONG" {
		t.Errorf("expected trailing `-p \"Say PONG\"`, got %v", args)
	}
}

func TestAgyProviderModelOverride(t *testing.T) {
	const override = "Gemini 3.1 Pro (High)"
	runner := &mockRunner{stdout: agyOK(t, "ok")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{Model: override})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != override {
		t.Errorf("Model = %q, want %q", result.Model, override)
	}
	if got := flagValue(runner.capturedArgs, "--model"); got != override {
		t.Errorf("--model = %q, want %q: %v", got, override, runner.capturedArgs)
	}
}

// --model and --output-format are pinned: a config entry must not duplicate or
// override them. `--output-format text` is the dangerous one — it would silently
// break every parse.
func TestAgyProviderStripsPinnedFlagsFromConfig(t *testing.T) {
	for _, flags := range [][]string{
		{"--model", "legacy-model"},
		{"-m=older-model"},
		{"--model=other"},
		{"--output-format", "text"},
		{"--output-format=stream-json"},
		{"--model", "legacy", "--output-format", "text", "-m=x"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			cfg := defaultConfig(t)
			pc := cfg.Providers["agy"]
			pc.Flags = append(slices.Clone(flags), pc.Flags...)
			cfg.Providers["agy"] = pc

			runner := &mockRunner{stdout: agyOK(t, "ok")}
			p := providers.NewAgyProvider(cfg, runner)
			if _, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			args := runner.capturedArgs

			if n := countFlag(args, "--model"); n != 1 {
				t.Errorf("--model appears %d times, want 1: %v", n, args)
			}
			if got := flagValue(args, "--model"); got != providers.AgyDefaultModel {
				t.Errorf("--model = %q, want the resolved default: %v", got, args)
			}
			if got := flagValue(args, "--output-format"); got != "json" {
				t.Errorf("--output-format = %q, want json: %v", got, args)
			}
			if n := countFlag(args, "--output-format"); n != 1 {
				t.Errorf("--output-format appears %d times, want 1: %v", n, args)
			}
			for _, a := range args {
				if a == "-m" || strings.HasPrefix(a, "-m=") || strings.HasPrefix(a, "--model=") ||
					strings.HasPrefix(a, "--output-format=") {
					t.Errorf("pinned flag %q survived: %v", a, args)
				}
			}
		})
	}
}

// Clearing providers.agy.model is a deliberate opt-out: दूतसभा emits no --model,
// so the user's own flag must survive rather than be stripped for nothing.
func TestAgyProviderEmptyModelKeepsUserFlag(t *testing.T) {
	cfg := defaultConfig(t)
	pc := cfg.Providers["agy"]
	pc.Model = ""
	pc.Flags = []string{"--model", "Gemini 3.1 Pro (Low)"}
	cfg.Providers["agy"] = pc

	runner := &mockRunner{stdout: agyOK(t, "ok")}
	p := providers.NewAgyProvider(cfg, runner)
	if _, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := flagValue(runner.capturedArgs, "--model"); got != "Gemini 3.1 Pro (Low)" {
		t.Errorf("--model = %q, want the user's flag: %v", got, runner.capturedArgs)
	}
}

// In JSON mode agy writes the reason to STDOUT and leaves stderr EMPTY. Reading
// stderr alone loses it entirely — the regression this guards.
func TestAgyProviderInvokeErrorEnvelopeOnStdout(t *testing.T) {
	const msg = `invalid model selection (--model "nope" --effort ""): model nope is not recognized`
	runner := &mockRunner{stdout: agyEnvelope(t, "ERROR", "", msg), exitCode: 1}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not recognized") {
		t.Errorf("error %q must carry the envelope's message", err.Error())
	}
}

// A tool call failing inside a turn that still answered is exit 0 + status
// "ERROR" + a populated response. Observed live. Rejecting it discards a usable
// answer, so status must not be the discriminator.
func TestAgyProviderDegradedTurnStillSucceeds(t *testing.T) {
	runner := &mockRunner{
		stdout: agyEnvelope(t, "ERROR", "6\n", "Find command timed out.: context deadline exceeded"),
	}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "count funcs", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("degraded turn must not fail the invocation: %v", err)
	}
	if result.Content != "6" {
		t.Errorf("Content = %q, want the answer kept", result.Content)
	}
}

func TestAgyProviderInvokeFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runner  *mockRunner
		wantSub string
	}{
		{"non-zero exit falls back to stderr", &mockRunner{
			stdout: []byte("not json"), stderr: []byte("Error: authentication required"), exitCode: 1,
		}, "authentication required"},
		{"non-zero exit with nothing to report", &mockRunner{exitCode: 2}, "exit code 2"},
		{"exit 0 but empty stdout", &mockRunner{}, "empty output"},
		{"exit 0 but unparseable", &mockRunner{stdout: []byte("Fetching available models...")}, "json parse"},
		{"empty response with an error field", &mockRunner{
			stdout: agyEnvelope(t, "ERROR", "", "quota exhausted"),
		}, "quota exhausted"},
		{"empty response with no error field", &mockRunner{
			stdout: agyEnvelope(t, "SUCCESS", "  \n", ""),
		}, "empty response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := providers.NewAgyProvider(defaultConfig(t), tc.runner)
			_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q should contain %q", err.Error(), tc.wantSub)
			}
		})
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

	if _, err := p.Invoke(ctx, "hello", providers.InvokeOptions{}); err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestAgyProviderHealthCheck(t *testing.T) {
	runner := &mockRunner{stdout: []byte("1.1.17\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Healthy {
		t.Errorf("expected Healthy=true, got error: %s", status.Error)
	}
	if status.CLIVersion != "1.1.17" {
		t.Errorf("CLIVersion = %q, want %q", status.CLIVersion, "1.1.17")
	}
	if status.Model != providers.AgyDefaultModel {
		t.Errorf("Model = %q, want %q", status.Model, providers.AgyDefaultModel)
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
	runner := &mockRunner{stderr: []byte("unknown flag: --version"), exitCode: 2}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false on non-zero exit")
	}
}
