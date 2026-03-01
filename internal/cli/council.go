package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/output"
	"github.com/indrasvat/dootsabha/internal/providers"
)

func newCouncilCmd() *cobra.Command {
	var (
		agents   string
		chair    string
		rounds   int
		parallel bool
	)

	cmd := &cobra.Command{
		Use:     "council <prompt>",
		Aliases: []string{"sabha", "सभा"},
		Short:   "council (sabha) — Multi-agent council deliberation",
		Long: `Run a multi-agent council: dispatch to all agents in parallel, cross-review,
and synthesize into a unified answer.

सभा (sabha) — बहु-एजेंट सभा विचार-विमर्श।

Exit codes: 0 success, 1 all failed, 3 provider error, 4 timeout, 5 partial result`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve bilingual flag aliases.
			if dootas, _ := cmd.Flags().GetString("dootas"); dootas != "" && !cmd.Flags().Changed("agents") {
				agents = dootas
			}
			if adhyaksha, _ := cmd.Flags().GetString("adhyaksha"); adhyaksha != "" && !cmd.Flags().Changed("chair") {
				chair = adhyaksha
			}
			if chakra, _ := cmd.Flags().GetInt("chakra"); chakra != 0 && !cmd.Flags().Changed("rounds") {
				rounds = chakra
			}
			if cmd.Flags().Changed("samantar") {
				samantar, _ := cmd.Flags().GetBool("samantar")
				parallel = samantar
			}

			prompt := args[0]

			cfg, err := core.LoadConfig(configFile)
			if err != nil {
				return &ExitError{Code: 5, Message: fmt.Sprintf("load config: %s", err)}
			}

			// Apply flag overrides to config.
			if cmd.Flags().Changed("chair") || cmd.Flags().Changed("adhyaksha") {
				cfg.Council.Chair = chair
			}
			if cmd.Flags().Changed("rounds") || cmd.Flags().Changed("chakra") {
				cfg.Council.Rounds = rounds
			}
			if cmd.Flags().Changed("parallel") || cmd.Flags().Changed("samantar") {
				cfg.Council.Parallel = parallel
			} else {
				parallel = cfg.Council.Parallel
			}
			if cfg.Council.Rounds < 1 {
				cfg.Council.Rounds = 1
			}

			timeout := globalTimeout
			if timeout == 0 {
				timeout = cfg.Timeout
			}
			if timeout == 0 {
				timeout = 5 * 60 * 1_000_000_000 // 5 minutes
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Parse agent names.
			agentNames := strings.Split(agents, ",")
			for i := range agentNames {
				agentNames[i] = strings.TrimSpace(agentNames[i])
			}
			if len(agentNames) > core.MaxAgents {
				return &ExitError{Code: 1, Message: fmt.Sprintf("too many agents: %d (max %d)", len(agentNames), core.MaxAgents)}
			}

			// Construct agents.
			runner := &core.SubprocessRunner{}
			coreAgents := make([]core.Agent, 0, len(agentNames))
			for _, name := range agentNames {
				prov, provErr := getProvider(name, cfg, runner)
				if provErr != nil {
					return &ExitError{Code: 1, Message: provErr.Error()}
				}
				coreAgents = append(coreAgents, &providerAgent{prov: prov})
			}

			eng := core.NewEngine(coreAgents, cfg)

			rc := output.NewRenderContext(os.Stdout, jsonOutput)

			// Set up progress rendering on stderr (TTY only).
			stderrIsTTY := isatty.IsTerminal(os.Stderr.Fd())
			if stderrIsTTY && !quiet {
				eng.SetProgress(stderrProgress())
			}

			// Run council pipeline.
			var allDispatches []core.DispatchResult
			var allReviews []core.ReviewResult
			var synthesis *core.SynthesisResult
			currentPrompt := prompt

			for round := 1; round <= cfg.Council.Rounds; round++ {
				// Stage 1: Dispatch
				if stderrIsTTY && !quiet && !rc.IsJSON() {
					renderStageHeader(rc, 1, "Dispatch", fmt.Sprintf("%d agents", len(agentNames)), parallel)
				}

				dispatches, dispErr := eng.Dispatch(ctx, currentPrompt, core.InvokeOptions{Timeout: timeout})
				if dispErr != nil {
					return &ExitError{Code: 1, Message: fmt.Sprintf("dispatch: %s", dispErr)}
				}
				allDispatches = dispatches

				// Count successes.
				successes := 0
				for _, d := range dispatches {
					if d.Error == nil {
						successes++
					}
				}

				if successes == 0 {
					return &ExitError{Code: 1, Message: "all agents failed during dispatch"}
				}

				// Stage 2: Peer Review (skip if <2 successes)
				var reviews []core.ReviewResult
				if successes >= 2 {
					if stderrIsTTY && !quiet && !rc.IsJSON() {
						renderStageHeader(rc, 2, "Peer Review", "", false)
					}
					reviews, err = eng.PeerReview(ctx, dispatches, core.InvokeOptions{Timeout: timeout})
					if err != nil {
						return &ExitError{Code: 3, Message: fmt.Sprintf("peer review: %s", err)}
					}
				}
				allReviews = reviews

				// Stage 3: Synthesis
				if stderrIsTTY && !quiet && !rc.IsJSON() {
					renderStageHeader(rc, 3, "Synthesis", fmt.Sprintf("chair: %s", cfg.Council.Chair), false)
				}
				synthesis, err = eng.Synthesize(ctx, dispatches, reviews, core.InvokeOptions{Timeout: timeout})
				if err != nil {
					return &ExitError{Code: 3, Message: fmt.Sprintf("synthesis: %s", err)}
				}

				// Multi-round: feed synthesis into next round's prompt.
				if round < cfg.Council.Rounds {
					currentPrompt = fmt.Sprintf("Previous synthesis:\n%s\n\nOriginal prompt:\n%s",
						core.TruncateString(synthesis.Content, 32*1024), prompt)
				}
			}

			// Render output.
			if rc.IsJSON() {
				return renderCouncilJSON(allDispatches, allReviews, synthesis)
			}

			renderCouncilTTY(rc, allDispatches, allReviews, synthesis)

			// Exit code 5 for partial results.
			for _, d := range allDispatches {
				if d.Error != nil {
					return &ExitError{Code: 5, Message: "partial result: some agents failed"}
				}
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&agents, "agents", "claude,codex,gemini", "Comma-separated agent names")
	f.String("dootas", "", "Alias for --agents (दूत)")
	_ = f.MarkHidden("dootas")
	f.StringVar(&chair, "chair", "", "Chair agent for synthesis (default: from config)")
	f.String("adhyaksha", "", "Alias for --chair (अध्यक्ष)")
	_ = f.MarkHidden("adhyaksha")
	f.IntVar(&rounds, "rounds", 0, "Number of deliberation rounds (default: from config)")
	f.Int("chakra", 0, "Alias for --rounds (चक्र)")
	_ = f.MarkHidden("chakra")
	f.BoolVar(&parallel, "parallel", true, "Run agents in parallel")
	f.Bool("samantar", true, "Alias for --parallel (समान्तर)")
	_ = f.MarkHidden("samantar")

	return cmd
}

// providerAgent adapts providers.Provider to core.Agent, breaking the import cycle
// between core and providers.
type providerAgent struct {
	prov providers.Provider
}

func (a *providerAgent) Name() string { return a.prov.Name() }

func (a *providerAgent) Invoke(ctx context.Context, prompt string, opts core.InvokeOptions) (*core.InvokeResult, error) {
	result, err := a.prov.Invoke(ctx, prompt, providers.InvokeOptions{
		Model:    opts.Model,
		MaxTurns: opts.MaxTurns,
		Timeout:  opts.Timeout,
	})
	if err != nil {
		return nil, err
	}
	return &core.InvokeResult{
		Content:   result.Content,
		Model:     result.Model,
		Duration:  result.Duration,
		CostUSD:   result.CostUSD,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	}, nil
}

// renderStageHeader writes a stage header to stdout.
func renderStageHeader(rc *output.RenderContext, stage int, name, info string, isParallel bool) {
	border := lipgloss.NewStyle().Foreground(output.AccentColor)
	header := fmt.Sprintf("═══ Stage %d: %s ═══", stage, name)

	line := output.Styled(rc, border, header)
	if info != "" {
		muted := lipgloss.NewStyle().Foreground(output.MutedColor)
		detail := info
		if isParallel {
			detail += " · parallel"
		}
		line += "  " + output.Styled(rc, muted, detail)
	}
	fmt.Fprintln(os.Stdout, line) //nolint:errcheck
}

// stderrProgress returns a ProgressFunc that renders agent status to stderr.
func stderrProgress() core.ProgressFunc {
	var mu sync.Mutex
	return func(provider string, event core.ProgressEvent) {
		mu.Lock()
		defer mu.Unlock()
		switch event {
		case core.ProgressStarted:
			fmt.Fprintf(os.Stderr, "  %s ...\r", provider) //nolint:errcheck
		case core.ProgressDone:
			fmt.Fprintf(os.Stderr, "\r\033[K  %s ✓\n", provider) //nolint:errcheck
		case core.ProgressFailed:
			fmt.Fprintf(os.Stderr, "\r\033[K  %s ✗\n", provider) //nolint:errcheck
		}
	}
}

// --- JSON output (council-specific types to avoid collision with review.go) ---

type councilJSON struct {
	Dispatch  []councilDispatchJSON `json:"dispatch"`
	Reviews   []councilReviewJSON   `json:"reviews"`
	Synthesis councilSynthesisJSON  `json:"synthesis"`
	Meta      councilMeta           `json:"meta"`
}

type councilDispatchJSON struct {
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	Content    string  `json:"content"`
	DurationMs int64   `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd"`
	TokensIn   int     `json:"tokens_in"`
	TokensOut  int     `json:"tokens_out"`
	Error      string  `json:"error,omitempty"`
}

type councilReviewJSON struct {
	Reviewer string   `json:"reviewer"`
	Reviewed []string `json:"reviewed"`
	Content  string   `json:"content"`
	Error    string   `json:"error,omitempty"`
}

type councilSynthesisJSON struct {
	Chair         string `json:"chair"`
	ChairFallback string `json:"chair_fallback,omitempty"`
	Content       string `json:"content"`
}

type councilMeta struct {
	SchemaVersion  int               `json:"schema_version"`
	Strategy       string            `json:"strategy"`
	DurationMs     int64             `json:"duration_ms"`
	TotalCostUSD   float64           `json:"total_cost_usd"`
	TotalTokensIn  int               `json:"total_tokens_in"`
	TotalTokensOut int               `json:"total_tokens_out"`
	Providers      map[string]string `json:"providers"`
}

func renderCouncilJSON(dispatches []core.DispatchResult, reviews []core.ReviewResult, synth *core.SynthesisResult) error {
	out := councilJSON{
		Meta: councilMeta{
			SchemaVersion: 1,
			Strategy:      "council",
			Providers:     make(map[string]string),
		},
	}

	var totalDuration time.Duration

	for _, d := range dispatches {
		dj := councilDispatchJSON{
			Provider:   d.Provider,
			Model:      d.Model,
			Content:    d.Content,
			DurationMs: d.Duration.Milliseconds(),
			CostUSD:    d.CostUSD,
			TokensIn:   d.TokensIn,
			TokensOut:  d.TokensOut,
		}
		if d.Error != nil {
			dj.Error = d.Error.Error()
			out.Meta.Providers[d.Provider] = "error"
		} else {
			out.Meta.Providers[d.Provider] = "ok"
		}
		out.Meta.TotalCostUSD += d.CostUSD
		out.Meta.TotalTokensIn += d.TokensIn
		out.Meta.TotalTokensOut += d.TokensOut
		totalDuration += d.Duration
		out.Dispatch = append(out.Dispatch, dj)
	}

	for _, r := range reviews {
		rj := councilReviewJSON{
			Reviewer: r.Reviewer,
			Reviewed: r.Reviewed,
			Content:  r.Content,
		}
		if r.Error != nil {
			rj.Error = r.Error.Error()
		}
		out.Meta.TotalCostUSD += r.CostUSD
		out.Meta.TotalTokensIn += r.TokensIn
		out.Meta.TotalTokensOut += r.TokensOut
		totalDuration += r.Duration
		out.Reviews = append(out.Reviews, rj)
	}

	if synth != nil {
		out.Synthesis = councilSynthesisJSON{
			Chair:         synth.Chair,
			ChairFallback: synth.ChairFallback,
			Content:       synth.Content,
		}
		out.Meta.TotalCostUSD += synth.CostUSD
		out.Meta.TotalTokensIn += synth.TokensIn
		out.Meta.TotalTokensOut += synth.TokensOut
		totalDuration += synth.Duration
	}

	out.Meta.DurationMs = totalDuration.Milliseconds()

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("write council json: %w", err)
	}
	return nil
}

// --- TTY output ---

func renderCouncilTTY(rc *output.RenderContext, dispatches []core.DispatchResult, reviews []core.ReviewResult, synth *core.SynthesisResult) {
	// Dispatch results.
	for _, d := range dispatches {
		dot := output.ProviderDot(rc, providerColor(d.Provider))
		status := output.StatusOK(rc)
		if d.Error != nil {
			status = output.StatusFail(rc)
		}
		muted := lipgloss.NewStyle().Foreground(output.MutedColor)
		dur := fmt.Sprintf("%.1fs", d.Duration.Seconds())
		fmt.Fprintf(os.Stdout, "%s %-8s %s %s\n", dot, d.Provider, output.Styled(rc, muted, dur), status) //nolint:errcheck
	}

	// Synthesis content.
	if synth != nil {
		fmt.Fprintln(os.Stdout)                //nolint:errcheck
		fmt.Fprintln(os.Stdout, synth.Content) //nolint:errcheck
	}

	// Footer.
	fmt.Fprintln(os.Stdout) //nolint:errcheck
	sep := lipgloss.NewStyle().Foreground(output.MutedColor)
	fmt.Fprintln(os.Stdout, output.Styled(rc, sep, strings.Repeat("─", 56))) //nolint:errcheck

	var totalDuration time.Duration
	var totalCost float64
	var totalIn, totalOut int
	for _, d := range dispatches {
		totalDuration += d.Duration
		totalCost += d.CostUSD
		totalIn += d.TokensIn
		totalOut += d.TokensOut
	}
	for _, r := range reviews {
		totalDuration += r.Duration
		totalCost += r.CostUSD
		totalIn += r.TokensIn
		totalOut += r.TokensOut
	}
	if synth != nil {
		totalDuration += synth.Duration
		totalCost += synth.CostUSD
		totalIn += synth.TokensIn
		totalOut += synth.TokensOut
	}

	muted := lipgloss.NewStyle().Foreground(output.MutedColor)
	footer := fmt.Sprintf("total: %.1fs │ cost: $%.3f │ tokens: %d in · %d out",
		totalDuration.Seconds(), totalCost, totalIn, totalOut)
	fmt.Fprintln(os.Stdout, output.Styled(rc, muted, footer)) //nolint:errcheck

	// Agent status line.
	var agentParts []string
	for _, d := range dispatches {
		s := output.StatusOK(rc)
		if d.Error != nil {
			s = output.StatusFail(rc)
		}
		agentParts = append(agentParts, fmt.Sprintf("%s %s", d.Provider, s))
	}
	agentsLine := "agents: " + strings.Join(agentParts, " · ")
	fmt.Fprintln(os.Stdout, output.Styled(rc, muted, agentsLine)) //nolint:errcheck
}
