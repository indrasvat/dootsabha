package providers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
)

func TestGrokProviderName(t *testing.T) {
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{})
	if got := p.Name(); got != "grok" {
		t.Errorf("Name() = %q, want %q", got, "grok")
	}
}

// GrokProvider must satisfy the shared Provider interface like its siblings.
func TestGrokProviderImplementsProvider(t *testing.T) {
	var _ providers.Provider = providers.NewGrokProvider(defaultConfig(t), &mockRunner{})
}

// flagValue returns the argument following name, or "" if absent.
func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func countFlag(args []string, name string) int {
	n := 0
	for _, a := range args {
		if a == name {
			n++
		}
	}
	return n
}

// The pinned argv is the whole contract. Every flag here is load-bearing:
// streaming-messages-json (json's .text concatenates tool-call preambles),
// --sandbox read-only (containment), --no-plan (else grok returns a plan).
func TestGrokProviderInvokeArgs(t *testing.T) {
	runner := &mockRunner{}
	p := providers.NewGrokProvider(defaultConfig(t), runner)

	_, _ = p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	args := runner.capturedArgs

	if got := flagValue(args, "--output-format"); got != "streaming-messages-json" {
		t.Errorf("--output-format = %q, want streaming-messages-json (json's .text is polluted)", got)
	}
	if got := flagValue(args, "--sandbox"); got != "read-only" {
		t.Errorf("--sandbox = %q, want read-only (containment escape without it)", got)
	}
	if got := flagValue(args, "--permission-mode"); got != "bypassPermissions" {
		t.Errorf("--permission-mode = %q, want bypassPermissions", got)
	}
	if got := flagValue(args, "-m"); got != "grok-4.5" {
		t.Errorf("-m = %q, want grok-4.5", got)
	}
	if got := flagValue(args, "--reasoning-effort"); got != "high" {
		t.Errorf("--reasoning-effort = %q, want high", got)
	}
	for _, want := range []string{"--always-approve", "--no-plan", "--no-subagents", "--no-auto-update"} {
		if !slices.Contains(args, want) {
			t.Errorf("%s missing from args: %v", want, args)
		}
	}
	if got := flagValue(args, "-p"); got != "Say PONG" {
		t.Errorf("-p = %q, want the prompt", got)
	}

	// --verbatim only strips the <user_query> delimiter grok's own system prompt is
	// written against; grok never rewrites prompts. Passing it would degrade, not help.
	if slices.Contains(args, "--verbatim") {
		t.Errorf("--verbatim must NOT be passed: %v", args)
	}
	// json mode is explicitly forbidden.
	if flagValue(args, "--output-format") == "json" {
		t.Error("--output-format json is forbidden")
	}
}

// A stray flag in user config must never be able to break parsing or reopen the
// containment hole. Pinned flags are stripped from config and re-applied.
func TestGrokProviderStripsPinnedFlagsFromConfig(t *testing.T) {
	cfg := defaultConfig(t)
	pc := cfg.Providers["grok"]
	pc.Flags = append([]string{
		"--output-format", "plain",
		"--sandbox", "off",
		"--permission-mode", "default",
		"--verbatim",
		"-m", "legacy-model",
		"--keep-me",
	}, pc.Flags...)
	cfg.Providers["grok"] = pc

	runner := &mockRunner{}
	p := providers.NewGrokProvider(cfg, runner)
	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	args := runner.capturedArgs

	if got := flagValue(args, "--output-format"); got != "streaming-messages-json" {
		t.Errorf("config must not override --output-format; got %q", got)
	}
	if got := flagValue(args, "--sandbox"); got != "read-only" {
		t.Errorf("config must not override --sandbox; got %q", got)
	}
	if got := flagValue(args, "--permission-mode"); got != "bypassPermissions" {
		t.Errorf("config must not override --permission-mode; got %q", got)
	}
	if got := flagValue(args, "-m"); got != "grok-4.5" {
		t.Errorf("config must not override -m; got %q", got)
	}
	if slices.Contains(args, "--verbatim") {
		t.Errorf("--verbatim from config must be stripped: %v", args)
	}
	// Exactly one of each pinned flag — no duplicates.
	for _, f := range []string{"--output-format", "--sandbox", "--permission-mode", "-m"} {
		if n := countFlag(args, f); n != 1 {
			t.Errorf("%s appears %d times, want exactly 1: %v", f, n, args)
		}
	}
	// Non-pinned config flags survive.
	if !slices.Contains(args, "--keep-me") {
		t.Errorf("non-pinned config flag --keep-me was dropped: %v", args)
	}
}

