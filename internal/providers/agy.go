package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/indrasvat/dootsabha/internal/core"
)

// AgyDefaultModel is the single source of truth for agy's default model.
//
// A display name, not the `agy models` id: both work, but this is the spelling
// the CLI echoes in its own error and that status/config show render. The
// (High|Medium|Low) suffix IS the effort selector — agy rejects --effort for the
// lower tiers, so दूतसभा never emits it as a second axis.
//
// The plugin and the extension context read this. The viper default (core cannot
// import providers) and configs/default.yaml cannot; TestAgyDefaultModelSourcesAgree
// guards those. Do not add a fifth copy.
const AgyDefaultModel = "Gemini 3.7 Flash (High)"

// agyStatusSuccess marks a clean turn. Any other value is a tool-level
// diagnostic, not an invocation failure — see Invoke.
const agyStatusSuccess = "SUCCESS"

// AgyProvider invokes the Antigravity CLI (`agy`), Google's replacement for the
// sunset Gemini CLI, and parses its `--output-format json` envelope (1.1.17).
//
// JSON is not optional: in that mode agy writes failures to STDOUT and leaves
// stderr empty, so a text-mode reader loses the error message entirely. It also
// carries the conversation id and token counts.
//
// agy reports no cost and does not echo the model, so CostUSD stays 0 and Model
// is the value दूतसभा sent.
type AgyProvider struct {
	cfg    *core.Config
	runner Runner
}

// agyResponse is the JSON envelope from `agy --output-format json`.
type agyResponse struct {
	ConversationID  string   `json:"conversation_id"`
	Status          string   `json:"status"`
	Response        string   `json:"response"`
	Error           string   `json:"error"`
	DurationSeconds float64  `json:"duration_seconds"`
	NumTurns        int      `json:"num_turns"`
	Usage           agyUsage `json:"usage"`
}

