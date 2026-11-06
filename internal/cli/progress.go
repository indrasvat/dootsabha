package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
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

// The run's progress file has two owners, the command and whatever ends the
// run — exit or the signal handler, which runs on its own goroutine and can
// arrive before the command has opened the file, or while it does. So the
// target is resolved once, before any handler is installed, and progressMu
// decides who opens it: the command, at the start of RunE, or the end of the
// run, if the command never got that far. Once the run has ended the lock is
// never released, so nothing opens the file after its run_finished.
var (
	progressMu sync.Mutex
	// progress is this run's log, or nil. Set only by the command, under
	// progressMu, before it starts any agent.
	progress *progressLog
	target   progressTarget
)

// progressTarget is where this run reports, and what it must not overwrite.
type progressTarget struct {
	path      string // "" = no file
	source    string // the flag or variable that named it, for error messages
	protected map[string]os.FileInfo
}

// resolveProgress decides the progress file before anything runs. The flags
// have not been parsed yet — and when Cobra rejects the command line they never
// are — so the path is read from argv, the way Execute already finds --json.
// A flag that is given wins over the environment even when empty, so a caller
// can opt out of an inherited DOOTSABHA_PROGRESS_FILE with --progress-file "".
//
// The variable is unset whatever the command, so an agent that itself runs
// dootsabha does not inherit it and truncate this run's file.
func resolveProgress(args []string) progressTarget {
	env := os.Getenv(progressEnv)
	_ = os.Unsetenv(progressEnv)
	cmd, _, err := rootCmd.Find(args)
	if err != nil || cmd.Flags().Lookup("progress-file") == nil {
		return progressTarget{}
	}
	t := progressTarget{path: env, source: progressEnv}
	if path, name, given := flagFromArgs(args, "--progress-file", "--pragati-patra"); given {
		t.path, t.source = path, name
	}
	if t.path == "" {
		return progressTarget{}
	}
	t.protected = map[string]os.FileInfo{}
	for name, f := range map[string]*os.File{"stdout": os.Stdout, "stderr": os.Stderr} {
		if info, err := f.Stat(); err == nil {
			t.protected[name] = info
		}
	}
	if cfg, _, given := flagFromArgs(args, "--config"); given {
		if info, err := os.Stat(cfg); err == nil {
			t.protected["the --config file"] = info
		}
	}
	return t
}

// flagFromArgs finds the last of names in argv, as "--flag value" or
// "--flag=value" — the last one wins, as it does for pflag.
func flagFromArgs(args []string, names ...string) (value, name string, given bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		for _, n := range names {
			switch {
			case a == n && i+1 < len(args):
				value, name, given = args[i+1], n, true
				i++
			case strings.HasPrefix(a, n+"="):
				value, name, given = strings.TrimPrefix(a, n+"="), n, true
			}
		}
	}
	return value, name, given
}

// startProgress opens the run's file at the start of RunE. A path that cannot
// be opened is a bad command line, reported before any agent runs.
func startProgress(cmd *cobra.Command) error {
	if f := cmd.Flags(); f.Changed("progress-file") && f.Changed("pragati-patra") {
		return &ExitError{Code: core.ExitUsage, Message: "--progress-file and --pragati-patra are the same flag; give one"}
	}
	progressMu.Lock()
	defer progressMu.Unlock()
	if target.path == "" {
		return nil
	}
	p, err := openProgressLog(target.path, target.protected)
	if err != nil {
		target.path = "" // reported here; the end of the run must not retry it
		return &ExitError{Code: core.ExitUsage, Message: fmt.Sprintf("%s (from %s)", err, target.source)}
	}
	progress = p
	return nil
}

