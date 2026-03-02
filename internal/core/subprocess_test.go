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

func TestSanitizeEnvForClaude(t *testing.T) {
	input := []string{
		"HOME=/home/user",
		"CLAUDECODE=1",
		"PATH=/usr/bin:/bin",
		"CLAUDE_CODE_ENTRYPOINT=cli",
		"TERM=xterm-256color",
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1",
		"EDITOR=vim",
	}

	got := SanitizeEnvForClaude(input)

	// Check that CLAUDECODE* and CLAUDE_CODE* are removed.
	for _, e := range got {
		if strings.HasPrefix(e, "CLAUDECODE") || strings.HasPrefix(e, "CLAUDE_CODE") {
			t.Errorf("SanitizeEnvForClaude left banned var in output: %q", e)
		}
	}

	// Check that safe vars are preserved.
	safe := map[string]bool{
		"HOME=/home/user":     true,
		"PATH=/usr/bin:/bin":  true,
		"TERM=xterm-256color": true,
		"EDITOR=vim":          true,
	}
	for _, e := range got {
		if !safe[e] {
			t.Errorf("unexpected entry in sanitized env: %q", e)
		}
		delete(safe, e)
	}
	for missing := range safe {
		t.Errorf("expected entry missing from sanitized env: %q", missing)
	}
}

func TestSanitizeEnvForClaude_EmptyInput(t *testing.T) {
	got := SanitizeEnvForClaude(nil)
	if got == nil {
		t.Error("expected non-nil slice from nil input")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestSanitizeEnvForClaude_AllBanned(t *testing.T) {
	input := []string{
		"CLAUDECODE=1",
		"CLAUDE_CODE_ENTRYPOINT=cli",
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1",
	}
	got := SanitizeEnvForClaude(input)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestSubprocessRunner_SubprocessDoesNotInheritClaudeEnv(t *testing.T) {
	// Verify that when we sanitize the env, the subprocess doesn't see CLAUDECODE.
	r := &SubprocessRunner{}
	ctx := context.Background()

	// Build an env with CLAUDECODE present, then sanitize it.
	baseEnv := append(os.Environ(), "CLAUDECODE=1", "CLAUDE_CODE_ENTRYPOINT=cli")
	cleanEnv := SanitizeEnvForClaude(baseEnv)

	result, err := r.Run(ctx,
		"sh", []string{"-c", `echo "CLAUDECODE=${CLAUDECODE:-ABSENT}"`},
		WithEnv(cleanEnv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(string(result.Stdout))
	if got != "CLAUDECODE=ABSENT" {
		t.Errorf("stdout = %q, want CLAUDECODE=ABSENT (var should be absent from env)", got)
	}
}
