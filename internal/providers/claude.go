package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/indrasvat/dootsabha/internal/core"
)

// ClaudeProvider invokes the claude CLI and parses its JSON output.
type ClaudeProvider struct {
	cfg    *core.Config
	runner Runner
}

// NewClaudeProvider constructs a ClaudeProvider backed by cfg and runner.
// Pass *core.SubprocessRunner as runner for production use.
func NewClaudeProvider(cfg *core.Config, runner Runner) *ClaudeProvider {
	return &ClaudeProvider{cfg: cfg, runner: runner}
}

// Name returns the provider identifier.
func (p *ClaudeProvider) Name() string { return "claude" }

// claudeResponse is the JSON envelope from `claude --output-format json`.
// All fields verified against claude 2.1.63 (Spike 0.2).
type claudeResponse struct {
	IsError      bool                   `json:"is_error"`
	Result       string                 `json:"result"`
	StopReason   *string                `json:"stop_reason"` // nullable
	SessionID    string                 `json:"session_id"`
	TotalCostUSD float64                `json:"total_cost_usd"`
	DurationMs   int                    `json:"duration_ms"`
	Usage        claudeUsage            `json:"usage"`
	ModelUsage   map[string]claudeModel `json:"modelUsage"` // empty map on error
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeModel struct {
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CostUSD      float64 `json:"costUSD"`
}

// Invoke runs `claude -p <prompt> --output-format json` and returns the
// parsed response. CLAUDECODE env vars are stripped per Spike 0.2.
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, opts InvokeOptions) (*ProviderResult, error) {
	pc := p.providerConfig()

	args := []string{"-p", prompt, "--output-format", "json"}
	args = append(args, pc.Flags...)

	model := pc.Model
	if opts.Model != "" {
		model = opts.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	sanitized := core.SanitizeEnvForClaude(os.Environ())
	res, err := p.runner.Run(ctx, pc.Binary, args, core.WithEnv(sanitized))
	if err != nil {
		return nil, fmt.Errorf("claude invoke: %w", err)
	}

	resp, err := parseClaudeJSON(res.Stdout)
	if err != nil {
		return nil, fmt.Errorf("claude invoke: %w", err)
	}
	if resp.IsError {
		return nil, fmt.Errorf("claude error: %s", resp.Result)
	}

	result := &ProviderResult{
		Content:   resp.Result,
		Duration:  res.Duration,
		CostUSD:   resp.TotalCostUSD,
		TokensIn:  resp.Usage.InputTokens,
		TokensOut: resp.Usage.OutputTokens,
		SessionID: resp.SessionID,
		Model:     model, // fallback; overwritten by modelUsage key below
	}

	// Extract the model name and per-model token counts from the modelUsage
	// map. The key is the model ID string. Only one entry is expected per call.
	for modelName, mu := range resp.ModelUsage {
		result.Model = modelName
		if mu.InputTokens > 0 {
			result.TokensIn = mu.InputTokens
		}
		if mu.OutputTokens > 0 {
			result.TokensOut = mu.OutputTokens
		}
		break
	}

	return result, nil
}

// HealthCheck runs `claude --version` to verify the binary is present.
func (p *ClaudeProvider) HealthCheck(ctx context.Context) (*HealthStatus, error) {
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

// providerConfig returns the claude ProviderConfig, falling back to defaults
// if the config map is missing the "claude" key.
func (p *ClaudeProvider) providerConfig() core.ProviderConfig {
	if pc, ok := p.cfg.Providers["claude"]; ok {
		return pc
	}
	return core.ProviderConfig{
		Binary: "claude",
		Model:  "claude-sonnet-4-6",
		Flags:  []string{"--dangerously-skip-permissions"},
	}
}

// parseClaudeJSON decodes the first JSON object from claude's stdout.
// Using json.NewDecoder (not json.Unmarshal) handles the double-JSON edge
// case documented in Spike 0.2 §2 — on model errors claude may emit the
// JSON object twice; Decoder stops after the first complete object.
func parseClaudeJSON(data []byte) (*claudeResponse, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty stdout — possible CLAUDECODE nested session error")
	}
	var resp claudeResponse
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&resp); err != nil {
		return nil, fmt.Errorf("json parse: %w (first 200 bytes: %q)", err, truncate(data, 200))
	}
	return &resp, nil
}

// parseVersion extracts the version string from "claude X.Y.Z" output.
// Returns the last whitespace-delimited token, or the full string on failure.
func parseVersion(output string) string {
	parts := strings.Fields(output)
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return output
}

// truncate returns at most n bytes from data.
func truncate(data []byte, n int) []byte {
	if len(data) <= n {
		return data
	}
	return data[:n]
}
