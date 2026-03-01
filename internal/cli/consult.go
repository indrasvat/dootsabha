package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/output"
	"github.com/indrasvat/dootsabha/internal/providers"
)

func newConsultCmd() *cobra.Command {
	var (
		agent    string
		model    string
		maxTurns int
	)

	cmd := &cobra.Command{
		Use:     "consult <prompt>",
		Aliases: []string{"paraamarsh", "परामर्श"},
		Short:   "consult (paraamarsh) — Query a single agent",
		Long: `Query a single AI agent with a prompt and receive its response.

परामर्श (paraamarsh) — एकल AI एजेंट से परामर्श करें।

Exit codes: 0 success, 1 error, 3 provider error, 4 timeout, 5 config error`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve bilingual flag aliases (Spike 0.6 Finding 6).
			if doota, _ := cmd.Flags().GetString("doota"); doota != "" && agent == "" {
				agent = doota
			}
			if pratyaya, _ := cmd.Flags().GetString("pratyaya"); pratyaya != "" && model == "" {
				model = pratyaya
			}

			if agent == "" {
				return &ExitError{Code: 1, Message: "--agent (or --doota) is required"}
			}

			prompt := args[0]

			cfg, err := core.LoadConfig(configFile)
			if err != nil {
				return &ExitError{Code: 5, Message: fmt.Sprintf("load config: %s", err)}
			}

			timeout := globalTimeout
			if timeout == 0 {
				timeout = cfg.Timeout
			}
			if timeout == 0 {
				timeout = 5 * 60 * 1_000_000_000 // 5 minutes in nanoseconds
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			runner := &core.SubprocessRunner{}
			prov, err := getProvider(agent, cfg, runner)
			if err != nil {
				return &ExitError{Code: 1, Message: err.Error()}
			}

			rc := output.NewRenderContext(os.Stdout, jsonOutput)

			result, err := prov.Invoke(ctx, prompt, providers.InvokeOptions{
				Model:    model,
				MaxTurns: maxTurns,
				Timeout:  timeout,
			})
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return &ExitError{Code: 4, Message: fmt.Sprintf("timeout after %s: %s", timeout, err)}
				}
				return &ExitError{Code: 3, Message: fmt.Sprintf("provider error: %s", err)}
			}

			if rc.IsJSON() {
				return output.WriteJSON(os.Stdout, result)
			}

			renderConsultResult(rc, prov.Name(), result)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&agent, "agent", "a", "", "Agent to query: claude, codex, gemini")
	f.String("doota", "", "Alias for --agent (दूत)")
	_ = cmd.Flags().MarkHidden("doota")
	f.StringVar(&model, "model", "", "Override model for this invocation")
	f.String("pratyaya", "", "Alias for --model (प्रत्यय)")
	_ = cmd.Flags().MarkHidden("pratyaya")
	f.IntVar(&maxTurns, "max-turns", 0, "Maximum agent turns (0 = no limit)")

	return cmd
}

// getProvider constructs the named provider backed by cfg and runner.
func getProvider(name string, cfg *core.Config, runner providers.Runner) (providers.Provider, error) {
	switch name {
	case "claude":
		return providers.NewClaudeProvider(cfg, runner), nil
	case "codex":
		return providers.NewCodexProvider(cfg, runner), nil
	case "gemini":
		return providers.NewGeminiProvider(cfg, runner), nil
	default:
		return nil, fmt.Errorf("unknown provider: %q — valid values: claude, codex, gemini", name)
	}
}

// providerColor returns the lipgloss color for a given provider name.
func providerColor(name string) lipgloss.Color {
	switch name {
	case "claude":
		return output.ClaudeColor
	case "codex":
		return output.CodexColor
	case "gemini":
		return output.GeminiColor
	default:
		return output.MutedColor
	}
}

// renderConsultResult formats a ProviderResult to stdout.
//
// TTY + color: provider dot + name header, content body, muted footer.
// TTY + NO_COLOR: same layout, no ANSI.
// Piped: content only, no decoration.
func renderConsultResult(rc *output.RenderContext, provName string, result *providers.ProviderResult) {
	dot := output.ProviderDot(rc, providerColor(provName))

	if rc.IsTTY {
		fmt.Fprintf(os.Stdout, "%s %s\n\n", dot, provName) //nolint:errcheck
	}

	fmt.Fprintln(os.Stdout, result.Content) //nolint:errcheck

	if rc.IsTTY {
		footer := fmt.Sprintf("  %.1fs", result.Duration.Seconds())
		if result.CostUSD > 0 {
			footer += fmt.Sprintf("  $%.4f", result.CostUSD)
		}
		if result.Model != "" {
			footer += fmt.Sprintf("  %s", result.Model)
		}
		muted := lipgloss.NewStyle().Foreground(output.MutedColor)
		fmt.Fprintln(os.Stdout, output.Styled(rc, muted, footer)) //nolint:errcheck
	}
}
