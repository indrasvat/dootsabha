// Spike 002: Gemini CLI JSON Output Parsing
//
// Tests gemini --output-format json schema: positional vs -p flag,
// --yolo vs --approval-mode yolo, token extraction, and error cases.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// GeminiResponse captures the full JSON envelope from gemini --output-format json.
// Top-level fields are flat: session_id, response, stats.
type GeminiResponse struct {
	SessionID string      `json:"session_id"`
	Response  string      `json:"response"`
	Stats     GeminiStats `json:"stats"`
}

// GeminiStats holds per-model and tool-call accounting.
type GeminiStats struct {
	Models map[string]GeminiModelStat `json:"models"`
	Tools  GeminiToolStats            `json:"tools"`
	Files  GeminiFileStats            `json:"files"`
}

// GeminiModelStat is per-model usage keyed by model name (e.g. "gemini-2.5-flash-lite").
type GeminiModelStat struct {
	API   GeminiAPIStat              `json:"api"`
	Roles map[string]GeminiRoleStat  `json:"roles"`
	// Tokens is also at the model level (aggregated from roles).
	Tokens GeminiTokenUsage `json:"tokens"`
}

// GeminiAPIStat tracks request/error/latency for a model.
type GeminiAPIStat struct {
	TotalRequests int `json:"totalRequests"`
	TotalErrors   int `json:"totalErrors"`
	TotalLatencyMs int `json:"totalLatencyMs"`
}

// GeminiRoleStat is per-role (e.g. "main", "utility_router") stat within a model.
type GeminiRoleStat struct {
	TotalRequests  int              `json:"totalRequests"`
	TotalErrors    int              `json:"totalErrors"`
	TotalLatencyMs int              `json:"totalLatencyMs"`
	Tokens         GeminiTokenUsage `json:"tokens"`
}

// GeminiTokenUsage covers input, prompt, candidates, total, cached, thoughts, tool.
type GeminiTokenUsage struct {
	Input      int `json:"input"`
	Prompt     int `json:"prompt"`
	Candidates int `json:"candidates"`
	Total      int `json:"total"`
	Cached     int `json:"cached"`
	Thoughts   int `json:"thoughts"`
	Tool       int `json:"tool"`
}

// GeminiToolStats aggregates tool call metrics.
type GeminiToolStats struct {
	TotalCalls      int `json:"totalCalls"`
	TotalSuccess    int `json:"totalSuccess"`
	TotalFail       int `json:"totalFail"`
	TotalDurationMs int `json:"totalDurationMs"`
}

// GeminiFileStats tracks lines added/removed by file operations.
type GeminiFileStats struct {
	TotalLinesAdded   int `json:"totalLinesAdded"`
	TotalLinesRemoved int `json:"totalLinesRemoved"`
}

// RunResult holds captured output from a gemini invocation.
type RunResult struct {
	Variant string
	Args    []string
	Stdout  string
	Stderr  string
	Elapsed time.Duration
	Err     error
}

// runGemini executes gemini with the given args and a timeout.
func runGemini(ctx context.Context, variant string, args ...string) RunResult {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "gemini", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return RunResult{
		Variant: variant,
		Args:    args,
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
		Elapsed: time.Since(start),
		Err:     err,
	}
}

// parseResponse decodes stdout as GeminiResponse.
func parseResponse(stdout string) (*GeminiResponse, error) {
	var resp GeminiResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &resp, nil
}

// totalTokens sums input tokens across all models in Stats.
func totalTokens(stats GeminiStats) (input, candidates int) {
	for _, m := range stats.Models {
		input += m.Tokens.Input
		candidates += m.Tokens.Candidates
	}
	return
}

