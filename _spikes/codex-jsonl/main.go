// Spike: Codex JSONL event stream parsing
// Validates extraction of agent_message content and turn.completed token usage.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ── JSONL event types ──────────────────────────────────────────────────────

// Event is the top-level JSONL line from `codex --json`.
type Event struct {
	Type string `json:"type"`

	// thread.started
	ThreadID string `json:"thread_id,omitempty"`

	// item.completed
	Item *Item `json:"item,omitempty"`

	// turn.completed
	Usage *Usage `json:"usage,omitempty"`
}

// Item is nested inside item.completed events.
type Item struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "reasoning" | "agent_message" | "tool_call" | etc.
	Text string `json:"text,omitempty"`
}

// Usage is nested inside turn.completed events.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ── Parse result ───────────────────────────────────────────────────────────

type ParseResult struct {
	ThreadID     string
	AgentMessage string
	Usage        *Usage
	EventTypes   []string // all observed event types, in order
	Errors       []string // non-fatal parse errors
}

// parseJSONL reads a JSONL byte stream and extracts structured data.
func parseJSONL(data []byte) ParseResult {
	var result ParseResult
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("malformed JSON: %v | line: %q", err, line))
			continue
		}
		result.EventTypes = append(result.EventTypes, ev.Type)
		switch ev.Type {
		case "thread.started":
			result.ThreadID = ev.ThreadID
		case "item.completed":
			if ev.Item != nil && ev.Item.Type == "agent_message" {
				result.AgentMessage = ev.Item.Text
			}
		case "turn.completed":
			result.Usage = ev.Usage
		}
	}
	if err := scanner.Err(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("scanner error: %v", err))
	}
	return result
}

// ── Mock data ──────────────────────────────────────────────────────────────

// sampleJSONL mimics real Codex CLI output (§4.1 verified format).
const sampleJSONL = `{"type":"thread.started","thread_id":"thread_abc123"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"The user wants me to say PONG."}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"PONG"}}
{"type":"turn.completed","usage":{"input_tokens":12,"output_tokens":4}}
`

// edgeCaseJSONL tests resilience: empty stream, no agent_message, malformed line.
const edgeCaseJSONL = `{"type":"thread.started","thread_id":"thread_edge"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"hmm"}}
{"type":"turn.completed","usage":{"input_tokens":5,"output_tokens":0}}
`

const malformedJSONL = `{"type":"thread.started","thread_id":"thread_broken"}
not-valid-json
{"type":"turn.completed","usage":{"input_tokens":0,"output_tokens":0}}
`

// ── Main ───────────────────────────────────────────────────────────────────

func main() {
	fmt.Println("=== Codex JSONL Spike ===")
	fmt.Println()

	// ── 1. Mock data tests ─────────────────────────────────────────────────
	fmt.Println("--- Test 1: Sample JSONL (§4.1 format) ---")
	r1 := parseJSONL([]byte(sampleJSONL))
	printResult(r1)

	fmt.Println("\n--- Test 2: No agent_message ---")
	r2 := parseJSONL([]byte(edgeCaseJSONL))
	printResult(r2)
	if r2.AgentMessage == "" {
		fmt.Println("  ✓ Handled: no agent_message returns empty string")
	}

	fmt.Println("\n--- Test 3: Malformed JSONL line ---")
	r3 := parseJSONL([]byte(malformedJSONL))
	printResult(r3)
	if len(r3.Errors) > 0 {
		fmt.Println("  ✓ Handled: malformed line recorded in Errors, parsing continues")
	}

	fmt.Println("\n--- Test 4: Empty stream ---")
	r4 := parseJSONL([]byte(""))
	printResult(r4)
	if r4.AgentMessage == "" && r4.Usage == nil {
		fmt.Println("  ✓ Handled: empty stream returns zero-value result")
	}

	// ── 2. Real CLI test ───────────────────────────────────────────────────
	fmt.Println("\n--- Test 5: Real Codex CLI (`codex exec --json 'Say PONG'`) ---")
	realOutput, err := runCodexCLI("Say PONG")
	if err != nil {
		fmt.Printf("  CLI error: %v\n", err)
		fmt.Println("  (Proceeding with mock data only — see README for auth requirements)")
	} else {
		fmt.Printf("  Raw output (%d bytes):\n", len(realOutput))
		printLines(realOutput, 30)
		fmt.Println()
		r5 := parseJSONL(realOutput)
		printResult(r5)
	}

	fmt.Println("\n=== Spike complete ===")
}

func runCodexCLI(prompt string) ([]byte, error) {
	// --sandbox danger-full-access + --skip-git-repo-check per task file §Step 4
	cmd := exec.Command("codex", "exec", "--json",
		"--sandbox", "danger-full-access",
		"--skip-git-repo-check",
		prompt,
	)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func printResult(r ParseResult) {
	fmt.Printf("  ThreadID:     %q\n", r.ThreadID)
	fmt.Printf("  AgentMessage: %q\n", r.AgentMessage)
	if r.Usage != nil {
		fmt.Printf("  Usage:        in=%d out=%d\n", r.Usage.InputTokens, r.Usage.OutputTokens)
	} else {
		fmt.Println("  Usage:        <nil>")
	}
	fmt.Printf("  EventTypes:   %v\n", r.EventTypes)
	if len(r.Errors) > 0 {
		fmt.Printf("  Errors:       %v\n", r.Errors)
	}
}

func printLines(data []byte, maxLines int) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	n := 0
	for scanner.Scan() && n < maxLines {
		fmt.Printf("    %s\n", scanner.Text())
		n++
	}
	if scanner.Scan() {
		fmt.Println("    ... (truncated)")
	}
}
