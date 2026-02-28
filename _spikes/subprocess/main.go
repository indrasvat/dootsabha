// Spike 004: Subprocess Management
//
// Validates: errgroup fan-out/fan-in, context cancellation propagation,
// process group cleanup (Setpgid + SIGTERM/SIGKILL), orphan reaper behavior,
// and macOS SIP interaction with process groups.
//
// Mimics dootsabha's Subprocess Runner (§5.2) and Ctrl+C behavior (§7.2).

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

const gracePeriod = 5 * time.Second

// killPgrp sends sig to the process group with id pgid.
// Negative pid in Kill() targets the entire process group.
func killPgrp(pgid int, sig syscall.Signal) {
	if err := syscall.Kill(-pgid, sig); err != nil {
		fmt.Printf("  kill(pgid=%d, sig=%v): %v\n", pgid, sig, err)
	}
}

// runAgent spawns args[0] with args[1:] as an isolated process group.
// It respects ctx cancellation: SIGTERM → gracePeriod → SIGKILL.
// NOTE: we use exec.Command (not exec.CommandContext) deliberately — we want to
// control the kill sequence ourselves instead of getting an immediate SIGKILL.
func runAgent(ctx context.Context, name string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec // spike code
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Setpgid=true: child becomes leader of a new process group.
	// This lets us kill the whole subtree (child + grandchildren) via -pgid.
	// On macOS with SIP enabled this still works for processes we spawn.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	fmt.Printf("[%s] starting: %v\n", name, args)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[%s] start: %w", name, err)
	}

	// After Setpgid=true, the child's pgid == its own pid (it is the group leader).
	pgid := cmd.Process.Pid
	fmt.Printf("[%s] pid=%d pgid=%d\n", name, cmd.Process.Pid, pgid)

	// Run Wait in a goroutine so we can select on ctx.Done() at the same time.
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		// Process exited before ctx was cancelled.
		if err != nil {
			return fmt.Errorf("[%s] exit error: %w", name, err)
		}
		fmt.Printf("[%s] completed successfully\n", name)
		return nil

	case <-ctx.Done():
		// Context cancelled (Ctrl+C or sibling failure).
		// Step 1: SIGTERM to entire process group.
		fmt.Printf("[%s] ctx cancelled — SIGTERM → pgid %d\n", name, pgid)
		killPgrp(pgid, syscall.SIGTERM)

		// Step 2: grace period — wait up to 5s for clean exit.
		select {
		case <-waitCh:
			fmt.Printf("[%s] exited cleanly after SIGTERM\n", name)
		case <-time.After(gracePeriod):
			// Step 3: SIGKILL if still alive after grace period.
			fmt.Printf("[%s] grace expired — SIGKILL → pgid %d\n", name, pgid)
			killPgrp(pgid, syscall.SIGKILL)
			<-waitCh // drain to avoid goroutine leak
			fmt.Printf("[%s] killed\n", name)
		}

		return ctx.Err()
	}
}

func main() {
	// Catch SIGINT (Ctrl+C) and SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()

	// Cancel root context on signal — this propagates to all runAgent calls.
	go func() {
		sig := <-sigCh
		fmt.Printf("\n[main] signal %v → initiating clean shutdown\n", sig)
		cancelRoot()
	}()

	scenario := os.Getenv("SCENARIO")
	if scenario == "" {
		scenario = "normal"
	}
	fmt.Printf("=== Subprocess Spike — scenario: %q ===\n\n", scenario)

	switch scenario {
	case "normal":
		runNormal(rootCtx)
	case "one-fails":
		runOneFails(rootCtx)
	case "long":
		runLong(rootCtx)
	default:
		fmt.Fprintf(os.Stderr, "unknown SCENARIO=%q — choices: normal, one-fails, long\n", scenario)
		os.Exit(2)
	}
}

// runNormal: all 3 agents finish successfully.
func runNormal(ctx context.Context) {
	fmt.Println("Scenario: all 3 succeed (sleep 1s, 2s, 3s)")

	type job struct {
		name string
		args []string // full command: args[0] is the binary
	}
	jobs := []job{
		{"claude", []string{"sleep", "1"}},
		{"codex", []string{"sleep", "2"}},
		{"gemini", []string{"sleep", "3"}},
	}

	g, gCtx := errgroup.WithContext(ctx)
	start := time.Now()
	for _, j := range jobs {
		jj := j
		g.Go(func() error {
			return runAgent(gCtx, jj.name, jj.args...)
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("\n[main] error: %v (elapsed %v)\n", err, time.Since(start))
		os.Exit(1)
	}
	fmt.Printf("\n[main] all succeeded (elapsed %v)\n", time.Since(start))
}

// runOneFails: codex fails → errgroup cancels context → claude+gemini killed.
func runOneFails(ctx context.Context) {
	fmt.Println("Scenario: codex fails after 1s → claude+gemini (10s) are killed")

	g, gCtx := errgroup.WithContext(ctx)
	start := time.Now()

	g.Go(func() error { return runAgent(gCtx, "claude", "sleep", "10") })
	g.Go(func() error { return runAgent(gCtx, "codex", "bash", "-c", "sleep 1; exit 1") })
	g.Go(func() error { return runAgent(gCtx, "gemini", "sleep", "10") })

	if err := g.Wait(); err != nil {
		fmt.Printf("\n[main] errgroup error (expected): %v (elapsed %v)\n", err, time.Since(start))
		fmt.Println("[main] verify: run `ps aux | grep 'sleep 10'` — should be empty")
		return
	}
	fmt.Printf("\n[main] done (elapsed %v)\n", time.Since(start))
}

// runLong: 60s agents — press Ctrl+C to test clean shutdown.
func runLong(ctx context.Context) {
	fmt.Println("Scenario: 3× sleep 60s — press Ctrl+C to test clean shutdown")
	fmt.Println("After Ctrl+C: verify `ps aux | grep 'sleep 60'` is empty")

	g, gCtx := errgroup.WithContext(ctx)
	start := time.Now()

	for _, name := range []string{"claude", "codex", "gemini"} {
		n := name
		g.Go(func() error { return runAgent(gCtx, n, "sleep", "60") })
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("\n[main] clean shutdown complete (elapsed %v)\n", time.Since(start))
		fmt.Println("[main] verify: run `ps aux | grep 'sleep 60'` — should be empty")
		return
	}
	fmt.Printf("\n[main] done (elapsed %v)\n", time.Since(start))
}
