// Spike: Claude JSON output parsing
// Validates the exact JSON schema from `claude -p "..." --output-format json`
// and confirms the CLAUDECODE nested session env var gotcha.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// ClaudeResult is the top-level JSON envelope from claude --output-format json.
// All fields verified against claude 2.1.63 real output.
type ClaudeResult struct {
	Type          string         `json:"type"`           // always "result"
	Subtype       string         `json:"subtype"`        // "success" even on error
	IsError       bool           `json:"is_error"`       // true when model/auth error
	DurationMs    int            `json:"duration_ms"`    // wall clock ms
	DurationAPIMs int            `json:"duration_api_ms"` // API call ms (0 on error)
	NumTurns      int            `json:"num_turns"`
	Result        string         `json:"result"`         // response text or error message
	StopReason    *string        `json:"stop_reason"`    // nullable
	SessionID     string         `json:"session_id"`
	TotalCostUSD  float64        `json:"total_cost_usd"` // 0.0 on error
	Usage         ClaudeUsage    `json:"usage"`
	ModelUsage    map[string]ModelUsage `json:"modelUsage"` // keyed by model ID; empty on error
	PermissionDenials []any      `json:"permission_denials"`
	FastModeState string         `json:"fast_mode_state"` // "off" | "on"
	UUID          string         `json:"uuid"`
}

// ClaudeUsage contains token accounting.
type ClaudeUsage struct {
	InputTokens              int              `json:"input_tokens"`
	CacheCreationInputTokens int              `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int              `json:"cache_read_input_tokens"`
	OutputTokens             int              `json:"output_tokens"`
	ServerToolUse            ServerToolUse    `json:"server_tool_use"`
	ServiceTier              string           `json:"service_tier"`
	CacheCreation            CacheCreation    `json:"cache_creation"`
	InferenceGeo             string           `json:"inference_geo"`
	Iterations               []any            `json:"iterations"`
	Speed                    string           `json:"speed"`
}

// ServerToolUse tracks built-in Claude server tool calls.
type ServerToolUse struct {
	WebSearchRequests int `json:"web_search_requests"`
	WebFetchRequests  int `json:"web_fetch_requests"`
}

// CacheCreation breaks down cache token types.
type CacheCreation struct {
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
}

// ModelUsage is per-model cost breakdown inside modelUsage map.
type ModelUsage struct {
	InputTokens              int     `json:"inputTokens"`
	OutputTokens             int     `json:"outputTokens"`
	CacheReadInputTokens     int     `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int     `json:"cacheCreationInputTokens"`
	WebSearchRequests        int     `json:"webSearchRequests"`
	CostUSD                  float64 `json:"costUSD"`
	ContextWindow            int     `json:"contextWindow"`
	MaxOutputTokens          int     `json:"maxOutputTokens"`
}

// runClaude executes claude CLI and returns parsed output.
// env is a list of KEY=VALUE strings appended to the subprocess env.
// Passing an empty env entry for CLAUDECODE is NOT sufficient — callers
// must explicitly remove CLAUDECODE from env to avoid nested session errors.
func runClaude(prompt string, extraArgs []string, inheritEnvWithout string) (*ClaudeResult, []byte, error) {
	args := append([]string{"-p", prompt, "--output-format", "json", "--dangerously-skip-permissions"}, extraArgs...)
	cmd := exec.Command("claude", args...)

	// Build env: start from os.Environ(), remove the key we want gone.
	filtered := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if len(inheritEnvWithout) > 0 {
			key := kv
			for i, c := range kv {
				if c == '=' {
					key = kv[:i]
					break
				}
			}
			if key == inheritEnvWithout {
				continue
			}
		}
		filtered = append(filtered, kv)
	}
	cmd.Env = filtered

	out, err := cmd.Output()
	if err != nil {
		// On error, claude still outputs valid JSON to stdout (exit code 1).
		// Capture the output from the ExitError if available.
		if exitErr, ok := err.(*exec.ExitError); ok {
			_ = exitErr
			// stdout is in `out` even on non-zero exit; stderr has the raw message.
		}
	}

	if len(out) == 0 {
		return nil, out, fmt.Errorf("no output from claude: %w", err)
	}

	var result ClaudeResult
	if parseErr := json.Unmarshal(out, &result); parseErr != nil {
		return nil, out, fmt.Errorf("json parse failed: %w (raw: %s)", parseErr, out)
	}
	return &result, out, nil
}

