package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"sync"
	"syscall"
	"time"
)

// outputDrainTimeout bounds how long Run waits for a finished subprocess's
// output to be read. Normally instant; it only bites when a descendant that
// escaped the process group is still holding the write end.
const outputDrainTimeout = 2 * time.Second

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

	// A call whose budget is already spent is doomed before it starts, so do not
	// pay for a fork/exec to kill it milliseconds later. Per-invocation timeouts
	// (issue #20) make expired contexts reaching here far more common than the
	// single shared deadline did.
	if err := ctx.Err(); err != nil {
		slog.Debug("subprocess skipped, budget already spent", "binary", binary, "error", err)
		return &SubprocessResult{ExitCode: -1}, fmt.Errorf("subprocess %q: %w", binary, err)
	}

	cmd := exec.Command(binary, args...)
	cmd.Env = cfg.env
	if cfg.dir != "" {
		cmd.Dir = cfg.dir
	}
	// Setpgid = true: child becomes its own process group leader (pgid == child.Pid).
	// This lets us kill the entire group with syscall.Kill(-pgid, sig).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Own the output pipes rather than handing os/exec an io.Writer.
	//
	// With a Writer, exec creates the pipe itself and cmd.Wait() blocks until
	// its copier reaches EOF — which needs EVERY holder of the write end to
	// close it. A grandchild that calls setsid() escapes the process group the
	// reaper signals AND keeps that write end, so Wait never returns and the
	// timeout never fires. Passing *os.File hands the descriptor to the child
	// directly, so Wait waits for the process and nothing else.
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("subprocess pipe %q: %w", binary, err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		return nil, fmt.Errorf("subprocess pipe %q: %w", binary, err)
	}
	cmd.Stdout = outW
	cmd.Stderr = errW

	var stdoutBuf, stderrBuf syncBuffer
	defer func() {
		_ = outR.Close()
		_ = errR.Close()
	}()

	slog.Debug("subprocess starting", "binary", binary, "args", args)
	start := time.Now()
	if err := cmd.Start(); err != nil {
		_ = outR.Close()
		_ = outW.Close()
		_ = errR.Close()
		_ = errW.Close()
		return nil, fmt.Errorf("subprocess start %q: %w", binary, err)
	}
	// The parent's write ends must go, or the readers below never see EOF even
	// when the child and all its descendants have exited.
	_ = outW.Close()
	_ = errW.Close()

	copied := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = io.Copy(&stdoutBuf, outR) }()
		go func() { defer wg.Done(); _, _ = io.Copy(&stderrBuf, errR) }()
		wg.Wait()
		close(copied)
	}()

	// drain waits for the output copiers, but never indefinitely: an escaped
	// grandchild can hold the write end open for as long as it likes, and the
	// caller is owed an answer either way.
	drain := func() {
		select {
		case <-copied:
		case <-time.After(outputDrainTimeout):
			slog.Warn("subprocess output still open after exit; returning what was read",
				"binary", binary, "stdout_len", stdoutBuf.Len(), "stderr_len", stderrBuf.Len())
		}
	}

	// pgid == child.Pid when Setpgid = true (child is process group leader).
	pgid := cmd.Process.Pid

	// Buffered channel (capacity 1) prevents goroutine leak if the ctx.Done() branch
	// returns before the goroutine sends — the send completes without a receiver.
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		drain()
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
			// Bounded. SIGKILL cannot reach a descendant that left the process
			// group, and waiting for one forever would make the timeout this
			// branch exists to enforce unenforceable.
			select {
			case <-waitCh:
			case <-time.After(cfg.gracePeriod):
				slog.Warn("subprocess survived SIGKILL, abandoning it",
					"binary", binary, "pgid", pgid)
			}
		}
		drain()
		elapsed := time.Since(start)
		return &SubprocessResult{
			Stdout:   stdoutBuf.Bytes(),
			Stderr:   stderrBuf.Bytes(),
			ExitCode: -1,
			Duration: elapsed,
		}, fmt.Errorf("subprocess %q: %w", binary, ctx.Err())
	}
}

// syncBuffer is a bytes.Buffer safe for one writer goroutine and a reader that
// may look while the writer is still going — which is exactly what happens when
// a subprocess is abandoned with its output pipe still open.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.buf.Bytes())
}

func (b *syncBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
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
