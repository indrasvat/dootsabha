package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// readEvents parses a progress file, failing on any line that is not one JSON
// object — a torn or interleaved line is exactly what a reader must never see.
func readEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open progress file: %v", err)
	}
	defer func() { _ = f.Close() }()
	var events []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("line %d is not one JSON object: %q (%v)", len(events)+1, sc.Text(), err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return events
}

func newTestLog(t *testing.T) (*progressLog, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.ndjson")
	p, err := openProgressLog(path)
	if err != nil {
		t.Fatalf("openProgressLog: %v", err)
	}
	return p, path
}

func TestProgressCallStatusMatchesTheExitDecision(t *testing.T) {
	interrupted.Store(false)
	p, path := newTestLog(t)
	boom := errors.New("provider broke")
	deadline := fmt.Errorf("invoke grok: %w", context.DeadlineExceeded)

	_ = p.call("dispatch", "codex", core.TimeoutScopeInvocation, func() error { return nil })
	_ = p.call("dispatch", "agy", core.TimeoutScopeInvocation, func() error { return boom })
	_ = p.call("dispatch", "grok", core.TimeoutScopeInvocation, func() error { return deadline })
	_ = p.call("synthesis", "claude", core.TimeoutScopeSession, func() error { return deadline })
	p.finish(4, nil)

	events := readEvents(t, path)
	finished := map[string]map[string]any{}
	started := map[float64]bool{}
	for _, ev := range events {
		switch ev["event"] {
		case "call_started":
			started[ev["call"].(float64)] = true
		case "call_finished":
			if !started[ev["call"].(float64)] {
				t.Errorf("call_finished %v has no call_started before it", ev["call"])
			}
			finished[ev["stage"].(string)+"/"+ev["agent"].(string)] = ev
		}
	}
	want := map[string][2]string{
		"dispatch/codex":   {"ok", ""},
		"dispatch/agy":     {"error", ""},
		"dispatch/grok":    {"timeout", "invocation"},
		"synthesis/claude": {"timeout", "session"},
	}
	for key, w := range want {
		ev, ok := finished[key]
		if !ok {
			t.Errorf("%s: no call_finished", key)
			continue
		}
		if ev["status"] != w[0] {
			t.Errorf("%s: status %v, want %s", key, ev["status"], w[0])
		}
		scope, _ := ev["scope"].(string)
		if scope != w[1] {
			t.Errorf("%s: scope %q, want %q", key, scope, w[1])
		}
		if _, ok := ev["duration_ms"]; !ok {
			t.Errorf("%s: no duration_ms", key)
		}
	}
	last := events[len(events)-1]
	if last["event"] != "run_finished" || last["exit_code"] != float64(4) {
		t.Errorf("last event = %v, want run_finished with exit_code 4", last)
	}
}

// The error is never written: a provider error can quote the response.
func TestProgressNeverWritesErrorText(t *testing.T) {
	interrupted.Store(false)
	p, path := newTestLog(t)
	_ = p.call("author", "codex", core.TimeoutScopeInvocation, func() error {
		return errors.New("SECRET-RESPONSE-FRAGMENT")
	})
	p.finish(3, nil)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SECRET-RESPONSE-FRAGMENT") {
		t.Errorf("progress file leaked error text:\n%s", raw)
	}
}

// A signal closes every call still running as interrupted, then ends the run
// with the signal and the code a shell would see.
func TestProgressFinishOnSignalClosesOpenCalls(t *testing.T) {
	interrupted.Store(false)
	p, path := newTestLog(t)
	release := make(chan struct{})
	var wg sync.WaitGroup
	for _, agent := range []string{"codex", "grok"} {
		wg.Go(func() {
			_ = p.call("dispatch", agent, core.TimeoutScopeInvocation, func() error { <-release; return nil })
		})
	}
	waitForEvents(t, path, 2)

	p.finish(128+int(syscall.SIGTERM), syscall.SIGTERM)
	close(release)
	wg.Wait()

	events := readEvents(t, path)
	interruptedCalls := 0
	for _, ev := range events {
		if ev["event"] == "call_finished" {
			if ev["status"] != "interrupted" {
				t.Errorf("open call closed as %v, want interrupted", ev["status"])
			}
			interruptedCalls++
		}
	}
	if interruptedCalls != 2 {
		t.Errorf("%d calls closed, want 2", interruptedCalls)
	}
	last := events[len(events)-1]
	if last["event"] != "run_finished" || last["exit_code"] != float64(143) || last["signal"] != "SIGTERM" {
		t.Errorf("last event = %v, want run_finished exit_code 143 signal SIGTERM", last)
	}
}

