package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/indrasvat/dootsabha/internal/core"
)

// Pinned invocation policy for grok. These are enforced by the provider and
// stripped from user config, because each one is load-bearing:
//
//   - streaming-messages-json: `--output-format json` concatenates every assistant
//     text block (tool-call preambles included) into `.text` with no separator.
//   - read-only sandbox: `--cwd` restricts nothing; without a sandbox grok has been
//     observed reading outside the working directory.
//   - --no-plan: plan mode returns a *plan* instead of the answer.
const (
	grokOutputFormat   = "streaming-messages-json"
	grokSandbox        = "read-only"
	grokPermissionMode = "bypassPermissions"
	grokDefaultModel   = "grok-4.5"
	grokDefaultEffort  = "high"
	grokDefaultBinary  = "grok"
)

// grokPinnedValueFlags take an argument; grokPinnedBoolFlags do not. Both are
// removed from config-supplied flags so a stray entry cannot break parsing or
// reopen the containment hole.
var (
	grokPinnedValueFlags = []string{
		"--output-format", "--sandbox", "--permission-mode",
		"-m", "--model", "-p", "--single", "--prompt-file", "--cwd", "--max-turns",
	}
	grokPinnedBoolFlags = []string{
		"--verbatim", "--plan", "--no-plan",
		"--always-approve", "--no-subagents", "--no-auto-update",
	}
	grokEffortFlags = []string{"--reasoning-effort", "--effort"}
)

// GrokProvider invokes the xAI Grok CLI (`grok`) and parses its NDJSON stream.
//
// grok supplies the richest telemetry of any दूतसभा provider: content, session id,
// token breakdown and cost. It is run with an isolated HOME so it does not inherit
// the caller's Claude Code harness (skills, hooks, MCP servers, CLAUDE.md).
type GrokProvider struct {
	cfg    *core.Config
	runner Runner
}

// NewGrokProvider constructs a GrokProvider backed by cfg and runner.
// Pass *core.SubprocessRunner as runner for production use.
func NewGrokProvider(cfg *core.Config, runner Runner) *GrokProvider {
	return &GrokProvider{cfg: cfg, runner: runner}
}

// Name returns the provider identifier.
func (p *GrokProvider) Name() string { return "grok" }

// Invoke runs the grok CLI and returns the parsed response.
func (p *GrokProvider) Invoke(ctx context.Context, prompt string, opts InvokeOptions) (*ProviderResult, error) {
	pc := p.providerConfig()

	model := pc.Model
	if opts.Model != "" {
		model = opts.Model
	}

	effort, rest := extractGrokEffort(pc.Flags)
	rest = stripPinnedFlags(rest)

	args := []string{
		"-p", prompt,
		"--output-format", grokOutputFormat,
		"-m", model,
		"--reasoning-effort", effort,
		"--sandbox", grokSandbox,
		"--permission-mode", grokPermissionMode,
		"--always-approve",
		"--no-plan",
		"--no-subagents",
		"--no-auto-update",
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(opts.MaxTurns))
	}
	args = append(args, rest...)

	env, err := grokIsolatedEnv(os.Environ())
	if err != nil {
		return nil, fmt.Errorf("grok invoke: %w", err)
	}

	slog.Debug("grok invoke", "binary", pc.Binary, "model", model, "effort", effort, "prompt_len", len(prompt))
	res, err := p.runner.Run(ctx, pc.Binary, args, core.WithEnv(env))
	if err != nil {
		return nil, fmt.Errorf("grok invoke: %w", err)
	}

	last := parseGrokNDJSON(res.Stdout)

	// No usable result event: fall back to exit code / stderr for a diagnosis.
	if last == nil {
		if res.ExitCode != 0 {
			return nil, fmt.Errorf("grok: %s", grokFailureMessage(res))
		}
		return nil, fmt.Errorf("grok invoke: no result event in stream")
	}

	// A runtime error is still type=="result" — discriminate on is_error.
	// Provider failure wins over any content seen earlier in the stream.
	if last.IsError {
		return nil, fmt.Errorf("grok: %s", last.errorMessage(res))
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("grok: %s", grokFailureMessage(res))
	}
	if strings.TrimSpace(last.Result) == "" {
		return nil, fmt.Errorf("grok invoke: empty response")
	}

	result := &ProviderResult{
		Content:   last.Result,
		Duration:  res.Duration,
		CostUSD:   last.TotalCostUSD,
		TokensIn:  last.Usage.InputTokens,
		TokensOut: last.Usage.OutputTokens,
		SessionID: last.SessionID,
		Model:     model, // overwritten below by the modelUsage key when present
	}
	// The modelUsage key is the *backend* model id and deliberately differs from
	// the -m value (e.g. "grok-4.5-build" vs "grok-4.5").
	for name, mu := range last.ModelUsage {
		result.Model = name
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

// HealthCheck runs `grok --version` to verify the binary is present.
func (p *GrokProvider) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	pc := p.providerConfig()

	res, err := p.runner.Run(ctx, pc.Binary, []string{"--version"})
	if err != nil {
		return &HealthStatus{Healthy: false, Error: err.Error()}, nil
	}
	if res.ExitCode != 0 {
		// Always carry a reason — a blank Error renders an unexplained red row.
		return &HealthStatus{Healthy: false, Error: grokFailureMessage(res)}, nil
	}

	return &HealthStatus{
		Healthy: true,
		// "grok 0.2.118 (1e1687c1cf6a) [stable]" -> "0.2.118"
		CLIVersion: parseVersion(strings.TrimSpace(string(res.Stdout))),
		Model:      pc.Model,
		AuthValid:  true,
	}, nil
}