func TestGrokProviderModelOverride(t *testing.T) {
	const override = "grok-4.5-fast"
	runner := &mockRunner{}
	p := providers.NewGrokProvider(defaultConfig(t), runner)

	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{Model: override})

	if got := flagValue(runner.capturedArgs, "-m"); got != override {
		t.Errorf("-m = %q, want %q", got, override)
	}
	if n := countFlag(runner.capturedArgs, "-m"); n != 1 {
		t.Errorf("-m appears %d times, want 1", n)
	}
}

// Reasoning effort is user-tunable (not correctness-critical), so config wins —
// but it defaults to high when unset.
func TestGrokProviderReasoningEffortFromConfig(t *testing.T) {
	cfg := defaultConfig(t)
	pc := cfg.Providers["grok"]
	// Viper replaces the flags list wholesale, so model the real shape rather than
	// prepending to the defaults (which would leave two effort flags).
	pc.Flags = []string{"--reasoning-effort", "low"}
	cfg.Providers["grok"] = pc

	runner := &mockRunner{}
	p := providers.NewGrokProvider(cfg, runner)
	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})

	if got := flagValue(runner.capturedArgs, "--reasoning-effort"); got != "low" {
		t.Errorf("--reasoning-effort = %q, want low (config should win)", got)
	}
	if n := countFlag(runner.capturedArgs, "--reasoning-effort"); n != 1 {
		t.Errorf("--reasoning-effort appears %d times, want 1: %v", n, runner.capturedArgs)
	}
}

func TestGrokProviderMaxTurns(t *testing.T) {
	runner := &mockRunner{}
	p := providers.NewGrokProvider(defaultConfig(t), runner)

	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{MaxTurns: 7})

	if got := flagValue(runner.capturedArgs, "--max-turns"); got != "7" {
		t.Errorf("--max-turns = %q, want 7", got)
	}
}

func TestGrokProviderBinaryFromConfig(t *testing.T) {
	runner := &mockRunner{}
	p := providers.NewGrokProvider(defaultConfig(t), runner)

	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})

	if runner.capturedBin != "grok" {
		t.Errorf("binary = %q, want grok", runner.capturedBin)
	}
}

// providerConfig must not deref a nil *core.Config (all three sibling providers do).
func TestGrokProviderNilConfigDoesNotPanic(t *testing.T) {
	runner := &mockRunner{}
	p := providers.NewGrokProvider(nil, runner)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil config panicked: %v", r)
		}
	}()
	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})

	if got := flagValue(runner.capturedArgs, "-m"); got != "grok-4.5" {
		t.Errorf("nil config should fall back to defaults; -m = %q", got)
	}
}

// A partial config block must merge with defaults rather than blank out Binary/Flags.
func TestGrokProviderPartialConfigMergesDefaults(t *testing.T) {
	cfg := defaultConfig(t)
	cfg.Providers["grok"] = core.ProviderConfig{} // all fields zero

	runner := &mockRunner{}
	p := providers.NewGrokProvider(cfg, runner)
	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})

	if runner.capturedBin != "grok" {
		t.Errorf("empty Binary should fall back to %q, got %q", "grok", runner.capturedBin)
	}
	if got := flagValue(runner.capturedArgs, "-m"); got != "grok-4.5" {
		t.Errorf("empty Model should fall back to grok-4.5, got %q", got)
	}
}

func TestGrokProviderInvokeRunnerError(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found")}
	p := providers.NewGrokProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "binary not found") {
		t.Errorf("error %q should wrap the runner error", err.Error())
	}
}