// endProgress writes run_finished and holds progressMu until the process
// ends. A run that ended before RunE opened its file — Cobra rejected the
// command line, or a signal came first — opens it here, so the file says how
// this run ended instead of still showing the previous one.
func endProgress(code int, sig os.Signal) {
	progressMu.Lock()
	p := progress
	if p == nil && target.path != "" {
		p, _ = openProgressLog(target.path, target.protected) // RunE never tried; nothing to report it to
	}
	p.finish(code, sig)
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
	size   int64 // bytes of whole lines written
	done   bool
	broken bool
}

type openCall struct {
	stage, agent string
	start        time.Time
}

// openProgressLog opens path for this run alone and empties it. It refuses,
// without touching the file:
//
//   - anything but a regular file — opening a FIFO for writing blocks until a
//     reader appears, before any budget exists to bound the wait;
//   - a file in protected (stdout, stderr, the config): emptying it would
//     destroy the result or the settings the run is about to read;
//   - a file another run holds: two runs in one file let the first's
//     run_finished tell a reader the second is done.
func openProgressLog(path string, protected map[string]os.FileInfo) (*progressLog, error) {
	if info, err := os.Stat(path); err == nil {
		if err := checkProgressTarget(info, protected); err != nil {
			return nil, fmt.Errorf("progress file %q: %w", path, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("progress file: %w", err)
	}
	// O_NONBLOCK: if a FIFO replaced the file since the check, open fails with
	// ENXIO instead of waiting. It has no effect on a regular file.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND|syscall.O_NONBLOCK, 0o644)
	if err != nil {
		return nil, fmt.Errorf("progress file: %w", err)
	}
	if err := claimProgressFile(f, protected); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("progress file %q: %w", path, err)
	}
	return &progressLog{f: f, start: time.Now(), open: map[int]openCall{}}, nil
}

// claimProgressFile re-checks the file actually opened, locks it for the life
// of the process (released when it exits), and only then empties it.
func claimProgressFile(f *os.File, protected map[string]os.FileInfo) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := checkProgressTarget(info, protected); err != nil {
		return err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return errors.New("in use by another dootsabha run — give each run its own file")
		}
		return fmt.Errorf("lock: %w", err)
	}
	return f.Truncate(0)
}

func checkProgressTarget(info os.FileInfo, protected map[string]os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file (%s)", info.Mode().Type())
	}
	for name, p := range protected {
		if os.SameFile(info, p) {
			return fmt.Errorf("is also %s, which it would overwrite", name)
		}
	}
	return nil
}

func addProgressFlag(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("progress-file", "", "Append one JSON line per stage event to this file while the run is going (env "+progressEnv+")")
	f.String("pragati-patra", "", "Alias for --progress-file (प्रगति-पत्र)")
	_ = f.MarkHidden("pragati-patra")
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
	PID        int    `json:"pid"`
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
	p.open[id] = openCall{stage: stage, agent: agent, start: start}
	p.write(callStartedEvent{TS: stamp(start), Event: "call_started", Call: id, Stage: stage, Agent: agent})
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
	ev := runFinishedEvent{TS: stamp(now), Event: "run_finished", PID: os.Getpid(), ExitCode: code, DurationMs: now.Sub(p.start).Milliseconds()}
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
//
// A write the disk could only partly take (full, or over a size limit) is cut
// back to the last whole line, so a reader never parses half an event.
// Callers hold p.mu.
func (p *progressLog) write(ev any) {
	if p.done || p.broken {
		return
	}
	line, err := json.Marshal(ev)
	if err == nil {
		var n int
		n, err = p.f.Write(append(line, '\n'))
		if err == nil {
			p.size += int64(n)
			return
		}
		if n > 0 {
			if terr := p.f.Truncate(p.size); terr != nil {
				err = errors.Join(err, fmt.Errorf("cut back the partial line: %w", terr))
			}
		}
	}
	p.broken = true
	slog.Error("progress file: write failed; no further events will be written", "error", err)
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
	case syscall.SIGPIPE:
		return "SIGPIPE"
	default:
		return fmt.Sprintf("signal %d", int(s))
	}
}
