// Spike: PTY vs Pipe Subprocess Test
//
// Tests whether claude/codex/gemini can be spawned via plain os/exec pipes
// without requiring a PTY (creack/pty). Each CLI is invoked with YOLO+JSON
// flags and the output is parsed to verify no interactive blocking occurs.
//
// Key findings from PRD §4.1:
//   - claude: --output-format json --dangerously-skip-permissions
//     GOTCHA: must unset CLAUDECODE env var (cannot nest inside Claude Code session)
//   - codex: --json --sandbox danger-full-access (JSONL stream)
//   - gemini: --yolo --output-format json OR positional arg
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const timeout = 30 * time.Second

type Result struct {
	CLI        string
	Installed  bool
	Success    bool
	RawOutput  string
	Parsed     any
	Error      string
	Duration   time.Duration
	StderrSnip string
}

func main() {
	fmt.Println("=== PTY vs Pipe Subprocess Spike ===")
	fmt.Println("Testing claude/codex/gemini with YOLO+JSON flags via plain os/exec pipes.")
	fmt.Println()

	results := []Result{
		testClaude(),
		testCodex(),
		testGemini(),
	}

	fmt.Println()
	fmt.Println("=== SUMMARY ===")
	for _, r := range results {
		fmt.Printf("\n[%s]\n", r.CLI)
		if !r.Installed {
			fmt.Printf("  STATUS:    NOT INSTALLED\n")
			continue
		}
		status := "PASS"
		if !r.Success {
			status = "FAIL"
		}
		fmt.Printf("  STATUS:    %s\n", status)
		fmt.Printf("  DURATION:  %v\n", r.Duration.Round(time.Millisecond))
		if r.Error != "" {
			fmt.Printf("  ERROR:     %s\n", r.Error)
		}
		if r.StderrSnip != "" {
			fmt.Printf("  STDERR:    %s\n", r.StderrSnip)
		}
		if r.Parsed != nil {
			out, _ := json.MarshalIndent(r.Parsed, "  ", "  ")
			fmt.Printf("  PARSED:    %s\n", out)
		} else if r.RawOutput != "" {
			raw := r.RawOutput
			if len(raw) > 200 {
				raw = raw[:200] + "...(truncated)"
			}
			fmt.Printf("  RAW OUT:   %s\n", raw)
		}
	}

	// Exit with non-zero if any installed CLI failed
	for _, r := range results {
		if r.Installed && !r.Success {
			os.Exit(1)
		}
	}
}

// testClaude spawns: claude -p "PONG" --output-format json --dangerously-skip-permissions
// Must unset CLAUDECODE to allow nested invocation.
func testClaude() Result {
	r := Result{CLI: "claude"}

	path, err := exec.LookPath("claude")
	if err != nil {
		r.Installed = false
		return r
	}
	r.Installed = true
	fmt.Printf("--- claude (%s) ---\n", path)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path,
		"-p", "Reply with just the word PONG and nothing else",
		"--output-format", "json",
		"--dangerously-skip-permissions",
		"--model", "claude-haiku-4-5-20251001", // fastest/cheapest for spike validation
	)

	// Strip CLAUDECODE* and CLAUDE_CODE* env vars to avoid:
	// "Claude Code cannot be launched inside another Claude Code session"
	// Note: setting CLAUDECODE="" is NOT sufficient — must remove the key entirely.
	// Observed vars in a Claude Code agent session:
	//   CLAUDECODE=1, CLAUDE_CODE_ENTRYPOINT=cli, CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDECODE") && !strings.HasPrefix(e, "CLAUDE_CODE") {
			filtered = append(filtered, e)
		}
	}
	cmd.Env = filtered

	start := time.Now()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	r.Duration = time.Since(start)

	stderrStr := strings.TrimSpace(stderr.String())
	if len(stderrStr) > 150 {
		r.StderrSnip = stderrStr[:150] + "..."
	} else if stderrStr != "" {
		r.StderrSnip = stderrStr
	}

	if runErr != nil && ctx.Err() == context.DeadlineExceeded {
		r.Error = fmt.Sprintf("TIMEOUT after %v — possible interactive blocking", timeout)
		r.RawOutput = stdout.String()
		return r
	}

	raw := stdout.String()
	r.RawOutput = raw
	fmt.Printf("  raw stdout (%d bytes): %s\n", len(raw), truncate(raw, 300))
	if stderrStr != "" {
		fmt.Printf("  stderr: %s\n", truncate(stderrStr, 150))
	}

	if runErr != nil {
		r.Error = runErr.Error()
		// Still try to parse output — some exit codes are non-zero but JSON is valid
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		r.Error = fmt.Sprintf("JSON parse failed: %v (exit: %v)", err, runErr)
		return r
	}
	r.Parsed = parsed
	r.Success = true
	return r
}