// A call that ends after the signal arrived but before finish ran was killed on
// purpose; reporting it as a provider error would send the reader looking for a
// broken agent.
func TestProgressCallEndingAfterASignalIsInterrupted(t *testing.T) {
	p, path := newTestLog(t)
	interrupted.Store(true)
	defer interrupted.Store(false)
	_ = p.call("dispatch", "claude", core.TimeoutScopeInvocation, func() error { return errors.New("signal: killed") })
	p.finish(143, syscall.SIGTERM)
	for _, ev := range readEvents(t, path) {
		if ev["event"] == "call_finished" && ev["status"] != "interrupted" {
			t.Errorf("status %v, want interrupted", ev["status"])
		}
	}
}

func TestProgressNothingAfterRunFinished(t *testing.T) {
	interrupted.Store(false)
	p, path := newTestLog(t)
	p.finish(0, nil)
	_ = p.call("dispatch", "codex", core.TimeoutScopeInvocation, func() error { return nil })
	p.finish(1, nil)
	events := readEvents(t, path)
	if len(events) != 1 || events[0]["event"] != "run_finished" || events[0]["exit_code"] != float64(0) {
		t.Errorf("events after run_finished: %v", events)
	}
}

// Parallel dispatch writes from several goroutines at once. Every line must
// still be one whole object.
func TestProgressConcurrentCallsNeverTearALine(t *testing.T) {
	interrupted.Store(false)
	p, path := newTestLog(t)
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			_ = p.call("dispatch", fmt.Sprintf("agent-%d", i), core.TimeoutScopeInvocation, func() error { return nil })
		})
	}
	wg.Wait()
	p.finish(0, nil)
	if n := len(readEvents(t, path)); n != 129 {
		t.Errorf("%d events, want 129 (64 started + 64 finished + run_finished)", n)
	}
}

// The run owns its file: a stale run's lines must not precede this one's.
func TestProgressOpenTruncates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.ndjson")
	if err := os.WriteFile(path, []byte(`{"event":"run_finished","exit_code":0}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := openProgressLog(path)
	if err != nil {
		t.Fatal(err)
	}
	p.finish(5, nil)
	events := readEvents(t, path)
	if len(events) != 1 || events[0]["exit_code"] != float64(5) {
		t.Errorf("stale lines survived: %v", events)
	}
}

func TestProgressOpenRejectsAnUnwritablePath(t *testing.T) {
	_, err := openProgressLog(t.TempDir()) // a directory
	if err == nil {
		t.Fatal("opening a directory as the progress file succeeded")
	}
}

func TestProgressNilLogStillRunsTheCall(t *testing.T) {
	var p *progressLog
	ran := false
	if err := p.call("dispatch", "codex", "", func() error { ran = true; return nil }); err != nil || !ran {
		t.Errorf("nil log: ran=%v err=%v", ran, err)
	}
	p.finish(0, nil) // must not panic
}

// An agent that runs dootsabha itself must not inherit the variable and
// truncate its parent's file.
func TestTakeProgressEnvUnsetsTheVariable(t *testing.T) {
	t.Setenv(progressEnv, "/tmp/parent.ndjson")
	if got := takeProgressEnv(); got != "/tmp/parent.ndjson" {
		t.Errorf("takeProgressEnv() = %q", got)
	}
	if v, set := os.LookupEnv(progressEnv); set {
		t.Errorf("%s still set to %q — children would inherit it", progressEnv, v)
	}
}

func TestProgressFlagOnEveryRunCommand(t *testing.T) {
	for _, name := range []string{"consult", "council", "review", "refine"} {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if cmd.Flags().Lookup("progress-file") == nil {
			t.Errorf("%s has no --progress-file", name)
		}
		alias := cmd.Flags().Lookup("pragati-patra")
		if alias == nil {
			t.Errorf("%s has no --pragati-patra alias", name)
		} else if !alias.Hidden {
			t.Errorf("%s: --pragati-patra should be hidden", name)
		}
	}
}

func waitForEvents(t *testing.T, path string, n int) {
	t.Helper()
	for range 500 {
		raw, _ := os.ReadFile(path)
		lines := 0
		for _, b := range raw {
			if b == '\n' {
				lines++
			}
		}
		if lines >= n {
			return
		}
		<-time.After(10 * time.Millisecond)
	}
	t.Fatalf("progress file never reached %d lines", n)
}