func main() {
	fmt.Println("=== Spike: Claude JSON Output Parsing ===")
	fmt.Println()

	// ── Test 1: Normal success case (CLAUDECODE unset) ──────────────────────
	fmt.Println("Test 1: Normal success (CLAUDECODE unset)")
	result, raw, err := runClaude("Say PONG", nil, "CLAUDECODE")
	if err != nil {
		fmt.Printf("  FAIL: %v\nRaw: %s\n", err, raw)
	} else {
		fmt.Printf("  PASS\n")
		fmt.Printf("  result:       %q\n", result.Result)
		fmt.Printf("  session_id:   %s\n", result.SessionID)
		fmt.Printf("  is_error:     %v\n", result.IsError)
		fmt.Printf("  total_cost:   $%.8f\n", result.TotalCostUSD)
		fmt.Printf("  input_tokens: %d\n", result.Usage.InputTokens)
		fmt.Printf("  output_tokens:%d\n", result.Usage.OutputTokens)
		fmt.Printf("  cache_create: %d\n", result.Usage.CacheCreationInputTokens)
		fmt.Printf("  model_usage:  ")
		for model, mu := range result.ModelUsage {
			fmt.Printf("%s (in=%d out=%d cost=$%.8f)\n", model, mu.InputTokens, mu.OutputTokens, mu.CostUSD)
		}
		fmt.Printf("  stop_reason:  %v\n", result.StopReason)
		fmt.Printf("  fast_mode:    %s\n", result.FastModeState)
		fmt.Printf("  uuid:         %s\n", result.UUID)
	}
	fmt.Println()

	// ── Test 2: Model override ───────────────────────────────────────────────
	fmt.Println("Test 2: Model override (haiku)")
	result2, _, err := runClaude("Say PONG", []string{"--model", "claude-haiku-4-5-20251001"}, "CLAUDECODE")
	if err != nil {
		fmt.Printf("  FAIL: %v\n", err)
	} else {
		fmt.Printf("  PASS\n")
		fmt.Printf("  result:     %q\n", result2.Result)
		fmt.Printf("  model_used: ")
		for model := range result2.ModelUsage {
			fmt.Printf("%s\n", model)
		}
		fmt.Printf("  total_cost: $%.8f\n", result2.TotalCostUSD)
	}
	fmt.Println()

	// ── Test 3: Invalid model (error JSON format) ────────────────────────────
	fmt.Println("Test 3: Invalid model name (error response format)")
	result3, _, err3 := runClaude("Say PONG", []string{"--model", "invalid-model-xyz"}, "CLAUDECODE")
	if err3 != nil && result3 == nil {
		fmt.Printf("  FAIL (no JSON): %v\n", err3)
	} else if result3 != nil {
		fmt.Printf("  PASS (error captured in JSON)\n")
		fmt.Printf("  is_error:     %v\n", result3.IsError)
		fmt.Printf("  result:       %q\n", result3.Result)
		fmt.Printf("  total_cost:   $%.2f\n", result3.TotalCostUSD)
		fmt.Printf("  model_usage:  (empty map, len=%d)\n", len(result3.ModelUsage))
	}
	fmt.Println()

	// ── Test 4: CLAUDECODE set (nested session — stderr only, no JSON) ────────
	fmt.Println("Test 4: CLAUDECODE set (nested session detection)")
	cmd := exec.Command("claude", "-p", "Say PONG", "--output-format", "json", "--dangerously-skip-permissions")
	cmd.Env = append(os.Environ(), "CLAUDECODE=1")
	out4, err4 := cmd.Output()
	if err4 != nil {
		fmt.Printf("  PASS: claude rejected nested session (exit code non-zero)\n")
		fmt.Printf("  stdout: %q (expected empty)\n", string(out4))
		if exitErr, ok := err4.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			fmt.Printf("  stderr snippet: %q\n", firstN(stderr, 80))
		}
	} else {
		fmt.Printf("  UNEXPECTED: claude ran inside CLAUDECODE session (stdout: %s)\n", out4)
	}
	fmt.Println()

	fmt.Println("=== Spike complete ===")
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