func printSection(title string) {
	fmt.Printf("\n=== %s ===\n", title)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Check gemini availability.
	path, err := exec.LookPath("gemini")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gemini CLI not found in PATH — cannot run spike")
		os.Exit(1)
	}
	fmt.Printf("gemini binary: %s\n", path)

	// ── Variant 1: positional prompt + --yolo ──────────────────────────────
	printSection("Variant 1: positional prompt + --yolo")
	r1 := runGemini(ctx, "positional+--yolo", "--yolo", "--output-format", "json", "Say PONG")
	fmt.Printf("args:    %v\n", r1.Args)
	fmt.Printf("elapsed: %v\n", r1.Elapsed)
	if r1.Err != nil {
		fmt.Printf("error:   %v\n", r1.Err)
	}
	resp1, err := parseResponse(r1.Stdout)
	if err != nil {
		fmt.Printf("parse error: %v\nraw stdout:\n%s\n", err, r1.Stdout)
	} else {
		in1, cand1 := totalTokens(resp1.Stats)
		fmt.Printf("session_id:  %s\n", resp1.SessionID)
		fmt.Printf("response:    %q\n", resp1.Response)
		fmt.Printf("models:      %d\n", len(resp1.Stats.Models))
		fmt.Printf("input_tok:   %d  candidates_tok: %d\n", in1, cand1)
		for model, stat := range resp1.Stats.Models {
			fmt.Printf("  [%s] latency=%dms roles=%v\n",
				model, stat.API.TotalLatencyMs, roleNames(stat.Roles))
		}
	}

	// ── Variant 2: -p flag + --yolo ────────────────────────────────────────
	printSection("Variant 2: -p flag + --yolo")
	r2 := runGemini(ctx, "-p+--yolo", "--yolo", "-p", "Say PONG", "--output-format", "json")
	fmt.Printf("args:    %v\n", r2.Args)
	fmt.Printf("elapsed: %v\n", r2.Elapsed)
	resp2, err := parseResponse(r2.Stdout)
	if err != nil {
		fmt.Printf("parse error: %v\nraw stdout:\n%s\n", err, r2.Stdout)
	} else {
		fmt.Printf("response:    %q\n", resp2.Response)
		fmt.Printf("models match: %v\n", matchesModelSet(resp1, resp2))
	}

	// ── Variant 3: --approval-mode yolo ────────────────────────────────────
	printSection("Variant 3: --approval-mode yolo (vs --yolo)")
	r3 := runGemini(ctx, "--approval-mode+yolo", "--approval-mode", "yolo", "--output-format", "json", "Say PONG")
	fmt.Printf("args:    %v\n", r3.Args)
	fmt.Printf("elapsed: %v\n", r3.Elapsed)
	resp3, err := parseResponse(r3.Stdout)
	if err != nil {
		fmt.Printf("parse error: %v\nraw stdout:\n%s\n", err, r3.Stdout)
	} else {
		fmt.Printf("response:    %q\n", resp3.Response)
		fmt.Printf("models match: %v\n", matchesModelSet(resp1, resp3))
	}

	// ── Structural validation ───────────────────────────────────────────────
	printSection("Structural Validation")
	if resp1 != nil {
		validateResponse(resp1)
	}

	// ── Schema dump ────────────────────────────────────────────────────────
	printSection("Raw JSON Schema (pretty-printed)")
	if r1.Stdout != "" {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(r1.Stdout), "", "  "); err == nil {
			fmt.Println(pretty.String())
		}
	}

	// ── Latency comparison ─────────────────────────────────────────────────
	printSection("Latency Comparison")
	fmt.Printf("  positional+--yolo:    %v\n", r1.Elapsed)
	fmt.Printf("  -p+--yolo:            %v\n", r2.Elapsed)
	fmt.Printf("  --approval-mode yolo: %v\n", r3.Elapsed)

	fmt.Println("\nSpike 002 complete.")
}

func roleNames(roles map[string]GeminiRoleStat) []string {
	names := make([]string, 0, len(roles))
	for k := range roles {
		names = append(names, k)
	}
	return names
}

// matchesModelSet checks that both responses list the same model names.
func matchesModelSet(a, b *GeminiResponse) bool {
	if a == nil || b == nil {
		return false
	}
	if len(a.Stats.Models) != len(b.Stats.Models) {
		return false
	}
	for k := range a.Stats.Models {
		if _, ok := b.Stats.Models[k]; !ok {
			return false
		}
	}
	return true
}

// validateResponse checks required fields are present and non-zero.
func validateResponse(r *GeminiResponse) {
	checks := []struct {
		name string
		pass bool
	}{
		{"session_id non-empty", r.SessionID != ""},
		{"response non-empty", r.Response != ""},
		{"stats.models non-empty", len(r.Stats.Models) > 0},
		{"at least one model has latency > 0", anyLatency(r.Stats)},
		{"at least one model has tokens > 0", anyTokens(r.Stats)},
	}
	allPass := true
	for _, c := range checks {
		mark := "PASS"
		if !c.pass {
			mark = "FAIL"
			allPass = false
		}
		fmt.Printf("  [%s] %s\n", mark, c.name)
	}
	if allPass {
		fmt.Println("  All structural checks passed.")
	}
}

func anyLatency(s GeminiStats) bool {
	for _, m := range s.Models {
		if m.API.TotalLatencyMs > 0 {
			return true
		}
	}
	return false
}

func anyTokens(s GeminiStats) bool {
	for _, m := range s.Models {
		if m.Tokens.Total > 0 {
			return true
		}
	}
	return false
}
