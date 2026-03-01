package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/indrasvat/dootsabha/internal/core"
)

// CodexProvider invokes the codex CLI and parses its JSONL event stream.
type CodexProvider struct {
	cfg    *core.Config
	runner Runner
}

// NewCodexProvider constructs a CodexProvider backed by cfg and runner.
// Pass *core.SubprocessRunner as runner for production use.
func NewCodexProvider(cfg *core.Config, runner Runner) *CodexProvider {
	return &CodexProvider{cfg: cfg, runner: runner}
}

// Name returns the provider identifier.
func (p *CodexProvider) Name() string { return "codex" }

// codexEvent is a single JSONL line emitted by `codex exec --json`.
// All fields verified against codex 0.106.0 (Spike 0.1).
type codexEvent struct {
	Type     string      `json:"type"`
	ThreadID string      `json:"thread_id,omitempty"`
	Item     *codexItem  `json:"item,omitempty"`
	Usage    *codexUsage `json:"usage,omitempty"`
	Message  string      `json:"message,omitempty"`
}

// codexItem is embedded in item.completed events.
type codexItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`              // "reasoning" | "agent_message" | "error"
	Text    string `json:"text,omitempty"`    // for reasoning + agent_message
	Message string `json:"message,omitempty"` // for type=="error"
}

// codexUsage is embedded in turn.completed events.
// cached_input_tokens is undocumented but present in v0.106.0 (Spike 0.1).
type codexUsage struct {
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

// Invoke runs `codex exec --json --sandbox danger-full-access --ephemeral --skip-git-repo-check <prompt>`
// and returns the parsed response from the JSONL event stream.
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, opts InvokeOptions) (*ProviderResult, error) {
	pc := p.providerConfig()

	// Build args: "exec --json" + config flags + prompt (positional last)
	args := []string{"exec", "--json"}
	args = append(args, pc.Flags...)
	args = append(args, prompt)

	res, err := p.runner.Run(ctx, pc.Binary, args)
	if err != nil {
		return nil, fmt.Errorf("codex invoke: %w", err)
	}

	agentMsg, usage, err := parseCodexJSONL(res.Stdout)
	if err != nil {
		return nil, fmt.Errorf("codex invoke: %w", err)
	}
	if agentMsg == "" {
		return nil, fmt.Errorf("codex invoke: no agent_message found in output")
	}

	result := &ProviderResult{
		Content:  agentMsg,
		Model:    pc.Model,
		Duration: res.Duration,
	}
	if usage != nil {
		result.TokensIn = usage.InputTokens
		result.TokensOut = usage.OutputTokens
	}

	return result, nil
}

// HealthCheck runs `codex --version` to verify the binary is present.
func (p *CodexProvider) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	pc := p.providerConfig()

	res, err := p.runner.Run(ctx, pc.Binary, []string{"--version"})
	if err != nil {
		return &HealthStatus{
			Healthy: false,
			Error:   err.Error(),
		}, nil
	}
	if res.ExitCode != 0 {
		return &HealthStatus{
			Healthy: false,
			Error:   strings.TrimSpace(string(res.Stderr)),
		}, nil
	}

	return &HealthStatus{
		Healthy:    true,
		CLIVersion: parseVersion(strings.TrimSpace(string(res.Stdout))),
		Model:      pc.Model,
		AuthValid:  true,
	}, nil
}

// providerConfig returns the codex ProviderConfig, falling back to defaults
// if the config map is missing the "codex" key.
func (p *CodexProvider) providerConfig() core.ProviderConfig {
	if pc, ok := p.cfg.Providers["codex"]; ok {
		return pc
	}
	return core.ProviderConfig{
		Binary: "codex",
		Model:  "gpt-5.3-codex",
		Flags:  []string{"--sandbox", "danger-full-access", "--ephemeral", "--skip-git-repo-check", "-c", "model_reasoning_effort=medium"},
	}
}

// parseCodexJSONL extracts the agent message and usage from the Codex JSONL stream.
// Robust: skips malformed lines and non-fatal error events; last agent_message wins.
// All behaviors verified against codex 0.106.0 (Spike 0.1).
func parseCodexJSONL(data []byte) (agentMsg string, usage *codexUsage, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev codexEvent
		if jsonErr := json.Unmarshal([]byte(line), &ev); jsonErr != nil {
			// Malformed line — skip and continue (defensive parsing).
			continue
		}
		switch ev.Type {
		case "item.completed":
			if ev.Item != nil && ev.Item.Type == "agent_message" {
				agentMsg = ev.Item.Text // last-write-wins for multiple messages
			}
			// item.type=="error" and item.type=="reasoning" are skipped (non-fatal)
		case "turn.completed":
			usage = ev.Usage
		case "error":
			// Top-level reconnect/transport errors are non-fatal (Spike 0.1 §1).
		}
	}
	return agentMsg, usage, scanner.Err()
}
