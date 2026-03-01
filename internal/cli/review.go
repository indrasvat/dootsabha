package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/output"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// reviewJSON is the top-level JSON output for the review command.
type reviewJSON struct {
	Author reviewAgentJSON `json:"author"`
	Review reviewAgentJSON `json:"review"`
	Meta   reviewMeta      `json:"meta"`
}

// reviewAgentJSON holds per-agent results in JSON output.
type reviewAgentJSON struct {
	Provider   string   `json:"provider"`
	Model      string   `json:"model"`
	Content    string   `json:"content"`
	DurationMs int64    `json:"duration_ms"`
	CostUSD    *float64 `json:"cost_usd"`
	TokensIn   *int     `json:"tokens_in"`
	TokensOut  *int     `json:"tokens_out"`
}

// reviewMeta holds aggregate metadata for the review pipeline.
type reviewMeta struct {
	SchemaVersion  int               `json:"schema_version"`
	Strategy       string            `json:"strategy"`
	DurationMs     int64             `json:"duration_ms"`
	TotalCostUSD   float64           `json:"total_cost_usd"`
	TotalTokensIn  int               `json:"total_tokens_in"`
	TotalTokensOut int               `json:"total_tokens_out"`
	Providers      map[string]string `json:"providers"`
}

func newReviewCmd() *cobra.Command {
	var (
		author   string
		reviewer string
		model    string
	)

	cmd := &cobra.Command{
		Use:     "review <prompt>",
		Aliases: []string{"sameeksha", "समीक्षा"},
		Short:   "review (sameeksha) — Author + reviewer pipeline",
		Long: `One agent produces output, another reviews it.

समीक्षा (sameeksha) — एक एजेंट सामग्री बनाता है, दूसरा उसकी समीक्षा करता है।

Exit codes: 0 success, 1 error, 3 provider error, 4 timeout, 5 config error`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve bilingual flag aliases (Spike 0.6 Finding 6).
			if kartaa, _ := cmd.Flags().GetString("kartaa"); kartaa != "" && author == "" {
				author = kartaa
			}
			if pareekshak, _ := cmd.Flags().GetString("pareekshak"); pareekshak != "" && reviewer == "" {
				reviewer = pareekshak
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

			authorProv, err := getProvider(author, cfg, runner)
			if err != nil {
				return &ExitError{Code: 1, Message: err.Error()}
			}
			reviewerProv, err := getProvider(reviewer, cfg, runner)
			if err != nil {
				return &ExitError{Code: 1, Message: err.Error()}
			}

			rc := output.NewRenderContext(os.Stdout, jsonOutput)
			invokeOpts := providers.InvokeOptions{
				Model:   model,
				Timeout: timeout,
			}

			// Step 1: Invoke author.
			totalStart := time.Now()
			authorResult, err := authorProv.Invoke(ctx, prompt, invokeOpts)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return &ExitError{Code: 4, Message: fmt.Sprintf("timeout after %s: %s", timeout, err)}
				}
				return &ExitError{Code: 3, Message: fmt.Sprintf("author (%s) failed: %s", author, err)}
			}

			// Step 2: Construct review prompt and invoke reviewer.
			reviewPrompt := fmt.Sprintf(
				"Review the following output from %s. Identify strengths, weaknesses, errors. Be specific.\n\n%s",
				author, authorResult.Content,
			)
			reviewerResult, err := reviewerProv.Invoke(ctx, reviewPrompt, invokeOpts)
			totalDuration := time.Since(totalStart)

			if err != nil {
				// Reviewer failed — return author result with error.
				if rc.IsJSON() {
					_ = renderReviewJSON(authorResult, nil, author, reviewer, totalDuration)
				} else {
					renderReviewSection(rc, author, "(author)", authorResult)
				}
				if errors.Is(err, context.DeadlineExceeded) {
					return &ExitError{Code: 4, Message: fmt.Sprintf("timeout after %s: %s", timeout, err)}
				}
				return &ExitError{Code: 3, Message: fmt.Sprintf("reviewer (%s) failed: %s", reviewer, err)}
			}

			// Render output.
			if rc.IsJSON() {
				return renderReviewJSON(authorResult, reviewerResult, author, reviewer, totalDuration)
			}

			renderReviewTTY(rc, author, reviewer, authorResult, reviewerResult, totalDuration)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&author, "author", "codex", "Agent that produces initial output")
	f.String("kartaa", "", "Alias for --author (कर्ता)")
	_ = f.MarkHidden("kartaa")
	f.StringVar(&reviewer, "reviewer", "claude", "Agent that reviews the output")
	f.String("pareekshak", "", "Alias for --reviewer (परीक्षक)")
	_ = f.MarkHidden("pareekshak")
	f.StringVar(&model, "model", "", "Override model for both agents")

	return cmd
}

