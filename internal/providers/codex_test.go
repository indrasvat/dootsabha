package providers_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/providers"
)

// successJSONL returns a canonical Codex JSONL fixture for the given message.
// Mirrors the real CLI output from Spike 0.1 including reconnect error events.
func successJSONL(agentText string) []byte {
	lines := []string{
		`{"type":"thread.started","thread_id":"019ca5e7-60a5-7c90-b8a6-59ff078ba683"}`,
		`{"type":"turn.started"}`,
		`{"type":"error","message":"Reconnecting... 2/5 (stream disconnected)"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"error","message":"Falling back from WebSockets to HTTPS transport."}}`,
		`{"type":"item.completed","item":{"id":"item_1","type":"reasoning","text":"**Responding with simple output**"}}`,
		`{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"` + agentText + `"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":18343,"cached_input_tokens":3456,"output_tokens":79}}`,
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func TestCodexProviderName(t *testing.T) {
	p := providers.NewCodexProvider(defaultConfig(t), &mockRunner{})
	if got := p.Name(); got != "codex" {
		t.Errorf("Name() = %q, want %q", got, "codex")
	}
}

func TestCodexProviderInvokeSuccess(t *testing.T) {
	runner := &mockRunner{stdout: successJSONL("PONG")}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "PONG" {
		t.Errorf("Content = %q, want %q", result.Content, "PONG")
	}
	if result.TokensIn != 18343 {
		t.Errorf("TokensIn = %d, want 18343", result.TokensIn)
	}
	if result.TokensOut != 79 {
		t.Errorf("TokensOut = %d, want 79", result.TokensOut)
	}
}

func TestCodexProviderInvokeArgs(t *testing.T) {
	runner := &mockRunner{stdout: successJSONL("ok")}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify "exec" and "--json" are in args.
	args := runner.capturedArgs
	if len(args) < 2 || args[0] != "exec" || args[1] != "--json" {
		t.Errorf("args[0:2] = %v, want [exec --json]", args[:min(2, len(args))])
	}
	// Verify prompt is the last arg.
	if args[len(args)-1] != "Say PONG" {
		t.Errorf("last arg = %q, want %q", args[len(args)-1], "Say PONG")
	}
}

func TestCodexProviderInvokeMissingAgentMessage(t *testing.T) {
	// Stream with no agent_message item → error.
	noMsg := `{"type":"thread.started","thread_id":"abc"}` + "\n" +
		`{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5}}` + "\n"
	runner := &mockRunner{stdout: []byte(noMsg)}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for missing agent_message, got nil")
	}
	if !strings.Contains(err.Error(), "no agent_message") {
		t.Errorf("error %q should mention no agent_message", err.Error())
	}
}

func TestCodexProviderInvokeMultipleAgentMessages(t *testing.T) {
	// Multiple agent_message items → last one wins.
	stream := `{"type":"item.completed","item":{"id":"i1","type":"agent_message","text":"first"}}` + "\n" +
		`{"type":"item.completed","item":{"id":"i2","type":"agent_message","text":"last"}}` + "\n" +
		`{"type":"turn.completed","usage":{"input_tokens":5,"output_tokens":2}}` + "\n"
	runner := &mockRunner{stdout: []byte(stream)}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "last" {
		t.Errorf("Content = %q, want %q (last-write-wins)", result.Content, "last")
	}
}

func TestCodexProviderInvokeSkipsErrorEvents(t *testing.T) {
	// Top-level "error" events and item.type=="error" must not halt parsing.
	// The stream should still produce a valid result.
	result, err := providers.NewCodexProvider(defaultConfig(t), &mockRunner{
		stdout: successJSONL("PONG"),
	}).Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error (error events should be skipped): %v", err)
	}
	if result.Content != "PONG" {
		t.Errorf("Content = %q, want PONG", result.Content)
	}
}

func TestCodexProviderInvokeMalformedLines(t *testing.T) {
	// Malformed JSON lines must be skipped; valid lines still parsed.
	stream := "not json at all\n" +
		`{"type":"item.completed","item":{"id":"i1","type":"agent_message","text":"ok"}}` + "\n" +
		"another bad line {\n" +
		`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}` + "\n"
	runner := &mockRunner{stdout: []byte(stream)}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "ok" {
		t.Errorf("Content = %q, want %q", result.Content, "ok")
	}
}

func TestCodexProviderInvokeTimeout(t *testing.T) {
	p := providers.NewCodexProvider(defaultConfig(t), &mockRunner{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invoke

	_, err := p.Invoke(ctx, "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestCodexProviderInvokeRunnerError(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found")}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "binary not found") {
		t.Errorf("error %q should contain %q", err.Error(), "binary not found")
	}
}

func TestCodexProviderHealthCheck(t *testing.T) {
	runner := &mockRunner{stdout: []byte("codex 0.106.0\n")}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Healthy {
		t.Errorf("expected Healthy=true, got error: %s", status.Error)
	}
	if status.CLIVersion != "0.106.0" {
		t.Errorf("CLIVersion = %q, want %q", status.CLIVersion, "0.106.0")
	}
	if !status.AuthValid {
		t.Error("expected AuthValid=true")
	}
}

func TestCodexProviderHealthCheckBinaryMissing(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found: no such file or directory")}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

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

func TestCodexProviderInvokeLargeJSONLLine(t *testing.T) {
	// Verify that lines >64KB (old bufio.Scanner limit) and >1MB parse correctly.
	// This was the root cause of GitHub issue #4, bug 1.
	for _, size := range []int{100_000, 2_000_000} {
		t.Run(fmt.Sprintf("line_%dB", size), func(t *testing.T) {
			// Build an agent_message with a large text field.
			largeText := strings.Repeat("x", size)
			stream := fmt.Sprintf(`{"type":"item.completed","item":{"id":"i1","type":"agent_message","text":"%s"}}`, largeText) + "\n" +
				`{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}` + "\n"
			runner := &mockRunner{stdout: []byte(stream)}
			p := providers.NewCodexProvider(defaultConfig(t), runner)

			result, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
			if err != nil {
				t.Fatalf("unexpected error on %dB line: %v", size, err)
			}
			if len(result.Content) != size {
				t.Errorf("Content length = %d, want %d", len(result.Content), size)
			}
		})
	}
}

func TestCodexProviderHealthCheckNonZeroExit(t *testing.T) {
	runner := &mockRunner{
		stderr:   []byte("unknown flag: --version"),
		exitCode: 2,
	}
	p := providers.NewCodexProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false on non-zero exit")
	}
}
