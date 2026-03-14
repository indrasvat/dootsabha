package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// SubprocessRunner executes CLI binaries as subprocesses with process group isolation.
type SubprocessRunner struct{}

// SubprocessResult holds the output of a completed subprocess invocation.
type SubprocessResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
}

// RunOption configures a subprocess invocation via the functional options pattern.
type RunOption func(*runConfig)

type runConfig struct {
	env         []string
	dir         string
	gracePeriod time.Duration
}

// WithEnv sets the subprocess environment. Overrides the default (os.Environ()).
func WithEnv(env []string) RunOption {
	return func(c *runConfig) {
		c.env = env
	}
}

// WithDir sets the working directory for the subprocess.
func WithDir(dir string) RunOption {
	return func(c *runConfig) {
		c.dir = dir
	}
}

// WithGracePeriod sets the time between SIGTERM and SIGKILL. Default: 5s.
func WithGracePeriod(d time.Duration) RunOption {
	return func(c *runConfig) {
		c.gracePeriod = d
	}
}

// InsideClaude reports whether dootsabha was launched from inside a Claude Code
// session. Set by DetectAndCleanClaude at startup, before any subcommands run.
var InsideClaude bool

// DetectAndCleanClaude checks whether we're inside a Claude Code session and
// unsets the CLAUDECODE env var so subprocesses can invoke `claude -p` without
// hitting the nested session error.
//
// Only CLAUDECODE needs to be unset — it is the sole var Claude CLI checks for
// nested session detection (Spike 0.2, validated by env-minimal spike).
// All other CLAUDE_CODE_* vars (CLAUDE_CODE_USE_BEDROCK, CLAUDE_CODE_USE_VERTEX,
// CLAUDE_CODE_ENTRYPOINT, etc.) are left untouched. This is critical for
// Bedrock/Vertex/Foundry users whose routing depends on these vars (issue #4).
func DetectAndCleanClaude() {
	InsideClaude = os.Getenv("CLAUDECODE") != ""
	_ = os.Unsetenv("CLAUDECODE")
}

// Run executes binary with args, captures stdout/stderr, and enforces context timeout.
//
// Key implementation details (from Spike 0.5 and 0.8):
//   - Uses exec.Command (NOT exec.CommandContext) — CommandContext sends SIGKILL immediately,
//     bypassing the SIGTERM→grace→SIGKILL sequence.
//   - Sets SysProcAttr.Setpgid = true so the child becomes its own process group leader
//     (pgid == child.Pid). This ensures the entire process group is killed on cancellation.
//   - On context cancellation: SIGTERM to -pgid → gracePeriod wait → SIGKILL to -pgid.
//   - Uses a buffered waitCh (capacity 1) to prevent goroutine leak if Run returns early.
func (r *SubprocessRunner) Run(ctx context.Context, binary string, args []string, opts ...RunOption) (*SubprocessResult, error) {
	cfg := &runConfig{
		env:         os.Environ(),
		gracePeriod: 5 * time.Second,
	}
	for _, o := range opts {
		o(cfg)
	}

	cmd := exec.Command(binary, args...)
	cmd.Env = cfg.env
	if cfg.dir != "" {
		cmd.Dir = cfg.dir
	}
	// Setpgid = true: child becomes its own process group leader (pgid == child.Pid).
	// This lets us kill the entire group with syscall.Kill(-pgid, sig).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	slog.Debug("subprocess starting", "binary", binary, "args", args)
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("subprocess start %q: %w", binary, err)
	}

	// pgid == child.Pid when Setpgid = true (child is process group leader).
	pgid := cmd.Process.Pid

	// Buffered channel (capacity 1) prevents goroutine leak if the ctx.Done() branch
	// returns before the goroutine sends — the send completes without a receiver.
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		elapsed := time.Since(start)
		exitCode := exitCodeFromErr(err)
		slog.Debug("subprocess finished", "binary", binary, "exit_code", exitCode,
			"duration", elapsed, "stdout_len", stdoutBuf.Len(), "stderr_len", stderrBuf.Len())
		return &SubprocessResult{
			Stdout:   stdoutBuf.Bytes(),
			Stderr:   stderrBuf.Bytes(),
			ExitCode: exitCode,
			Duration: elapsed,
		}, nil

	case <-ctx.Done():
		// Reaper: send SIGTERM to the entire process group, wait for grace period,
		// then SIGKILL if still alive. Negative pgid targets the whole group.
		slog.Warn("subprocess timed out, sending SIGTERM", "binary", binary, "pgid", pgid)
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		select {
		case <-waitCh:
			// Process exited cleanly within grace period.
		case <-time.After(cfg.gracePeriod):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			<-waitCh // drain to release cmd resources
		}
		elapsed := time.Since(start)
		return &SubprocessResult{
			Stdout:   stdoutBuf.Bytes(),
			Stderr:   stderrBuf.Bytes(),
			ExitCode: -1,
			Duration: elapsed,
		}, fmt.Errorf("subprocess %q: %w", binary, ctx.Err())
	}
}

// exitCodeFromErr extracts the numeric exit code from a cmd.Wait() error.
// Returns 0 for nil (success), the process exit code for *exec.ExitError, or 1 otherwise.
func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