// grokUsage is the token block on a streaming result event.
type grokUsage struct {
	InputTokens              int `json:"input_tokens"` // uncached only — not full prompt size
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// grokModelUsage is the per-backend-model breakdown, keyed by backend model id.
type grokModelUsage struct {
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CostUSD      float64 `json:"costUSD"`
}

// grokResult is the final `type:"result"` line of the streaming-messages-json
// stream. Verified against grok 0.2.118. Note this shape is used for BOTH success
// and failure: an error carries is_error=true, subtype="error_during_execution",
// a populated errors[] array, and NO result field.
//
// total_cost_usd may be omitted upstream; CostUSD is a plain float64 and makes no
// claim to distinguish "unreported" from "zero". (total_cost_usd_ticks exists only
// in `--output-format json`, which दूतसभा does not use.)
type grokResult struct {
	Type         string                    `json:"type"`
	Subtype      string                    `json:"subtype"`
	IsError      bool                      `json:"is_error"`
	Result       string                    `json:"result"`
	SessionID    string                    `json:"session_id"`
	TotalCostUSD float64                   `json:"total_cost_usd"`
	Usage        grokUsage                 `json:"usage"`
	ModelUsage   map[string]grokModelUsage `json:"modelUsage"`
	Errors       []string                  `json:"errors"`
}

// errorMessage renders the failure text for an is_error result, falling back to
// the subtype and then stderr so the user is never handed a bare "error".
func (r *grokResult) errorMessage(res *core.SubprocessResult) string {
	if msg := strings.TrimSpace(strings.Join(r.Errors, "; ")); msg != "" {
		return msg
	}
	if r.Subtype != "" {
		return r.Subtype
	}
	return grokFailureMessage(res)
}

// grokFailureMessage prefers stderr, falling back to the exit code.
func grokFailureMessage(res *core.SubprocessResult) string {
	if msg := strings.TrimSpace(string(res.Stderr)); msg != "" {
		return msg
	}
	return fmt.Sprintf("exit code %d", res.ExitCode)
}

// parseGrokNDJSON returns the last `type:"result"` event in the stream, or nil.
//
// Malformed and non-result lines are skipped defensively — the stream also carries
// system/assistant/user events. Uses bytes.Split rather than bufio.Scanner because
// real review results exceed Scanner's 64KB token limit (see parseCodexJSONL).
func parseGrokNDJSON(data []byte) *grokResult {
	var last *grokResult
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev grokResult
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // malformed line — skip
		}
		if ev.Type == "result" {
			cur := ev
			last = &cur // last result wins
		}
	}
	return last
}

// providerConfig returns the grok ProviderConfig merged over built-in defaults.
//
// Unlike its sibling providers this is nil-safe and merges field-by-field, so a
// nil *core.Config or a partial `providers.grok` block cannot blank out the
// binary name or the pinned model.
func (p *GrokProvider) providerConfig() core.ProviderConfig {
	def := core.ProviderConfig{
		Binary: grokDefaultBinary,
		Model:  grokDefaultModel,
	}
	if p.cfg == nil {
		return def
	}
	pc, ok := p.cfg.Providers["grok"]
	if !ok {
		return def
	}
	if pc.Binary == "" {
		pc.Binary = def.Binary
	}
	if pc.Model == "" {
		pc.Model = def.Model
	}
	return pc
}