// toAgentJSON converts a ProviderResult to JSON output, handling nil results.
func toAgentJSON(name string, r *providers.ProviderResult) reviewAgentJSON {
	if r == nil {
		return reviewAgentJSON{Provider: name}
	}
	j := reviewAgentJSON{
		Provider:   name,
		Model:      r.Model,
		Content:    r.Content,
		DurationMs: r.Duration.Milliseconds(),
	}
	if r.CostUSD > 0 {
		cost := r.CostUSD
		j.CostUSD = &cost
	}
	if r.TokensIn > 0 {
		t := r.TokensIn
		j.TokensIn = &t
	}
	if r.TokensOut > 0 {
		t := r.TokensOut
		j.TokensOut = &t
	}
	return j
}

// renderReviewJSON writes the review result as JSON to stdout.
func renderReviewJSON(authorResult, reviewerResult *providers.ProviderResult, authorName, reviewerName string, totalDuration time.Duration) error {
	providerStatus := map[string]string{authorName: "ok"}
	if reviewerResult != nil {
		providerStatus[reviewerName] = "ok"
	} else {
		providerStatus[reviewerName] = "error"
	}

	var totalCost float64
	var totalIn, totalOut int
	if authorResult != nil {
		totalCost += authorResult.CostUSD
		totalIn += authorResult.TokensIn
		totalOut += authorResult.TokensOut
	}
	if reviewerResult != nil {
		totalCost += reviewerResult.CostUSD
		totalIn += reviewerResult.TokensIn
		totalOut += reviewerResult.TokensOut
	}

	data := reviewJSON{
		Author: toAgentJSON(authorName, authorResult),
		Review: toAgentJSON(reviewerName, reviewerResult),
		Meta: reviewMeta{
			SchemaVersion:  1,
			Strategy:       "review",
			DurationMs:     totalDuration.Milliseconds(),
			TotalCostUSD:   totalCost,
			TotalTokensIn:  totalIn,
			TotalTokensOut: totalOut,
			Providers:      providerStatus,
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

// renderReviewSection renders a single agent's output section.
func renderReviewSection(rc *output.RenderContext, provName, role string, result *providers.ProviderResult) {
	if rc.IsTTY {
		dot := output.ProviderDot(rc, providerColor(provName))
		fmt.Fprintf(os.Stdout, "%s %s   %s\n\n", dot, provName, role) //nolint:errcheck
	}
	fmt.Fprintln(os.Stdout, result.Content) //nolint:errcheck
}

// renderReviewTTY renders the full review pipeline output to the terminal.
func renderReviewTTY(rc *output.RenderContext, authorName, reviewerName string, authorResult, reviewerResult *providers.ProviderResult, totalDuration time.Duration) {
	renderReviewSection(rc, authorName, "(author)", authorResult)

	if rc.IsTTY {
		fmt.Fprintln(os.Stdout) //nolint:errcheck
	}

	renderReviewSection(rc, reviewerName, "(reviewer)", reviewerResult)

	if rc.IsTTY {
		// Footer with separator and totals.
		sep := strings.Repeat("─", min(rc.Width, 60))
		muted := lipgloss.NewStyle().Foreground(output.MutedColor)
		fmt.Fprintln(os.Stdout, output.Styled(rc, muted, sep)) //nolint:errcheck

		totalCost := authorResult.CostUSD + reviewerResult.CostUSD
		totalIn := authorResult.TokensIn + reviewerResult.TokensIn
		totalOut := authorResult.TokensOut + reviewerResult.TokensOut

		footer := fmt.Sprintf("total: %.1fs", totalDuration.Seconds())
		if totalCost > 0 {
			footer += fmt.Sprintf(" │ cost: $%.3f", totalCost)
		}
		if totalIn > 0 || totalOut > 0 {
			footer += fmt.Sprintf(" │ tokens: %s in · %s out", fmtTokens(totalIn), fmtTokens(totalOut))
		}
		fmt.Fprintln(os.Stdout, output.Styled(rc, muted, footer)) //nolint:errcheck
	}
}

// fmtTokens formats a token count with comma separators.
func fmtTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}
