package cli

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

const (
	// agentStopGrace is how long agents get between SIGTERM and SIGKILL when
	// dootsabha stops them — the same grace a timed-out call gets.
	agentStopGrace = 5 * time.Second
	// noticeWait bounds the stderr notice: a reader that stopped reading must
	// not keep dootsabha alive after its agents are gone.
	noticeWait = 250 * time.Millisecond
)

var (
	// hurry is closed when the first trapped signal arrives. An exit already
	// stopping agents then skips the grace instead of making the caller wait.
	hurry     = make(chan struct{})
	hurryOnce sync.Once
	// shutdown runs once per process, whichever path gets there first: a
	// signal, a normal or error exit, or SIGPIPE. Each stops every agent
	// before the process ends, and a second caller blocks until it does.
	shutdown sync.Once
	// interrupted is set when a terminating signal arrives, so the command's
	// own failure report — about agents that were killed on purpose — is not
	// printed as though something went wrong.
	interrupted atomic.Bool
)

// setupInterrupt makes SIGINT, SIGTERM and SIGHUP stop every agent before
// dootsabha exits (GitHub issue #33): agents run in their own process groups,
// out of reach of a signal sent to dootsabha.
//
// After cleanup the signal is re-raised with its default action, so the caller
// sees the same death it always did and the exit-code contract is untouched. A
// second signal skips the grace and kills the agents at once.
//
// A signal ignored at startup — SIGINT for a shell's background job, SIGHUP
// under nohup — stays ignored: notifying on it would un-ignore it.
func setupInterrupt() {
	var sigs []os.Signal
	for _, s := range []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP} {
		if !signal.Ignored(s) {
			sigs = append(sigs, s)
		}
	}
	if len(sigs) == 0 {
		return
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, sigs...)
	go func() {
		sig := (<-signals).(syscall.Signal)
		interrupted.Store(true)
		hurryOnce.Do(func() { close(hurry) })
		shutdown.Do(func() {
			again := make(chan struct{})
			go func() { <-signals; close(again) }()
			n := core.StopAll(agentStopGrace, again)
			if !quiet {
				notice(fmt.Sprintf("dootsabha: %s — stopped %d agent process group(s)\n", sig, n))
			}
			signal.Reset(sigs...)
			_ = syscall.Kill(os.Getpid(), sig)
			// A signal sent to the own pid can land on another thread while this
			// one runs on; give it time to arrive rather than racing it.
			time.Sleep(time.Second)
			os.Exit(128 + int(sig))
		})
	}()
}

// notice writes msg to stderr without waiting on a reader that has stopped.
func notice(msg string) {
	done := make(chan struct{})
	go func() {
		fmt.Fprint(os.Stderr, msg) //nolint:errcheck
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(noticeWait):
	}
}

// exit stops any agent still running, then ends the process with code. If a
// signal got to shutdown first, it blocks until that handler ends the process.
func exit(code int) {
	shutdown.Do(func() {
		core.StopAll(agentStopGrace, hurry)
		os.Exit(code)
	})
}
