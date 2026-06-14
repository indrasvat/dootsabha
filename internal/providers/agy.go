package providers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/indrasvat/dootsabha/internal/core"
)

// AgyProvider invokes the Antigravity CLI (`agy`) and captures its plain-text output.
//
// Antigravity is Google's agent-first replacement for the Gemini CLI (scheduled
// sunset 2026-06-18). Unlike `gemini`, its print mode (`-p`) emits plain text only — there
// is no `--output-format json`, so no session ID, token counts, or cost are
// available. The provider populates Content and Duration; token/cost fields stay 0.
type AgyProvider struct {
	cfg    *core.Config
	runner Runner
}

// NewAgyProvider constructs an AgyProvider backed by cfg and runner.
// Pass *core.SubprocessRunner as runner for production use.
func NewAgyProvider(cfg *core.Config, runner Runner) *AgyProvider {
	return &AgyProvider{cfg: cfg, runner: runner}
}

// Name returns the provider identifier.
func (p *AgyProvider) Name() string { return "agy" }

// Invoke runs `agy --dangerously-skip-permissions --model <model> -p <prompt>` and
// returns the captured stdout as Content. The --dangerously-skip-permissions flag is
// the yolo-equivalent that auto-approves tool permission requests for non-interactive
// use. Verified against agy 1.0.8.
func (p *AgyProvider) Invoke(ctx context.Context, prompt string, opts InvokeOptions) (*ProviderResult, error) {
	pc := p.providerConfig()

	// Build args: "--model <model>" + config flags + "-p <prompt>".
	args := make([]string, 0, len(pc.Flags)+4)
	model := pc.Model
	if opts.Model != "" {
		model = opts.Model
	}
	flags := pc.Flags
	if model != "" {
		flags = stripAgyModelFlags(flags)
		args = append(args, "--model", model)
	}
	args = append(args, flags...)
	args = append(args, "-p", prompt)

	slog.Debug("agy invoke", "binary", pc.Binary, "model", model, "prompt_len", len(prompt))
	res, err := p.runner.Run(ctx, pc.Binary, args)
	if err != nil {
		return nil, fmt.Errorf("agy invoke: %w", err)
	}

	// Antigravity has no JSON error format — failures surface on stderr / exit code.
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(string(res.Stderr))
		if msg == "" {
			msg = fmt.Sprintf("exit code %d", res.ExitCode)
		}
		return nil, fmt.Errorf("agy: %s", msg)
	}

	content := strings.TrimSpace(string(res.Stdout))
	if content == "" {
		// Surface any stderr tail to make an empty (but exit-0) response debuggable.
		if msg := strings.TrimSpace(string(res.Stderr)); msg != "" {
			return nil, fmt.Errorf("agy invoke: empty response (stderr: %s)", core.TruncateString(msg, 200))
		}
		return nil, fmt.Errorf("agy invoke: empty response")
	}

	return &ProviderResult{
		Content:  content,
		Duration: res.Duration,
		Model:    model,
		// SessionID, TokensIn/Out, and CostUSD are unavailable in print mode.
	}, nil
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

// providerConfig returns the agy ProviderConfig, falling back to defaults
// if the config map is missing the "agy" key.
func (p *AgyProvider) providerConfig() core.ProviderConfig {
	if pc, ok := p.cfg.Providers["agy"]; ok {
		return pc
	}
	return core.ProviderConfig{
		Binary: "agy",
		Model:  "Gemini 3.5 Flash (High)",
		Flags:  []string{"--dangerously-skip-permissions"},
	}
}

func stripAgyModelFlags(flags []string) []string {
	out := make([]string, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		flag := flags[i]
		switch {
		case flag == "--model" || flag == "-m":
			if i+1 < len(flags) {
				i++
			}
			continue
		case strings.HasPrefix(flag, "--model=") || strings.HasPrefix(flag, "-m="):
			continue
		default:
			out = append(out, flag)
		}
	}
	return out
}
