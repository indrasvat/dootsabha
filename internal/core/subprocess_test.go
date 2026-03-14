package core

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSubprocessRunner_NormalExecution(t *testing.T) {
	r := &SubprocessRunner{}
	ctx := context.Background()

	result, err := r.Run(ctx, "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	got := strings.TrimSpace(string(result.Stdout))
	if got != "hello" {
		t.Errorf("stdout = %q, want %q", got, "hello")
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestSubprocessRunner_ExitCodeCapture(t *testing.T) {
	r := &SubprocessRunner{}
	ctx := context.Background()

	result, err := r.Run(ctx, "sh", []string{"-c", "exit 42"})
	// err may be nil — we return nil for non-zero exits from the normal path
	_ = err
	if result == nil {
		t.Fatal("result is nil")
		return
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestSubprocessRunner_StdoutStderrSeparation(t *testing.T) {
	r := &SubprocessRunner{}
	ctx := context.Background()

	result, err := r.Run(ctx, "sh", []string{"-c", "echo toout; echo toerr >&2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(result.Stdout), "toout") {
		t.Errorf("stdout %q does not contain 'toout'", result.Stdout)
	}
	if !strings.Contains(string(result.Stderr), "toerr") {
		t.Errorf("stderr %q does not contain 'toerr'", result.Stderr)
	}
	if strings.Contains(string(result.Stdout), "toerr") {
		t.Error("stderr content leaked into stdout")
	}
}

func TestSubprocessRunner_ContextCancellation(t *testing.T) {
	r := &SubprocessRunner{}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately after launch window; process should be killed.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := r.Run(ctx, "sleep", []string{"60"},
		WithGracePeriod(200*time.Millisecond))
	if err == nil {
		t.Fatal("expected error on context cancellation, got nil")
	}
	if result == nil {
		t.Fatal("result should not be nil even on cancellation")
		return
	}
	if result.ExitCode != -1 {
		t.Errorf("exit code = %d, want -1 on cancellation", result.ExitCode)
	}
}

func TestSubprocessRunner_TimeoutEnforcement(t *testing.T) {
	r := &SubprocessRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	result, err := r.Run(ctx, "sleep", []string{"10"},
		WithGracePeriod(200*time.Millisecond))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if result == nil {
		t.Fatal("result should not be nil on timeout")
	}
	// Should have been killed well before the full 10 seconds.
	if elapsed > 2*time.Second {
		t.Errorf("took %v, expected well under 2s", elapsed)
	}
}

func TestSubprocessRunner_GracePeriodKill(t *testing.T) {
	r := &SubprocessRunner{}

	// This script ignores SIGTERM (traps it) but will be SIGKILLed after grace period.
	script := `trap '' TERM; sleep 60`
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := r.Run(ctx, "sh", []string{"-c", script},
		WithGracePeriod(300*time.Millisecond))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error on context cancellation, got nil")
	}
	// Grace period is 300ms; context times out at 100ms; SIGKILL at ~400ms total.
	// Should complete well under 2s.
	if elapsed > 2*time.Second {
		t.Errorf("grace period + kill took %v, expected under 2s", elapsed)
	}
}

func TestSubprocessRunner_WithEnvOption(t *testing.T) {
	r := &SubprocessRunner{}
	ctx := context.Background()

	// Provide a minimal env that includes only TEST_VAR.
	// PATH must be included or sh won't find echo.
	env := []string{
		"TEST_VAR=hello_from_env",
		"PATH=" + os.Getenv("PATH"),
	}
	result, err := r.Run(ctx, "sh", []string{"-c", "echo $TEST_VAR"},
		WithEnv(env))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(string(result.Stdout))
	if got != "hello_from_env" {
		t.Errorf("stdout = %q, want %q", got, "hello_from_env")
	}
}

func TestDetectAndCleanClaude_InsideSession(t *testing.T) {
	// Simulate being inside a Claude Code session.
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")

	DetectAndCleanClaude()

	if !InsideClaude {
		t.Error("InsideClaude should be true when CLAUDECODE was set")
	}
	if os.Getenv("CLAUDECODE") != "" {
		t.Error("CLAUDECODE should be unset after DetectAndCleanClaude")
	}
	// Other CLAUDE_CODE_* vars MUST be preserved.
	if os.Getenv("CLAUDE_CODE_ENTRYPOINT") != "cli" {
		t.Error("CLAUDE_CODE_ENTRYPOINT should be preserved")
	}
	if os.Getenv("CLAUDE_CODE_USE_BEDROCK") != "1" {
		t.Error("CLAUDE_CODE_USE_BEDROCK should be preserved (Bedrock users need this)")
	}
}

func TestDetectAndCleanClaude_OutsideSession(t *testing.T) {
	// Simulate running standalone (not inside Claude Code).
	t.Setenv("CLAUDECODE", "")
	_ = os.Unsetenv("CLAUDECODE")

	InsideClaude = false // reset
	DetectAndCleanClaude()

	if InsideClaude {
		t.Error("InsideClaude should be false when CLAUDECODE was not set")
	}
}

func TestDetectAndCleanClaude_PreservesAllRoutingVars(t *testing.T) {
	// All CLAUDE_CODE_* routing/config vars must survive DetectAndCleanClaude.
	t.Setenv("CLAUDECODE", "1")
	routingVars := map[string]string{
		"CLAUDE_CODE_USE_BEDROCK":                  "1",
		"CLAUDE_CODE_USE_VERTEX":                   "1",
		"CLAUDE_CODE_USE_FOUNDRY":                  "1",
		"CLAUDE_CODE_SKIP_BEDROCK_AUTH":            "1",
		"CLAUDE_CODE_SKIP_VERTEX_AUTH":             "1",
		"CLAUDE_CODE_SKIP_FOUNDRY_AUTH":            "1",
		"CLAUDE_CODE_MODEL":                        "claude-sonnet-4-6",
		"CLAUDE_CODE_MAX_OUTPUT_TOKENS":            "4096",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_ENTRYPOINT":                   "cli",
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS":     "1",
	}
	for k, v := range routingVars {
		t.Setenv(k, v)
	}

	DetectAndCleanClaude()

	if os.Getenv("CLAUDECODE") != "" {
		t.Error("CLAUDECODE should be unset")
	}
	for k, v := range routingVars {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q (must be preserved)", k, got, v)
		}
	}
}

func TestDetectAndCleanClaude_SubprocessInheritsCleanEnv(t *testing.T) {
	// After DetectAndCleanClaude, subprocesses should not see CLAUDECODE
	// but should see other CLAUDE_CODE_* vars.
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")

	DetectAndCleanClaude()

	r := &SubprocessRunner{}
	result, err := r.Run(context.Background(),
		"sh", []string{"-c", `echo "CC=${CLAUDECODE:-ABSENT} BR=${CLAUDE_CODE_USE_BEDROCK:-ABSENT}"`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(string(result.Stdout))
	if got != "CC=ABSENT BR=1" {
		t.Errorf("stdout = %q, want %q", got, "CC=ABSENT BR=1")
	}
}
