package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
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
//
// ONLY the fields दूतसभा consumes are declared. agy also emits duration_seconds,
// num_turns and usage.{thinking,cache_read,total}_tokens; decoding a field we
// never read puts it on the failure path, where one upstream type change would
// discard a good answer — and, on a failed call, the error text with it.
type agyResponse struct {
	ConversationID string   `json:"conversation_id"`
	Status         string   `json:"status"`
	Response       string   `json:"response"`
	Error          string   `json:"error"`
	Usage          agyUsage `json:"usage"`
}

// agyUsage is agy's token accounting. Counts are json.Number so a float or an
// absurd value degrades that ONE number to 0 instead of failing the whole parse.
//
// thinking_tokens is deliberately absent: verified on 1.1.17, total == in+out and
// thinking is a SUBSET of output, so adding it double-counts every reasoning turn.
type agyUsage struct {
	InputTokens  json.Number `json:"input_tokens"`
	OutputTokens json.Number `json:"output_tokens"`
}

// agyCount converts a tolerated json.Number to int, yielding 0 for anything
// unusable. Bounded by MaxInt32 because the gRPC plugin narrows to int32.
func agyCount(n json.Number) int {
	if i, err := n.Int64(); err == nil {
		if i < 0 || i > math.MaxInt32 {
			return 0
		}
		return int(i)
	}
	f, err := n.Float64()
	if err != nil || math.IsNaN(f) || f < 0 || f > math.MaxInt32 {
		return 0
	}
	return int(f)
}

// agyErrorMaxBytes bounds a surfaced agy error. Generous, because agy's most
// common failure — an unknown model — answers itself by listing every valid model
// (~750 bytes), and cutting that removes the only actionable content. Still
// capped: the field is CLI-generated, but a hostile binary could flood it.
const agyErrorMaxBytes = 2048

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
	// The prompt goes BEFORE the user's flags. agy parses argv with Go's stdlib
	// flag package, which STOPS at the first non-flag token — so one stray token
	// in `flags` (a typo'd value, a bare `true`) used to swallow `-p <prompt>`
	// entirely: agy then abandoned print mode and tried to open an interactive
	// TUI. Verified against 1.1.17.
	args = append(args, "-p", prompt)
	args = append(args, flags...)

	slog.Debug("agy invoke", "binary", pc.Binary, "model", model, "prompt_len", len(prompt))
	res, err := p.runner.Run(ctx, pc.Binary, args)
	if err != nil {
		return nil, fmt.Errorf("agy invoke: %w", err)
	}

	if res.ExitCode != 0 {
		return nil, fmt.Errorf("agy: %s", agyFailureMessage(res.Stdout, res.Stderr, res.ExitCode))
	}
	resp, err := parseAgyJSON(res.Stdout)
	if err != nil {
		return nil, fmt.Errorf("agy invoke: %w", err)
	}

	content := strings.TrimSpace(resp.Response)
	if content == "" {
		if msg := strings.TrimSpace(resp.Error); msg != "" {
			return nil, fmt.Errorf("agy: %s", core.TruncateString(msg, agyErrorMaxBytes))
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
		TokensIn:  agyCount(resp.Usage.InputTokens),
		TokensOut: agyCount(resp.Usage.OutputTokens),
		// CostUSD stays 0 — agy reports no cost in any output format.
	}, nil
}

// agyFailureMessage picks the most informative text for a non-zero exit. In JSON
// mode the reason is in the envelope and stderr is EMPTY, so the envelope wins.
//
// It is re-decoded MINIMALLY here, independently of parseAgyJSON, so that a drift
// anywhere else in the document cannot cost the user the one line explaining the
// failure — losing that is precisely the pre-707 bug.
func agyFailureMessage(stdout, stderr []byte, exitCode int) string {
	var minimal struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(stdout)).Decode(&minimal); err == nil {
		if msg := strings.TrimSpace(minimal.Error); msg != "" {
			return core.TruncateString(msg, agyErrorMaxBytes)
		}
	}
	if msg := strings.TrimSpace(string(stderr)); msg != "" {
		return core.TruncateString(msg, agyErrorMaxBytes)
	}
	return fmt.Sprintf("exit code %d", exitCode)
}

// parseAgyJSON decodes agy's single-document envelope. Verified on 1.1.17:
// stdout is exactly one line, on both the success and the failure path.
func parseAgyJSON(data []byte) (*agyResponse, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty output from agy (expected --output-format json)")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var resp agyResponse
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("json parse: %w (first 200 bytes: %q)", err, truncate(data, 200))
	}
	// Decode stops at the first value. Without this, a second envelope — or a
	// banner a future release appends — is silently dropped and the wrong turn
	// is reported as the answer.
	if dec.More() {
		return nil, fmt.Errorf("expected exactly one JSON document on stdout, got trailing data")
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

// agyPinned reports whether f is a flag दूतसभा sets itself, in ANY spelling agy
// accepts.
//
// agy parses argv with Go's stdlib flag package, so `-model` and `--model` are
// the SAME flag and a repeat is LAST-WINS. Matching only the double-dash form let
// a config `-output-format text` through and break every parse, and a config
// `-model X` silently run a different model than the one दूतसभा reports — which
// nothing downstream can detect, because the envelope never echoes the model.
//
// There is no attached short form to handle: agy's only single-letter flags are
// -c/-i/-p, so an `-m` in config reaches agy and is rejected loudly (exit 2).
func agyPinned(f string, includeModel bool) bool {
	name, _, _ := strings.Cut(f, "=")
	switch strings.TrimLeft(name, "-") {
	case "output-format":
		return true
	case "model":
		return includeModel
	}
	return false
}

// stripAgyPinnedFlags drops flags दूतसभा sets itself, so config cannot duplicate
// or override them. --output-format always (a user's `text` would break every
// parse); --model only when one was resolved to re-add.
func stripAgyPinnedFlags(flags []string, stripModel bool) []string {
	out := make([]string, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		if !agyPinned(f, stripModel) {
			out = append(out, f)
			continue
		}
		// Swallow the value only when the next token IS a value — otherwise
		// `["--output-format", "--dangerously-skip-permissions"]` silently drops
		// the auto-approve flag.
		if !strings.Contains(f, "=") && i+1 < len(flags) && !isFlagToken(flags[i+1]) {
			i++
		}
	}
	return out
}