// agyUsage is agy's token accounting. Verified on 1.1.17: total_tokens ==
// input+output, and thinking_tokens is a SUBSET of output_tokens — adding them
// double-counts every reasoning turn.
type agyUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ThinkingTokens  int `json:"thinking_tokens"`
	CacheReadTokens int `json:"cache_read_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

// NewAgyProvider constructs an AgyProvider backed by cfg and runner.
// Pass *core.SubprocessRunner as runner for production use.
func NewAgyProvider(cfg *core.Config, runner Runner) *AgyProvider {
	return &AgyProvider{cfg: cfg, runner: runner}
}

// Name returns the provider identifier.
func (p *AgyProvider) Name() string { return "agy" }

// Invoke runs `agy --model <model> --output-format json -p <prompt>`.
//
// `status` is NOT the failure discriminator: a turn whose tool call failed but
// which still answered returns exit 0 + status "ERROR" + a populated response
// (observed live — a `find` hit its deadline, the answer was still right).
// Rejecting that would discard usable output. So:
//
//	exit != 0                     → failure; message from JSON `error`, else stderr
//	exit == 0, response non-empty → success; non-SUCCESS status logged, not fatal
//	exit == 0, response empty     → failure; JSON `error` if agy gave one
func (p *AgyProvider) Invoke(ctx context.Context, prompt string, opts InvokeOptions) (*ProviderResult, error) {
	pc := p.providerConfig()

	args := make([]string, 0, len(pc.Flags)+6)
	model := pc.Model
	if opts.Model != "" {
		model = opts.Model
	}
	// --output-format is always ours; --model only when दूतसभा resolved one, so
	// clearing providers.agy.model still lets a user pick from flags.
	flags := stripAgyPinnedFlags(pc.Flags, model != "")
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "--output-format", "json")
	args = append(args, flags...)
	args = append(args, "-p", prompt)

	slog.Debug("agy invoke", "binary", pc.Binary, "model", model, "prompt_len", len(prompt))
	res, err := p.runner.Run(ctx, pc.Binary, args)
	if err != nil {
		return nil, fmt.Errorf("agy invoke: %w", err)
	}

	resp, parseErr := parseAgyJSON(res.Stdout)

	if res.ExitCode != 0 {
		return nil, fmt.Errorf("agy: %s", agyFailureMessage(resp, parseErr, res))
	}
	if parseErr != nil {
		return nil, fmt.Errorf("agy invoke: %w", parseErr)
	}

	content := strings.TrimSpace(resp.Response)
	if content == "" {
		if msg := strings.TrimSpace(resp.Error); msg != "" {
			return nil, fmt.Errorf("agy: %s", core.TruncateString(msg, 400))
		}
		return nil, fmt.Errorf("agy invoke: empty response")
	}

	// A tool failed inside a still-usable turn: surface under -v, do not discard.
	if resp.Status != agyStatusSuccess {
		slog.Warn("agy reported a degraded turn",
			"status", resp.Status,
			"error", core.TruncateString(strings.TrimSpace(resp.Error), 200),
			"conversation_id", resp.ConversationID)
	}

	return &ProviderResult{
		Content:   content,
		Duration:  res.Duration,
		Model:     model,
		SessionID: resp.ConversationID,
		TokensIn:  resp.Usage.InputTokens,
		TokensOut: resp.Usage.OutputTokens,
		// CostUSD stays 0 — agy reports no cost in any output format.
	}, nil
}

// agyFailureMessage picks the most informative text for a non-zero exit. In JSON
// mode the reason is in the envelope and stderr is EMPTY, so try that first.
func agyFailureMessage(resp *agyResponse, parseErr error, res *core.SubprocessResult) string {
	if parseErr == nil && resp != nil {
		if msg := strings.TrimSpace(resp.Error); msg != "" {
			return core.TruncateString(msg, 400)
		}
	}
	if msg := strings.TrimSpace(string(res.Stderr)); msg != "" {
		return core.TruncateString(msg, 400)
	}
	return fmt.Sprintf("exit code %d", res.ExitCode)
}

// parseAgyJSON decodes agy's single-document envelope. Verified on 1.1.17:
// stdout is exactly one line, on both the success and the failure path.
func parseAgyJSON(data []byte) (*agyResponse, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty output from agy (expected --output-format json)")
	}
	var resp agyResponse
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&resp); err != nil {
		return nil, fmt.Errorf("json parse: %w (first 200 bytes: %q)", err, truncate(data, 200))
	}
	return &resp, nil
}

// HealthCheck runs `agy --version` to verify the binary is present.
func (p *AgyProvider) HealthCheck(ctx context.Context) (*HealthStatus, error) {
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

// providerConfig returns the agy ProviderConfig, falling back to defaults if the
// config is nil or missing the "agy" key. Nil-safe like GrokProvider's: callers
// construct providers before config load fails, and panicking there is worse
// than running on defaults.
func (p *AgyProvider) providerConfig() core.ProviderConfig {
	def := core.ProviderConfig{
		Binary: "agy",
		Model:  AgyDefaultModel,
		Flags:  []string{"--dangerously-skip-permissions"},
	}
	if p.cfg == nil {
		return def
	}
	if pc, ok := p.cfg.Providers["agy"]; ok {
		return pc
	}
	return def
}

// stripAgyPinnedFlags drops flags दूतसभा sets itself, so config cannot duplicate
// or override them. --output-format always (a user's `text` would break every
// parse); --model only when one was resolved to re-add.
func stripAgyPinnedFlags(flags []string, stripModel bool) []string {
	const (
		modelLong  = "--model"
		modelShort = "-m"
		formatFlag = "--output-format"
	)
	out := make([]string, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		flag := flags[i]
		isModel := stripModel && (flag == modelLong || flag == modelShort)
		isModelAttached := stripModel &&
			(strings.HasPrefix(flag, modelLong+"=") || strings.HasPrefix(flag, modelShort+"="))
		switch {
		case isModel || flag == formatFlag:
			if i+1 < len(flags) {
				i++
			}
			continue
		case isModelAttached, strings.HasPrefix(flag, formatFlag+"="):
			continue
		default:
			out = append(out, flag)
		}
	}
	return out
}