func TestGrokProviderInvokeCancelledContext(t *testing.T) {
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := p.Invoke(ctx, "hi", providers.InvokeOptions{}); err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// --- NDJSON parsing -------------------------------------------------------

// resultLine builds a grok streaming `result` event.
func resultLine(text string, in, out int, cost float64) string {
	return `{"type":"result","subtype":"success","is_error":false,` +
		`"result":` + quote(text) + `,"stop_reason":"end_turn",` +
		`"session_id":"019fbc10-9c47-77f2-a8d4-046ae03de003",` +
		`"duration_ms":106781,"num_turns":1,` +
		`"total_cost_usd":` + fmt.Sprintf("%g", cost) + `,` +
		`"usage":{"input_tokens":` + fmt.Sprint(in) + `,"output_tokens":` + fmt.Sprint(out) +
		`,"cache_read_input_tokens":62336,"cache_creation_input_tokens":0},` +
		`"modelUsage":{"grok-4.5-build":{"inputTokens":` + fmt.Sprint(in) +
		`,"outputTokens":` + fmt.Sprint(out) + `,"costUSD":` + fmt.Sprintf("%g", cost) + `}}}`
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

const (
	grokSystemLine    = `{"type":"system","subtype":"init","session_id":"019fbc10","model":"grok-4.5"}`
	grokAssistantLine = `{"type":"assistant","message":{"id":"msg_0","role":"assistant",` +
		`"content":[{"type":"text","text":"I'll read the file first."}]}}`
)

func TestGrokProviderInvokeSuccess(t *testing.T) {
	stream := strings.Join([]string{
		grokSystemLine,
		grokAssistantLine,
		resultLine("# Review\n\nLooks good.", 25956, 5724, 0.1049568),
	}, "\n")
	runner := &mockRunner{stdout: []byte(stream)}
	p := providers.NewGrokProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "review this", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Content comes from the result line only — NOT from concatenated assistant blocks.
	if result.Content != "# Review\n\nLooks good." {
		t.Errorf("Content = %q; must be the result line, not assistant preambles", result.Content)
	}
	if result.SessionID != "019fbc10-9c47-77f2-a8d4-046ae03de003" {
		t.Errorf("SessionID = %q", result.SessionID)
	}
	if result.TokensIn != 25956 || result.TokensOut != 5724 {
		t.Errorf("tokens = (%d,%d), want (25956,5724)", result.TokensIn, result.TokensOut)
	}
	if result.CostUSD != 0.1049568 {
		t.Errorf("CostUSD = %v, want 0.1049568", result.CostUSD)
	}
	// modelUsage key deliberately differs from the -m value.
	if result.Model != "grok-4.5-build" {
		t.Errorf("Model = %q, want grok-4.5-build (modelUsage key, not the -m value)", result.Model)
	}
}

func TestGrokProviderLastResultWins(t *testing.T) {
	stream := strings.Join([]string{
		resultLine("first", 1, 1, 0.1),
		resultLine("second", 2, 2, 0.2),
	}, "\n")
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{stdout: []byte(stream)})

	result, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "second" {
		t.Errorf("Content = %q, want last result to win", result.Content)
	}
}

func TestGrokProviderSkipsMalformedAndBlankLines(t *testing.T) {
	stream := strings.Join([]string{
		"", "   ", "{not json", grokSystemLine,
		resultLine("ok", 1, 1, 0.01), "",
	}, "\n")
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{stdout: []byte(stream)})

	result, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("malformed lines must be skipped, got: %v", err)
	}
	if result.Content != "ok" {
		t.Errorf("Content = %q", result.Content)
	}
}

// A runtime error is still type=="result"; discriminate on is_error, and the
// message lives in errors[] with no result field at all.
func TestGrokProviderErrorDuringExecution(t *testing.T) {
	stream := grokSystemLine + "\n" +
		`{"type":"result","subtype":"error_during_execution","is_error":true,` +
		`"num_turns":0,"stop_reason":null,"total_cost_usd":0.0,` +
		`"usage":{"input_tokens":0,"output_tokens":0},"modelUsage":{},` +
		`"errors":["Couldn't set model 'nope': unknown model id"],` +
		`"session_id":"019fbc2d"}`
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{stdout: []byte(stream), exitCode: 1})

	_, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for is_error:true, got nil")
	}
	if !strings.Contains(err.Error(), "unknown model id") {
		t.Errorf("error %q should carry the errors[] message", err.Error())
	}
}

