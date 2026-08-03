package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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

Exit codes: 0 success, 2 bad command, 3 all agents failed, 4 timeout, 5 partial result, 6 config error`,
		Args:         usageArgs(cobra.ExactArgs(1)),
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
				return &ExitError{Code: core.ExitConfig, Message: fmt.Sprintf("load config: %s", err)}
			}

			// Apply flag overrides to config.
			if cmd.Flags().Changed("chair") || cmd.Flags().Changed("adhyaksha") {
				cfg.Council.Chair = chair
			}
			// Validate the RESOLVED chair. Checking only the flag let an unknown
			// chair from YAML or DOOTSABHA_COUNCIL_CHAIR through, after which
			// synthesis silently fell back to another agent and reported success.
			if err := validateChair(cfg.Council.Chair); err != nil {
				return &ExitError{Code: core.ExitUsage, Message: err.Error()}
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
			if cfg.Council.Rounds > core.MaxRounds {
				return &ExitError{
					Code:    core.ExitUsage,
					Message: fmt.Sprintf("too many rounds: %d (max %d)", cfg.Council.Rounds, core.MaxRounds),
				}
			}

			// Dispatch, peer review and synthesis are separate invocations, and
			// multi-round councils repeat all three. Each call gets its own
			// window inside one pipeline ceiling (issue #20); the engine derives
			// the per-invocation context from InvokeOptions.Timeout.
			// Per round: one call per agent, one peer review per agent, one
			// synthesis. Rounds repeat all three.
			budget := newBudget(cmd, cfg, cfg.Council.Rounds*(2*len(splitAgentList(agents))+1))
			defer budget.Close()
			ctx := budget.Session()
			invokeOpts := core.InvokeOptions{Timeout: budget.PerInvoke()}

			// Parse agent names.
			agentNames := splitAgentList(agents)
			if err := validateAgentNames(agentNames, "--agents"); err != nil {
				return err
			}
			if len(agentNames) > core.MaxAgents {
				return &ExitError{Code: core.ExitUsage, Message: fmt.Sprintf("too many agents: %d (max %d)", len(agentNames), core.MaxAgents)}
			}

			// Construct agents.
			runner := &core.SubprocessRunner{}
			coreAgents := make([]core.Agent, 0, len(agentNames))
			for _, name := range agentNames {
				prov, provErr := getProvider(name, cfg, runner)
				if provErr != nil {
					return &ExitError{Code: core.ExitUsage, Message: provErr.Error()}
				}
				coreAgents = append(coreAgents, &providerAgent{prov: prov})
			}

			eng := core.NewEngine(coreAgents, cfg)

			rc := output.NewRenderContext(os.Stdout, jsonOutput)

			// Progress rendering on stderr (TTY only, not JSON mode).
			stderrIsTTY := isatty.IsTerminal(os.Stderr.Fd())

			// Render command header.
			if rc.IsTTY && !rc.IsJSON() {
				info := fmt.Sprintf("agents: %s · chair: %s", strings.Join(agentNames, ", "), cfg.Council.Chair)
				fmt.Fprintln(os.Stdout, output.CommandHeader(rc, "Council", info)) //nolint:errcheck
				fmt.Fprintln(os.Stdout)                                            //nolint:errcheck
			}

			// Run council pipeline.
			var allDispatches []core.DispatchResult
			var allReviews []core.ReviewResult
			var synthesis *core.SynthesisResult
			currentPrompt := prompt

			for round := 1; round <= cfg.Council.Rounds; round++ {
				// Stage 1: Dispatch
				dispatchInfo := fmt.Sprintf("%d agents", len(agentNames))
				if parallel {
					dispatchInfo += " · parallel"
				}
				if stderrIsTTY && !quiet && !rc.IsJSON() {
					fmt.Fprintln(os.Stdout, output.SectionDivider(rc, "Dispatch", dispatchInfo)) //nolint:errcheck
				}
				if stderrIsTTY && !quiet && !rc.IsJSON() {
					eng.SetProgress(stderrProgress("dispatch", rc.HasColor))
				}

				dispatches, dispErr := eng.Dispatch(ctx, currentPrompt, invokeOpts)
				if dispErr != nil {
					if rc.IsJSON() {
						_ = renderCouncilJSON(allDispatches, nil, nil)
					}
					return &ExitError{Code: core.ExitError, Message: fmt.Sprintf("dispatch: %s", dispErr)}
				}
				// Accumulate across rounds. Assigning here reported only the FINAL
				// round, so a 3-round council under-reported its own cost by ~2/3
				// and discarded earlier rounds' agent output entirely.
				allDispatches = append(allDispatches, dispatches...)

				// Count successes.
				successes := 0
				for _, d := range dispatches {
					if d.Error == nil {
						successes++
					}
				}

				if successes == 0 {
					if rc.IsJSON() {
						_ = renderCouncilJSON(allDispatches, allReviews, nil)
					}
					// Judge from the ACCUMULATED payload, not just this round. With
					// multi-round accumulation an earlier round's output may still be
					// in the JSON, and reporting "nothing usable" would tell callers
					// to discard content they already paid for.
					fallback := core.ExitProvider
					for _, d := range allDispatches {
						if d.Error == nil {
							fallback = core.ExitPartial
							break
						}
					}
					// Name the budget that fired, if one did — "all agents
					// failed" alone sends a reader hunting for a broken CLI
					// when the real answer is "raise this timeout".
					msg := "all agents failed during dispatch"
					deadline := councilDeadline(allDispatches, allReviews, nil, ctx.Err())
					if deadline != nil {
						msg = "all agents failed during dispatch: " + timeoutMessage(budget, "", deadline)
					}
					return &ExitError{Code: stageExitCode(ctx, deadline, fallback), Message: msg}
				}

				// Stage 2: Peer Review (skip if <2 successes)
				var reviews []core.ReviewResult
				if successes >= 2 {
					if stderrIsTTY && !quiet && !rc.IsJSON() {
						fmt.Fprintln(os.Stdout)                                               //nolint:errcheck
						fmt.Fprintln(os.Stdout, output.SectionDivider(rc, "Peer Review", "")) //nolint:errcheck
					}
					if stderrIsTTY && !quiet && !rc.IsJSON() {
						eng.SetProgress(stderrProgress("review", rc.HasColor))
					}
					reviews, err = eng.PeerReview(ctx, dispatches, invokeOpts)
					if err != nil {
						if rc.IsJSON() {
							_ = renderCouncilJSON(allDispatches, allReviews, nil)
						}
						// Dispatch already produced usable agent output, so this is
						// a partial result — not "nothing usable". Reporting 3 here
						// told callers to discard content they had already paid for.
						return &ExitError{Code: stageExitCode(ctx, err, core.ExitPartial), Message: stageFailureMessage(budget, "peer review", err)}
					}
				}
				allReviews = append(allReviews, reviews...)

				// Stage 3: Synthesis
				if stderrIsTTY && !quiet && !rc.IsJSON() {
					fmt.Fprintln(os.Stdout)                                                                                      //nolint:errcheck
					fmt.Fprintln(os.Stdout, output.SectionDivider(rc, "Synthesis", fmt.Sprintf("chair: %s", cfg.Council.Chair))) //nolint:errcheck
				}
				synthesis, err = eng.Synthesize(ctx, dispatches, reviews, invokeOpts)
				if err != nil {
					if rc.IsJSON() {
						_ = renderCouncilJSON(allDispatches, allReviews, nil)
					}
					// Same reasoning as peer review: the dispatch output is in the
					// payload and is usable, so this is partial, not total failure.
					return &ExitError{Code: stageExitCode(ctx, err, core.ExitPartial), Message: fmt.Sprintf("synthesis: %s", err)}
				}

				// Surface a chair fallback. It is recorded in JSON as
				// `chair_fallback`, but a human reading the terminal would
				// otherwise believe their chosen chair wrote the synthesis when a
				// different agent actually did.
				if synthesis != nil && synthesis.ChairFallback != "" && !rc.IsJSON() && !quiet {
					fmt.Fprintf(os.Stderr, "Warning: chair %q unavailable — synthesized by %q instead\n", //nolint:errcheck
						cfg.Council.Chair, synthesis.ChairFallback)
				}

				// Multi-round: feed synthesis into next round's prompt.
				if round < cfg.Council.Rounds {
					currentPrompt = fmt.Sprintf("Previous synthesis:\n%s\n\nOriginal prompt:\n%s",
						core.TruncateString(synthesis.Content, 32*1024), prompt)
				}
			}

			// Render output.
			if rc.IsJSON() {
				if err := renderCouncilJSON(allDispatches, allReviews, synthesis); err != nil {
					return &ExitError{Code: core.ExitError, Message: fmt.Sprintf("write json: %s", err)}
				}
				// Return correct exit code even in JSON mode.
				return councilExit(budget, allDispatches, allReviews, synthesis)
			}

			renderCouncilTTY(rc, allDispatches, allReviews, synthesis)

			return councilExit(budget, allDispatches, allReviews, synthesis)
		},
	}

	f := cmd.Flags()
	defaultAgents := "claude,codex,agy"
	if core.InsideClaude {
		defaultAgents = "codex,agy" // Claude is already the host session
	}
	f.StringVar(&agents, "agents", defaultAgents, "Comma-separated agent names (claude, codex, agy, grok)")
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

// councilErrors collects every per-agent failure a council round recorded,
// across all three stages.
//
// The chair's error is included even when the fallback then succeeded: a chair
// that timed out is still "an agent timed out", and dropping it let a council
// exit 0 on a run that hit a deadline.
func councilErrors(dispatches []core.DispatchResult, reviews []core.ReviewResult, synth *core.SynthesisResult) []error {
	errs := make([]error, 0, len(dispatches)+len(reviews)+1)
	for _, d := range dispatches {
		errs = append(errs, d.Error)
	}
	for _, r := range reviews {
		errs = append(errs, r.Error)
	}
	if synth != nil {
		errs = append(errs, synth.ChairError)
	}
	return errs
}

// councilDeadline returns a deadline error recorded anywhere in the council.
//
// With per-invocation budgets (issue #20) one agent can blow its own window
// while the session stays healthy, so the session context no longer answers
// "did anything time out?". Exit code 4 outranks the partial result it leaves.
func councilDeadline(dispatches []core.DispatchResult, reviews []core.ReviewResult, synth *core.SynthesisResult, sessionErr error) error {
	return firstDeadline(append(councilErrors(dispatches, reviews, synth), sessionErr)...)
}

// councilExit picks the exit code once the council's output has been rendered.
// A timeout anywhere outranks a partial result (precedence 4 > 5).
func councilExit(budget *core.Budget, dispatches []core.DispatchResult, reviews []core.ReviewResult, synth *core.SynthesisResult) error {
	if d := councilDeadline(dispatches, reviews, synth, budget.Session().Err()); d != nil {
		return &ExitError{Code: core.ExitTimeout, Message: timeoutMessage(budget, "", d)}
	}
	// Any stage failing leaves gaps in an otherwise usable answer. Counting
	// only dispatch errors reported a clean 0 for a council that lost a peer
	// review or silently swapped out its chair.
	for _, err := range councilErrors(dispatches, reviews, synth) {
		if err != nil {
			return &ExitError{Code: core.ExitPartial, Message: "partial result: some agents failed"}
		}
	}
	return nil
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

// stderrProgress returns a ProgressFunc that renders agent status to stderr with provider dots.
// The stage parameter controls the label: "dispatch"=no label, "review"=shows "reviewing".
func stderrProgress(stage string, hasColor bool) core.ProgressFunc {
	var mu sync.Mutex
	rc := &output.RenderContext{IsTTY: true, HasColor: hasColor, Width: 60}
	return func(provider string, event core.ProgressEvent) {
		mu.Lock()
		defer mu.Unlock()
		dot := output.ProviderDot(rc, providerColor(provider))
		label := provider
		if stage == "review" {
			label = provider + " reviewing"
		}
		switch event {
		case core.ProgressStarted:
			fmt.Fprintf(os.Stderr, "  %s %s ...\r", dot, label) //nolint:errcheck
		case core.ProgressDone:
			check := output.StatusOK(rc)
			fmt.Fprintf(os.Stderr, "\r\033[K  %s %s %s\n", dot, label, check) //nolint:errcheck
		case core.ProgressFailed:
			cross := output.StatusFail(rc)
			fmt.Fprintf(os.Stderr, "\r\033[K  %s %s %s\n", dot, label, cross) //nolint:errcheck
		}
	}
}

// --- JSON output (council-specific types to avoid collision with review.go) ---

type councilJSON struct {
	Dispatch  []councilDispatchJSON `json:"dispatch"`
	Reviews   []councilReviewJSON   `json:"reviews"`
	Synthesis *councilSynthesisJSON `json:"synthesis"`
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
		out.Synthesis = &councilSynthesisJSON{
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

	markJSONWritten()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("write council json: %w", err)
	}
	return nil
}

// --- TTY output ---

func renderCouncilTTY(rc *output.RenderContext, dispatches []core.DispatchResult, reviews []core.ReviewResult, synth *core.SynthesisResult) {
	// Content separator (between progress stages and content).
	if sep := output.ContentSeparator(rc); sep != "" {
		fmt.Fprintln(os.Stdout)      //nolint:errcheck
		fmt.Fprintln(os.Stdout, sep) //nolint:errcheck
	}

	// Synthesis content.
	if synth != nil {
		fmt.Fprintln(os.Stdout)                //nolint:errcheck
		fmt.Fprintln(os.Stdout, synth.Content) //nolint:errcheck
	}

	// Footer.
	fmt.Fprintln(os.Stdout) //nolint:errcheck
	if sep := output.ContentSeparator(rc); sep != "" {
		fmt.Fprintln(os.Stdout, sep) //nolint:errcheck
	}

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

	// Metrics line.
	parts := []string{fmt.Sprintf("%.1fs", totalDuration.Seconds())}
	if totalCost > 0 {
		parts = append(parts, fmt.Sprintf("$%.3f", totalCost))
	}
	if totalIn > 0 || totalOut > 0 {
		parts = append(parts, fmt.Sprintf("%s in · %s out", fmtTokens(totalIn), fmtTokens(totalOut)))
	}
	fmt.Fprintln(os.Stdout, output.FooterMetrics(rc, parts...)) //nolint:errcheck

	// Agent status line with provider dots.
	var agentParts []string
	for _, d := range dispatches {
		dot := output.ProviderDot(rc, providerColor(d.Provider))
		s := output.StatusOK(rc)
		if d.Error != nil {
			s = output.StatusFail(rc)
		}
		agentParts = append(agentParts, fmt.Sprintf("%s %s %s", dot, d.Provider, s))
	}
	fmt.Fprintln(os.Stdout, "  "+strings.Join(agentParts, " · ")) //nolint:errcheck
}
