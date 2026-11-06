package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

// Once dootsabha is stopping, a pipeline's next stage must not start an agent
// that nothing would stop.
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
