package cli

import (
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// agentStopGrace is how long agents get between SIGTERM and SIGKILL when
// dootsabha itself is signalled — the same grace a timed-out call gets.
const agentStopGrace = 5 * time.Second

// interrupted is set the moment a terminating signal arrives. From then on the
// signal handler owns the exit: it stops the agents and re-raises.
var interrupted atomic.Bool

// setupInterrupt makes SIGINT, SIGTERM and SIGHUP stop every agent before
// dootsabha exits (GitHub issue #33). Agents run in their own process groups,
// so none of these signals reaches them on its own.
//
// After cleanup the signal is re-raised with its default action, so the caller
// sees exactly the status it would have before (143, 130, 129): the exit-code
// contract is untouched. A second signal during cleanup ends dootsabha at once.
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
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)
	go func() {
		sig := (<-ch).(syscall.Signal)
		interrupted.Store(true)
		signal.Reset(sigs...)
		n := core.StopAll(agentStopGrace)
		if !quiet {
			fmt.Fprintf(os.Stderr, "dootsabha: %s — stopped %d agent process group(s)\n", sig, n) //nolint:errcheck
		}
		_ = syscall.Kill(os.Getpid(), sig)
		// Unreachable unless the re-raise was somehow blocked; exit with the
		// status the signal would have produced rather than hang.
		os.Exit(128 + int(sig))
	}()
}

// exit ends the process with code, unless a signal is being handled: then the
// handler is about to stop the agents and re-raise, and exiting first would
// both orphan them and report a code the caller never caused.
func exit(code int) {
	if interrupted.Load() {
		select {}
	}
	os.Exit(code)
}