// extractGrokEffort pulls --reasoning-effort/--effort out of flags, returning the
// requested effort (defaulting to high) and the remaining flags. Effort is
// user-tunable, so config wins — it is not a pinned correctness flag.
func extractGrokEffort(flags []string) (effort string, rest []string) {
	effort = grokDefaultEffort
	rest = make([]string, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		matched := false
		for _, ef := range grokEffortFlags {
			switch {
			case f == ef:
				// Only consume the next token when it is a real value. Treating a
				// following flag as the value would both set a nonsense effort and
				// silently drop that flag.
				if i+1 < len(flags) && !isFlagToken(flags[i+1]) {
					effort = flags[i+1]
					i++
				}
				matched = true
			case strings.HasPrefix(f, ef+"="):
				effort = strings.TrimPrefix(f, ef+"=")
				matched = true
			}
			if matched {
				break
			}
		}
		if !matched {
			rest = append(rest, f)
		}
	}
	return effort, rest
}

// stripPinnedFlags removes provider-controlled flags (and their values) from
// config-supplied flags, so user config can never override the pinned policy.
func stripPinnedFlags(flags []string) []string {
	out := make([]string, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		if skipped, consumesValue := matchPinned(f); skipped {
			// Only swallow the next token when it is a value, not another flag —
			// otherwise `["--sandbox", "--keep-me"]` silently drops --keep-me.
			if consumesValue && !strings.Contains(f, "=") && i+1 < len(flags) && !isFlagToken(flags[i+1]) {
				i++ // drop the flag's value too
			}
			continue
		}
		out = append(out, f)
	}
	return out
}

// isFlagToken reports whether tok looks like a flag rather than a flag's value.
// A bare "-" is a conventional stdin placeholder, so it counts as a value.
func isFlagToken(tok string) bool {
	return len(tok) > 1 && tok[0] == '-'
}

// matchPinned reports whether f is a pinned flag and whether it takes a value.
func matchPinned(f string) (pinned, consumesValue bool) {
	name := f
	if before, _, ok := strings.Cut(f, "="); ok {
		name = before
	}
	if slices.Contains(grokPinnedValueFlags, name) {
		return true, true
	}
	if slices.Contains(grokPinnedBoolFlags, name) {
		return true, false
	}
	return false, false
}

var (
	grokHomeOnce sync.Once
	grokHomeDir  string
	grokHomeErr  error
)

// grokEmptyHome returns a process-wide empty directory used as $HOME for grok.
// Created 0700 via MkdirTemp, so it is race-safe and unpredictable.
func grokEmptyHome() (string, error) {
	grokHomeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dootsabha-grok-home-")
		if err != nil {
			grokHomeErr = fmt.Errorf("create isolated grok HOME: %w", err)
			return
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			// Don't leak the directory when we refuse to use it.
			_ = os.RemoveAll(dir)
			grokHomeErr = fmt.Errorf("chmod isolated grok HOME: %w", err)
			return
		}
		grokHomeDir = dir
	})
	return grokHomeDir, grokHomeErr
}

// grokIsolatedEnv rewrites base so grok cannot inherit the caller's Claude Code
// harness. grok discovers ~/.claude/** off $HOME, so pointing HOME at an empty
// directory severs skills, agents, plugins, MCP servers, hooks and CLAUDE.md
// injection in one move — while GROK_HOME keeps auth working.
//
// Measured effect: 6 MCP servers -> 0, 53 hooks -> 0, input tokens 22066 -> 13431.
//
// The real grok home MUST be resolved from base before HOME is overridden.
func grokIsolatedEnv(base []string) ([]string, error) {
	realGrokHome := envLookup(base, "GROK_HOME")
	if realGrokHome == "" {
		if home := envLookup(base, "HOME"); home != "" {
			realGrokHome = filepath.Join(home, ".grok")
		}
	}

	emptyHome, err := grokEmptyHome()
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(base)+2)
	for _, kv := range base {
		switch {
		case strings.HasPrefix(kv, "HOME="), strings.HasPrefix(kv, "GROK_HOME="):
			continue // replaced below
		default:
			out = append(out, kv)
		}
	}
	out = append(out, "HOME="+emptyHome)
	if realGrokHome != "" {
		out = append(out, "GROK_HOME="+realGrokHome)
	}
	return out, nil
}

// envLookup returns the value of key in a KEY=VALUE slice, or "".
func envLookup(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if after, ok := strings.CutPrefix(kv, prefix); ok {
			return after
		}
	}
	return ""
}
