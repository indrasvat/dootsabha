package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
)

// progressSchemaVersion versions the event contract in
// skill/references/progress-file.md. Adding a field is not a new version;
// renaming or removing one, or changing what a value means, is.
const progressSchemaVersion = 1

// progressEnv names the file when --progress-file is not given, for wrappers
// that cannot add flags.
const progressEnv = "DOOTSABHA_PROGRESS_FILE"

// progressFromEnv is read once at startup; see takeProgressEnv.
var progressFromEnv string

// progress is this run's log, or nil when no file was asked for. It is
// package-level because the run ends in exit() or the signal handler, neither
// of which a command can hand it to.
var progress *progressLog

// takeProgressEnv reads the variable and unsets it, so an agent that itself
// runs dootsabha does not inherit it and truncate this run's file.
func takeProgressEnv() string {
	v := os.Getenv(progressEnv)
	_ = os.Unsetenv(progressEnv)
	return v
}

// progressLog is the sidecar file a caller reads while a run is in flight
// (GitHub issue #34). One JSON object per line, each written with one write(2)
// on an O_APPEND file, so a reader polling it never sees half a line.
//
// It carries names, stages, durations and status — never prompt, response or
// error text, because an error can quote a response.
type progressLog struct {
	mu     sync.Mutex
	f      *os.File
	start  time.Time
	next   int
	open   map[int]openCall
	done   bool
	broken bool
}

type openCall struct {
	stage, agent string
	start        time.Time
}

func openProgressLog(path string) (*progressLog, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("progress file: %w", err)
	}
	return &progressLog{f: f, start: time.Now(), open: map[int]openCall{}}, nil
}

// addProgressFlag registers --progress-file and its alias on a run command.
func addProgressFlag(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("progress-file", "", "Append one JSON line per stage event to this file while the run is going (env "+progressEnv+")")
	f.String("pragati-patra", "", "Alias for --progress-file (प्रगति-पत्र)")
	_ = f.MarkHidden("pragati-patra")
}

// startProgress opens the file the command was asked to write, if any. A path
// that cannot be opened is a bad command line, reported before any agent runs.
func startProgress(cmd *cobra.Command) error {
	path, _ := cmd.Flags().GetString("progress-file")
	if path == "" {
		path, _ = cmd.Flags().GetString("pragati-patra")
	}
	if path == "" {
		path = progressFromEnv
	}
	if path == "" {
		return nil
	}
	p, err := openProgressLog(path)
	if err != nil {
		return &ExitError{Code: core.ExitUsage, Message: err.Error()}
	}
	progress = p
	return nil
}

type runStartedEvent struct {
	TS               string   `json:"ts"`
	Event            string   `json:"event"`
	SchemaVersion    int      `json:"schema_version"`
	Command          string   `json:"command"`
	PID              int      `json:"pid"`
	Agents           []string `json:"agents"`
	Chair            string   `json:"chair,omitempty"`
	Author           string   `json:"author,omitempty"`
	TimeoutMs        int64    `json:"timeout_ms"`
	SessionTimeoutMs int64    `json:"session_timeout_ms"`
}

type callStartedEvent struct {
	TS    string `json:"ts"`
	Event string `json:"event"`
	Call  int    `json:"call"`
	Stage string `json:"stage"`
	Agent string `json:"agent"`
}

type callFinishedEvent struct {
	TS         string `json:"ts"`
	Event      string `json:"event"`
	Call       int    `json:"call"`
	Stage      string `json:"stage"`
	Agent      string `json:"agent"`
	Status     string `json:"status"`
	Scope      string `json:"scope,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

type runFinishedEvent struct {
	TS         string `json:"ts"`
	Event      string `json:"event"`
	ExitCode   int    `json:"exit_code"`
	Signal     string `json:"signal,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// run describes what this run will do. Written once, before the first call.
type run struct {
	command        string
	agents         []string
	chair, author  string
	timeout        time.Duration
	sessionTimeout time.Duration
}

func (p *progressLog) runStarted(r run) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.write(runStartedEvent{
		TS: stamp(time.Now()), Event: "run_started", SchemaVersion: progressSchemaVersion,
		Command: r.command, PID: os.Getpid(), Agents: r.agents, Chair: r.chair, Author: r.author,
		TimeoutMs: r.timeout.Milliseconds(), SessionTimeoutMs: r.sessionTimeout.Milliseconds(),
	})
}

// call runs one provider call between its call_started and call_finished.
// scope is the budget that bounded it, reported only if it timed out. The
// status is decided from the same error the caller records in its ledger.
func (p *progressLog) call(stage, agent, scope string, invoke func() error) error {
	if p == nil {
		return invoke()
	}
	p.mu.Lock()
	p.next++
	id := p.next
	start := time.Now()
	if !p.done {
		p.open[id] = openCall{stage: stage, agent: agent, start: start}
		p.write(callStartedEvent{TS: stamp(start), Event: "call_started", Call: id, Stage: stage, Agent: agent})
	}
	p.mu.Unlock()

	err := invoke()

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, open := p.open[id]; !open {
		return err // finish already closed it
	}
	delete(p.open, id)
	status, scope := callStatus(err, scope)
	now := time.Now()
	p.write(callFinishedEvent{
		TS: stamp(now), Event: "call_finished", Call: id, Stage: stage, Agent: agent,
		Status: status, Scope: scope, DurationMs: now.Sub(start).Milliseconds(),
	})
	return err
}

// callStatus classifies a call's error the way Outcome.Exit does: a deadline is
// a timeout, anything else a failure. A call that ended after a terminating
// signal was stopped on purpose, and saying so keeps a reader from hunting for
// a broken agent.
func callStatus(err error, scope string) (status, timeoutScope string) {
	switch {
	case err == nil:
		return "ok", ""
	case interrupted.Load():
		return "interrupted", ""
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout", scope
	default:
		return "error", ""
	}
}

// finish closes every call still running as interrupted and writes
// run_finished. It runs once, from exit or the signal handler, after the agents
// are stopped; nothing is written after it.
func (p *progressLog) finish(code int, sig os.Signal) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done {
		return
	}
	now := time.Now()
	for id, c := range p.open {
		p.write(callFinishedEvent{
			TS: stamp(now), Event: "call_finished", Call: id, Stage: c.stage, Agent: c.agent,
			Status: "interrupted", DurationMs: now.Sub(c.start).Milliseconds(),
		})
	}
	clear(p.open)
	ev := runFinishedEvent{TS: stamp(now), Event: "run_finished", ExitCode: code, DurationMs: now.Sub(p.start).Milliseconds()}
	if s, ok := sig.(syscall.Signal); ok {
		ev.Signal = signalName(s)
	}
	p.write(ev)
	p.done = true
	_ = p.f.Close()
}

// write appends one event as one line. A failed write is reported once, at
// Error so it shows under --json too, and the run goes on: losing a
// half-hour council to its own progress log would be the worse failure.
// Callers hold p.mu.
func (p *progressLog) write(ev any) {
	if p.done || p.broken {
		return
	}
	line, err := json.Marshal(ev)
	if err == nil {
		_, err = p.f.Write(append(line, '\n'))
	}
	if err != nil {
		p.broken = true
		slog.Error("progress file: write failed; no further events will be written", "error", err)
	}
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func signalName(s syscall.Signal) string {
	switch s {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGHUP:
		return "SIGHUP"
	default:
		return fmt.Sprintf("signal %d", int(s))
	}
}
