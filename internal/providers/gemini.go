package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/indrasvat/dootsabha/internal/core"
)

// GeminiProvider invokes the gemini CLI and parses its JSON output.
type GeminiProvider struct {
	cfg    *core.Config
	runner Runner
}

// NewGeminiProvider constructs a GeminiProvider backed by cfg and runner.
// Pass *core.SubprocessRunner as runner for production use.
func NewGeminiProvider(cfg *core.Config, runner Runner) *GeminiProvider {
	return &GeminiProvider{cfg: cfg, runner: runner}
}

// Name returns the provider identifier.
func (p *GeminiProvider) Name() string { return "gemini" }

// geminiResponse is the JSON envelope from `gemini --approval-mode yolo --output-format json`.
// All fields verified against gemini 0.30.0 (Spike 0.3).
type geminiResponse struct {
	SessionID string      `json:"session_id"`
	Response  string      `json:"response"`
	Stats     geminiStats `json:"stats"`
}

type geminiStats struct {
	Models map[string]geminiModelStat `json:"models"`
}

type geminiModelStat struct {
	Tokens geminiTokenUsage          `json:"tokens"`
	Roles  map[string]geminiRoleStat `json:"roles"`
}

type geminiRoleStat struct {
	Tokens geminiTokenUsage `json:"tokens"`
}

// geminiTokenUsage holds per-model/per-role token counts.
// Field names verified against gemini 0.30.0 (Spike 0.3).
type geminiTokenUsage struct {
	Input      int `json:"input"`
	Prompt     int `json:"prompt"`     // duplicate of Input in v0.30.0
	Candidates int `json:"candidates"` // output tokens
	Total      int `json:"total"`
	Cached     int `json:"cached"`
	Thoughts   int `json:"thoughts"`
	Tool       int `json:"tool"`
}

// Invoke runs `gemini --approval-mode yolo --output-format json <prompt>` and returns the
// parsed response. Prompt is passed as a positional argument (Spike 0.3 §2).
func (p *GeminiProvider) Invoke(ctx context.Context, prompt string, opts InvokeOptions) (*ProviderResult, error) {
	pc := p.providerConfig()

	// Build args: config flags + "--output-format json" + prompt (positional last)
	args := append([]string{}, pc.Flags...)
	args = append(args, "--output-format", "json")
	args = append(args, prompt)

	res, err := p.runner.Run(ctx, pc.Binary, args)
	if err != nil {
		return nil, fmt.Errorf("gemini invoke: %w", err)
	}

	// Errors appear on stderr only — no JSON error format (Spike 0.3 §7).
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(string(res.Stderr))
		if msg == "" {
			msg = fmt.Sprintf("exit code %d", res.ExitCode)
		}
		return nil, fmt.Errorf("gemini: %s", msg)
	}

	resp, err := parseGeminiJSON(res.Stdout)
	if err != nil {
		return nil, fmt.Errorf("gemini invoke: %w", err)
	}

	result := &ProviderResult{
		Content:   resp.Response,
		SessionID: resp.SessionID,
		Duration:  res.Duration,
		Model:     pc.Model,
	}

	// Extract token counts from the "main" role model (Spike 0.3 §5).
	for modelName, stat := range resp.Stats.Models {
		for roleName, role := range stat.Roles {
			if roleName == "main" {
				result.Model = modelName
				result.TokensIn = role.Tokens.Input
				result.TokensOut = role.Tokens.Candidates
				break
			}
		}
	}

	return result, nil
}

// HealthCheck runs `gemini --version` to verify the binary is present.
func (p *GeminiProvider) HealthCheck(ctx context.Context) (*HealthStatus, error) {
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

// providerConfig returns the gemini ProviderConfig, falling back to defaults
// if the config map is missing the "gemini" key.
func (p *GeminiProvider) providerConfig() core.ProviderConfig {
	if pc, ok := p.cfg.Providers["gemini"]; ok {
		return pc
	}
	return core.ProviderConfig{
		Binary: "gemini",
		Model:  "gemini-3-pro",
		Flags:  []string{"--approval-mode", "yolo"},
	}
}

// parseGeminiJSON decodes the single JSON object from gemini's stdout.
func parseGeminiJSON(data []byte) (*geminiResponse, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty stdout")
	}
	var resp geminiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("json parse: %w (first 200 bytes: %q)", err, truncate(data, 200))
	}
	return &resp, nil
}