// testCodex spawns: codex exec --json --sandbox danger-full-access "PONG"
// Output is JSONL (one JSON object per line). Final message is in
// item.completed where item.type == "agent_message".
func testCodex() Result {
	r := Result{CLI: "codex"}

	path, err := exec.LookPath("codex")
	if err != nil {
		r.Installed = false
		return r
	}
	r.Installed = true
	fmt.Printf("--- codex (%s) ---\n", path)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path,
		"exec",
		"--json",
		"--sandbox", "danger-full-access",
		"Reply with just the word PONG and nothing else",
	)

	start := time.Now()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	r.Duration = time.Since(start)

	stderrStr := strings.TrimSpace(stderr.String())
	if len(stderrStr) > 150 {
		r.StderrSnip = stderrStr[:150] + "..."
	} else if stderrStr != "" {
		r.StderrSnip = stderrStr
	}

	if runErr != nil && ctx.Err() == context.DeadlineExceeded {
		r.Error = fmt.Sprintf("TIMEOUT after %v — possible interactive blocking", timeout)
		r.RawOutput = stdout.String()
		return r
	}

	raw := stdout.String()
	r.RawOutput = raw
	fmt.Printf("  raw stdout (%d bytes):\n%s\n", len(raw), truncate(raw, 500))
	if stderrStr != "" {
		fmt.Printf("  stderr: %s\n", truncate(stderrStr, 150))
	}

	if runErr != nil {
		r.Error = runErr.Error()
	}

	// Parse JSONL — collect all lines, find agent_message
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var allEvents []map[string]any
	var agentMsg string
	var tokenUsage map[string]any

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			r.Error = fmt.Sprintf("JSONL line parse failed: %v on line: %s", err, line)
			return r
		}
		allEvents = append(allEvents, event)

		eventType, _ := event["type"].(string)
		switch eventType {
		case "item.completed":
			if item, ok := event["item"].(map[string]any); ok {
				if item["type"] == "agent_message" {
					agentMsg, _ = item["text"].(string)
				}
			}
		case "turn.completed":
			if usage, ok := event["usage"].(map[string]any); ok {
				tokenUsage = usage
			}
		}
	}
	_ = lines

	if agentMsg == "" && r.Error == "" {
		r.Error = "No agent_message found in JSONL output"
		r.RawOutput = raw
		return r
	}

	r.Parsed = map[string]any{
		"agent_message": agentMsg,
		"event_count":   len(allEvents),
		"token_usage":   tokenUsage,
	}
	r.Success = r.Error == ""
	if !r.Success && agentMsg != "" {
		// Got response but non-zero exit — still a valid finding
		r.Success = true
	}
	return r
}

// testGemini spawns: gemini --yolo --output-format json "PONG"
// Supports both positional arg and -p/--prompt flag.
func testGemini() Result {
	r := Result{CLI: "gemini"}

	path, err := exec.LookPath("gemini")
	if err != nil {
		r.Installed = false
		return r
	}
	r.Installed = true
	fmt.Printf("--- gemini (%s) ---\n", path)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path,
		"--yolo",
		"--output-format", "json",
		"-p", "Reply with just the word PONG and nothing else",
	)

	start := time.Now()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	r.Duration = time.Since(start)

	stderrStr := strings.TrimSpace(stderr.String())
	if len(stderrStr) > 150 {
		r.StderrSnip = stderrStr[:150] + "..."
	} else if stderrStr != "" {
		r.StderrSnip = stderrStr
	}

	if runErr != nil && ctx.Err() == context.DeadlineExceeded {
		r.Error = fmt.Sprintf("TIMEOUT after %v — possible interactive blocking", timeout)
		r.RawOutput = stdout.String()
		return r
	}

	raw := stdout.String()
	r.RawOutput = raw
	fmt.Printf("  raw stdout (%d bytes): %s\n", len(raw), truncate(raw, 300))
	if stderrStr != "" {
		fmt.Printf("  stderr: %s\n", truncate(stderrStr, 150))
	}

	if runErr != nil {
		r.Error = runErr.Error()
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		r.Error = fmt.Sprintf("JSON parse failed: %v (exit: %v)", err, runErr)
		return r
	}
	r.Parsed = parsed
	r.Success = true
	return r
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
