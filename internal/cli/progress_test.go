package cli

import (
	"bufio"
	"bytes"
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
	p, err := openProgressLog(path, nil)
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
	var statuses []any
	for _, ev := range readEvents(t, path) {
		if ev["event"] == "call_finished" {
			statuses = append(statuses, ev["status"])
		}
	}
	if len(statuses) != 1 || statuses[0] != "interrupted" {
		t.Errorf("call_finished statuses %v, want exactly [interrupted]", statuses)
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
	p, err := openProgressLog(path, nil)
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
	_, err := openProgressLog(t.TempDir(), nil) // a directory
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

func TestResolveProgress(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(cfg, []byte("timeout: 7m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, env  string
		args       []string
		wantPath   string
		wantSource string
	}{
		{"flag", "", []string{"consult", "--progress-file", "a.ndjson", "--timeout", "abc"}, "a.ndjson", "--progress-file"},
		{"flag=value", "", []string{"council", "--progress-file=b.ndjson"}, "b.ndjson", "--progress-file"},
		{"alias before the command", "", []string{"--pragati-patra", "c.ndjson", "review"}, "c.ndjson", "--pragati-patra"},
		{"Devanagari command alias", "", []string{"सभा", "--progress-file", "d.ndjson"}, "d.ndjson", "--progress-file"},
		{"env", "e.ndjson", []string{"refine", "x"}, "e.ndjson", progressEnv},
		{"flag beats env", "e.ndjson", []string{"consult", "--progress-file", "f.ndjson"}, "f.ndjson", "--progress-file"},
		{"empty flag opts out of env", "e.ndjson", []string{"consult", "--progress-file", ""}, "", ""},
		{"after -- is a prompt", "", []string{"consult", "--", "--progress-file", "x"}, "", ""},
		{"a command without the flag ignores env", "e.ndjson", []string{"status"}, "", ""},
		// pflag's reading, not a token scan: here --progress-file is the model.
		{"a flag value that looks like the flag", "", []string{"consult", "--model", "--progress-file", "x.ndjson"}, "", ""},
		{"shorthand flags before it", "", []string{"consult", "-a", "claude", "-v", "--progress-file", "g.ndjson", "hi"}, "g.ndjson", "--progress-file"},
		{"--help runs nothing", "", []string{"consult", "--help", "--progress-file", "h.ndjson"}, "", ""},
		{"-h runs nothing, env either", "e.ndjson", []string{"council", "-h"}, "", ""},
		{"--help=false runs the command", "", []string{"consult", "--help=false", "--progress-file", "i.ndjson"}, "i.ndjson", "--progress-file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(progressEnv, tt.env)
			got := resolveProgress(tt.args)
			if got.path != tt.wantPath || got.source != tt.wantSource {
				t.Errorf("resolveProgress(%q) = %q from %q; want %q from %q", tt.args, got.path, got.source, tt.wantPath, tt.wantSource)
			}
			if v, set := os.LookupEnv(progressEnv); set {
				t.Errorf("%s still set to %q — agents would inherit it", progressEnv, v)
			}
		})
	}

	t.Run("the --config file is protected", func(t *testing.T) {
		got := resolveProgress([]string{"consult", "--config", cfg, "--progress-file", cfg})
		info, _ := os.Stat(cfg)
		if p, ok := got.protected["the config file"]; !ok || !os.SameFile(p, info) {
			t.Errorf("protected = %v, want the --config file", got.protected)
		}
	})

	t.Run("the default config is protected without --config", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		def := filepath.Join(home, ".config", "dootsabha", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(def), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(def, []byte("timeout: 7m\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(def)
		// --config "" is no --config: LoadConfig reads the default either way.
		for _, args := range [][]string{
			{"consult", "--progress-file", def},
			{"consult", "--config", "", "--progress-file", def},
		} {
			got := resolveProgress(args)
			if p, ok := got.protected["the config file"]; !ok || !os.SameFile(p, info) {
				t.Errorf("resolveProgress(%q): protected = %v, want the default config", args, got.protected)
			}
		}
	})

	t.Run("both spellings: no target, so neither file is touched", func(t *testing.T) {
		if got := resolveProgress([]string{"consult", "--progress-file", "a", "--pragati-patra", "b"}); got.path != "" {
			t.Errorf("path = %q, want none", got.path)
		}
	})
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
		if bytes.Count(raw, []byte("\n")) >= n {
			return
		}
		<-time.After(10 * time.Millisecond)
	}
	t.Fatalf("progress file never reached %d lines", n)
}

// Opening a FIFO for writing blocks until a reader appears, and no budget
// exists yet to bound it: the run hung before any agent started.
func TestProgressOpenRefusesAFIFOWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := openProgressLog(path, nil); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a FIFO was accepted as the progress file")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("opening a FIFO with no reader blocked")
	}
}

// The progress file is truncated when it opens. Pointing it at the file that
// stdout is redirected to, or at the config, destroyed that file.
func TestProgressOpenRefusesAProtectedFileAndLeavesItAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openProgressLog(path, map[string]os.FileInfo{"stdout": info}); err == nil ||
		!strings.Contains(err.Error(), "stdout") {
		t.Fatalf("err = %v, want a refusal naming stdout", err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "keep me" {
		t.Errorf("protected file was changed to %q", raw)
	}
}

// Two runs on one path: the second truncated the first's head and then the
// first's run_finished told a reader the second was done.
func TestProgressOpenRefusesAPathAnotherRunHolds(t *testing.T) {
	first, path := newTestLog(t)
	first.runStarted(run{command: "consult", agents: []string{"claude"}})
	if _, err := openProgressLog(path, nil); err == nil {
		t.Fatal("a second run opened a progress file the first still holds")
	}
	if events := readEvents(t, path); len(events) != 1 || events[0]["event"] != "run_started" {
		t.Errorf("the first run's file was changed: %v", events)
	}
	first.finish(0, nil)
	again, err := openProgressLog(path, nil)
	if err != nil {
		t.Fatalf("path not reusable once the first run finished: %v", err)
	}
	again.finish(0, nil)
}

// A call that had already run out of time when a signal arrived timed out; the
// signal did not stop it, and saying "interrupted" would hide the timeout.
func TestProgressDeadlineBeatsALaterSignal(t *testing.T) {
	p, path := newTestLog(t)
	interrupted.Store(true)
	defer interrupted.Store(false)
	_ = p.call("dispatch", "grok", core.TimeoutScopeInvocation, func() error {
		return fmt.Errorf("invoke grok: %w", context.DeadlineExceeded)
	})
	p.finish(143, syscall.SIGTERM)
	var finished []map[string]any
	for _, ev := range readEvents(t, path) {
		if ev["event"] == "call_finished" {
			finished = append(finished, ev)
		}
	}
	if len(finished) != 1 || finished[0]["status"] != "timeout" || finished[0]["scope"] != "invocation" {
		t.Errorf("call_finished = %v, want exactly one timeout/invocation", finished)
	}
}
