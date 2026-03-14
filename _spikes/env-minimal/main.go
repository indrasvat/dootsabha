// Spike: Validate that unsetting ONLY CLAUDECODE is sufficient to allow
// `claude -p` subprocess invocation from inside a Claude Code session,
// while preserving ALL other CLAUDE_CODE_* env vars.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("=== Spike: Minimal env var fix for nested Claude session ===")
	fmt.Println()

	// ── Check 1: Are we inside a Claude Code session? ──────────────────
	claudeCode := os.Getenv("CLAUDECODE")
	fmt.Printf("CLAUDECODE = %q (inside Claude Code: %v)\n", claudeCode, claudeCode != "")

	// Show all CLAUDE* env vars currently set
	fmt.Println("\nAll CLAUDE* env vars in current process:")
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDE") {
			fmt.Printf("  %s\n", e)
		}
	}

	// ── Check 2: Unset ONLY CLAUDECODE ─────────────────────────────────
	fmt.Println("\n--- Unsetting ONLY 'CLAUDECODE' ---")
	os.Unsetenv("CLAUDECODE")

	// Verify it's gone
	fmt.Printf("CLAUDECODE after unset = %q (should be empty)\n", os.Getenv("CLAUDECODE"))

	// Verify other CLAUDE_CODE_* vars are still present
	fmt.Println("\nRemaining CLAUDE* env vars after unset:")
	remaining := 0
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDE") {
			fmt.Printf("  %s\n", e)
			remaining++
		}
	}
	if remaining == 0 {
		fmt.Println("  (none)")
	}

	// ── Check 3: Can we invoke `claude -p` now? ────────────────────────
	fmt.Println("\n--- Test: invoke `claude -p` with CLAUDECODE unset ---")
	cmd := exec.Command("claude", "-p", "Say exactly: SPIKE_OK", "--output-format", "json", "--dangerously-skip-permissions", "--no-session-persistence", "--model", "claude-haiku-4-5-20251001")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Inherit current env (CLAUDECODE is already unset from this process)

	err := cmd.Run()
	fmt.Printf("Exit code: %d\n", cmd.ProcessState.ExitCode())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	if stderr.Len() > 0 {
		fmt.Printf("Stderr: %s\n", strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() > 0 {
		out := stdout.String()
		if len(out) > 500 {
			out = out[:500] + "..."
		}
		fmt.Printf("Stdout (first 500 chars): %s\n", out)
	}

	// Check for the nested session error
	if strings.Contains(stderr.String(), "cannot be launched inside another") {
		fmt.Println("\n✗ FAIL: Still getting nested session error!")
		os.Exit(1)
	}
	if strings.Contains(stdout.String(), "SPIKE_OK") || strings.Contains(stdout.String(), "is_error") {
		fmt.Println("\n✓ PASS: claude -p works with only CLAUDECODE unset")
	} else if stdout.Len() > 0 {
		fmt.Println("\n✓ PASS: claude -p produced output (no nested session error)")
	} else {
		fmt.Println("\n? INCONCLUSIVE: no stdout, check stderr above")
	}

	// ── Check 4: Verify CLAUDE_CODE_* vars survive to subprocess ───────
	fmt.Println("\n--- Test: verify CLAUDE_CODE_* vars pass through to subprocess ---")
	// Set a test routing var to verify it passes through
	os.Setenv("CLAUDE_CODE_USE_BEDROCK", "test_spike_value")
	cmd2 := exec.Command("sh", "-c", `echo "CLAUDECODE=${CLAUDECODE:-ABSENT}" && echo "CLAUDE_CODE_USE_BEDROCK=${CLAUDE_CODE_USE_BEDROCK:-ABSENT}"`)
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	cmd2.Run()
	fmt.Print(out2.String())

	if strings.Contains(out2.String(), "CLAUDECODE=ABSENT") && strings.Contains(out2.String(), "CLAUDE_CODE_USE_BEDROCK=test_spike_value") {
		fmt.Println("✓ PASS: CLAUDECODE absent, CLAUDE_CODE_USE_BEDROCK preserved")
	} else {
		fmt.Println("✗ FAIL: env vars not as expected")
	}

	fmt.Println("\n=== Spike complete ===")
}
