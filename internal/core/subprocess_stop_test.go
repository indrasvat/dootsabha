package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// StopAll is what a signal to dootsabha runs (GitHub issue #33). These tests
// drive the package-level registry, so each resets it afterwards.

func resetLiveGroups(t *testing.T) {
	t.Cleanup(func() {
		liveGroups.mu.Lock()
		for pgid := range liveGroups.live {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
		liveGroups.stopping = false
		liveGroups.mu.Unlock()
	})
}

// startTracked runs script under Run in the background and returns its process
// group, plus a channel carrying Run's error. script receives a path as $1 and
// must create it once its setup is done, so no signal lands before a trap is set.
func startTracked(t *testing.T, script string) (int, <-chan error) {
	t.Helper()
	ready := filepath.Join(t.TempDir(), "ready")
	done := make(chan error, 1)
	go func() {
		_, err := (&SubprocessRunner{}).Run(context.Background(), "bash", []string{"-c", script, "agent", ready})
		done <- err
	}()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(ready); err != nil {
			continue
		}
		liveGroups.mu.Lock()
		for pgid := range liveGroups.live {
			liveGroups.mu.Unlock()
			return pgid, done
		}
		liveGroups.mu.Unlock()
	}
	t.Fatal("agent never became ready")
	return 0, nil
}

func groupGone(pgid int, within time.Duration) bool {
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
			return true
		}
	}
	return false
}

// The agent's own children must go too: a real `claude -p` holds its MCP
// servers in its group, and those outlived the leader before #33.
func TestStopAllEndsTheWholeGroup(t *testing.T) {
	resetLiveGroups(t)
	pgid, done := startTracked(t, `sleep 300 & touch "$1"; wait`)

	if n := StopAll(2 * time.Second); n != 1 {
		t.Errorf("StopAll signalled %d groups, want 1", n)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after StopAll")
	}
	if !groupGone(pgid, time.Second) {
		t.Errorf("process group %d still has members after StopAll", pgid)
	}
}

// An agent that ignores SIGTERM is killed once the grace runs out, and StopAll
// does not wait longer than that grace.
func TestStopAllKillsAGroupThatIgnoresSIGTERM(t *testing.T) {
	resetLiveGroups(t)
	pgid, _ := startTracked(t, `trap "" TERM; sleep 300 & touch "$1"; wait`)

	start := time.Now()
	StopAll(300 * time.Millisecond)
	if took := time.Since(start); took < 300*time.Millisecond || took > 2*time.Second {
		t.Errorf("StopAll took %s with a 300ms grace; SIGTERM was not ignored, or the grace was not bounded", took)
	}
	if !groupGone(pgid, 2*time.Second) {
		t.Errorf("process group %d survived SIGKILL", pgid)
	}
}

func TestRunRefusesToStartOnceStopping(t *testing.T) {
	resetLiveGroups(t)
	StopAll(0)

	marker := filepath.Join(t.TempDir(), "ran")
	_, err := (&SubprocessRunner{}).Run(context.Background(), "touch", []string{marker})
	if !errors.Is(err, ErrStopping) {
		t.Errorf("Run error = %v, want ErrStopping", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("the agent ran after StopAll")
	}
}

// A call cut short takes its whole group with it: a member that ignores the
// reaper's SIGTERM must not outlive the leader. A call that finished on its own
// leaves what it started alone — an agent may launch a server on purpose.
func TestRunKillsWhatACutShortCallLeavesBehind(t *testing.T) {
	for name, tc := range map[string]struct {
		script  string
		timeout time.Duration
		killed  bool
	}{
		"cut short: leader exits on the reaper's TERM": {`(trap "" TERM; exec sleep 300) & echo $! > "$1"; wait`, 300 * time.Millisecond, true},
		"finished: the agent's background task stays":  {`sleep 300 & echo $! > "$1"; exit 0`, 0, false},
	} {
		t.Run(name, func(t *testing.T) {
			resetLiveGroups(t)
			childFile := filepath.Join(t.TempDir(), "child")
			ctx := context.Background()
			if tc.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.timeout)
				defer cancel()
			}
			_, _ = (&SubprocessRunner{}).Run(ctx, "bash", []string{"-c", tc.script, "agent", childFile},
				WithGracePeriod(time.Second))
			raw, err := os.ReadFile(childFile)
			if err != nil {
				t.Fatalf("agent never recorded its child: %v", err)
			}
			child, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
			alive := syscall.Kill(child, 0) == nil
			_ = syscall.Kill(child, syscall.SIGKILL)
			if alive == tc.killed {
				t.Errorf("child %d alive=%v after the call, want alive=%v", child, alive, !tc.killed)
			}
		})
	}
}