// Provider failure must win over content when an error result follows content.
func TestGrokProviderErrorPrecedenceOverContent(t *testing.T) {
	stream := resultLine("partial content", 1, 1, 0.01) + "\n" +
		`{"type":"result","subtype":"error_during_execution","is_error":true,` +
		`"errors":["stream aborted"],"session_id":"x"}`
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{stdout: []byte(stream)})

	_, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("a later error result must override earlier content")
	}
	if !strings.Contains(err.Error(), "stream aborted") {
		t.Errorf("error %q should carry the abort message", err.Error())
	}
}

func TestGrokProviderNoResultLine(t *testing.T) {
	stream := grokSystemLine + "\n" + grokAssistantLine
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{stdout: []byte(stream)})

	if _, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{}); err == nil {
		t.Fatal("expected error when no result line is present")
	}
}

func TestGrokProviderEmptyStdout(t *testing.T) {
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{stdout: []byte("  \n")})

	if _, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{}); err == nil {
		t.Fatal("expected error for empty stdout")
	}
}

func TestGrokProviderNonZeroExitUsesStderr(t *testing.T) {
	runner := &mockRunner{stderr: []byte("Error: authentication required"), exitCode: 1}
	p := providers.NewGrokProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("error %q should surface stderr", err.Error())
	}
}

func TestGrokProviderNonZeroExitEmptyStderr(t *testing.T) {
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{exitCode: 2})

	_, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "exit code") {
		t.Errorf("error %q should mention exit code when stderr is empty", err.Error())
	}
}

// Real grok review lines exceed bufio.Scanner's 64KB token limit (see codex.go:153).
func TestGrokProviderHandlesLineOver64KB(t *testing.T) {
	big := strings.Repeat("A", 100_000)
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{stdout: []byte(resultLine(big, 1, 1, 0.01))})

	result, err := p.Invoke(context.Background(), "x", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("100KB line must parse (Scanner's 64KB limit would fail): %v", err)
	}
	if len(result.Content) != len(big) {
		t.Errorf("Content length = %d, want %d", len(result.Content), len(big))
	}
}

// --- HealthCheck ----------------------------------------------------------

func TestGrokProviderHealthCheck(t *testing.T) {
	runner := &mockRunner{stdout: []byte("grok 0.2.118 (1e1687c1cf6a) [stable]\n")}
	p := providers.NewGrokProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Healthy {
		t.Errorf("expected Healthy=true, got error: %s", status.Error)
	}
	// Assert parseability, never an exact patch version — the binary self-updates.
	if status.CLIVersion == "" || !strings.HasPrefix(status.CLIVersion, "0.") {
		t.Errorf("CLIVersion = %q, want a parsed semver-ish version", status.CLIVersion)
	}
	if len(runner.capturedArgs) != 1 || runner.capturedArgs[0] != "--version" {
		t.Errorf("HealthCheck args = %v, want [--version]", runner.capturedArgs)
	}
}

func TestGrokProviderHealthCheckBinaryMissing(t *testing.T) {
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{err: fmt.Errorf("binary not found")})

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

func TestGrokProviderHealthCheckNonZeroExit(t *testing.T) {
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{stderr: []byte("boom"), exitCode: 2})

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false on non-zero exit")
	}
}

// An unhealthy provider must always carry a reason; a blank Error renders an
// unexplained red row in `dootsabha status`.
func TestGrokProviderHealthCheckNonZeroExitEmptyStderrHasReason(t *testing.T) {
	p := providers.NewGrokProvider(defaultConfig(t), &mockRunner{exitCode: 3})

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Fatal("expected Healthy=false")
	}
	if !strings.Contains(status.Error, "exit code") {
		t.Errorf("Error = %q, want an exit-code fallback when stderr is empty", status.Error)
	}
}
