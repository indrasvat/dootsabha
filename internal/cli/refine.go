package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/output"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// --- JSON types for refine output ---

type refineVersionJSON struct {
	Version    int      `json:"version"`
	Provider   string   `json:"provider"`
	Content    string   `json:"content"`
	Reviewer   string   `json:"reviewer,omitempty"`
	Review     string   `json:"review,omitempty"`
	DurationMs int64    `json:"duration_ms"`
	CostUSD    *float64 `json:"cost_usd,omitempty"`
	TokensIn   *int     `json:"tokens_in,omitempty"`
	TokensOut  *int     `json:"tokens_out,omitempty"`
}

type refineFinalJSON struct {
	Version int    `json:"version"`
	Content string `json:"content"`
}

type refineMetaJSON struct {
	SchemaVersion  int               `json:"schema_version"`
	Strategy       string            `json:"strategy"`
	Anonymous      bool              `json:"anonymous"`
	DurationMs     int64             `json:"duration_ms"`
	TotalCostUSD   float64           `json:"total_cost_usd"`
	TotalTokensIn  int               `json:"total_tokens_in"`
	TotalTokensOut int               `json:"total_tokens_out"`
	Providers      map[string]string `json:"providers"`
}

type refineJSON struct {
	Versions []refineVersionJSON `json:"versions"`
	Final    refineFinalJSON     `json:"final"`
	Meta     refineMetaJSON      `json:"meta"`
}

func newRefineCmd() *cobra.Command {
	var (
		author       string
		reviewersRaw string
		anonymous    bool
		model        string
	)

	cmd := &cobra.Command{
		Use:     "refine <prompt>",
		Aliases: []string{"sanshodhan", "संशोधन"},
		Short:   "refine (sanshodhan) — Sequential review + incorporation pipeline",
		Long: `Author generates content, reviewers review sequentially, author incorporates feedback.

संशोधन (sanshodhan) — लेखक सामग्री बनाता है, समीक्षक क्रमशः समीक्षा करते हैं, लेखक प्रतिक्रिया शामिल करता है।

Exit codes: 0 success, 1 error, 3 provider error, 4 timeout, 5 partial result`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve bilingual flag aliases.
			if kartaa, _ := cmd.Flags().GetString("kartaa"); kartaa != "" && author == "" {
				author = kartaa
			}
			if pareekshak, _ := cmd.Flags().GetString("pareekshak"); pareekshak != "" && reviewersRaw == "" {
				reviewersRaw = pareekshak
			}
			if gupt, _ := cmd.Flags().GetBool("gupt"); cmd.Flags().Changed("gupt") {
				anonymous = gupt
			}

			prompt := args[0]
			reviewerNames := parseReviewerList(reviewersRaw)

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

			rc := output.NewRenderContext(os.Stdout, jsonOutput)
			invokeOpts := providers.InvokeOptions{
				Model:   model,
				Timeout: timeout,
			}

			// Render header (TTY only, not JSON).
			if rc.IsTTY && !rc.IsJSON() {
				info := fmt.Sprintf("author: %s · reviewers: %s", author, strings.Join(reviewerNames, ", "))
				fmt.Fprintln(os.Stdout, output.CommandHeader(rc, "Refine", info)) //nolint:errcheck
				fmt.Fprintln(os.Stdout)                                           //nolint:errcheck
			}

			totalStart := time.Now()
			providerStatus := map[string]string{author: "ok"}
			var versions []refineVersionJSON
			var totalCost float64
			var totalIn, totalOut int
			partial := false

			// Step 1: Author generates v1.
			slog.Info("refine: author generating v1", "author", author, "reviewers", reviewerNames, "anonymous", anonymous)
			if rc.IsTTY && !rc.IsJSON() {
				stderrRefineStep(rc, author, "v1", true)
			}
			v1Result, err := authorProv.Invoke(ctx, prompt, invokeOpts)
			if err != nil {
				if rc.IsTTY && !rc.IsJSON() {
					stderrRefineStep(rc, author, "v1", false)
				}
				if errors.Is(err, context.DeadlineExceeded) {
					return &ExitError{Code: 4, Message: fmt.Sprintf("timeout after %s: %s", timeout, err)}
				}
				return &ExitError{Code: 3, Message: fmt.Sprintf("author (%s) failed on v1: %s", author, err)}
			}
			if rc.IsTTY && !rc.IsJSON() {
				stderrRefineDone(rc, author, "v1", v1Result.Duration)
			}

			slog.Info("refine: v1 complete", "author", author, "duration", v1Result.Duration,
				"tokens_in", v1Result.TokensIn, "tokens_out", v1Result.TokensOut, "content_len", len(v1Result.Content))
			currentContent := v1Result.Content
			currentVersion := 1
			totalCost += v1Result.CostUSD
			totalIn += v1Result.TokensIn
			totalOut += v1Result.TokensOut

			versions = append(versions, toRefineVersionJSON(1, author, v1Result, "", "", nil))

			// Steps 2..N: reviewer reviews → author incorporates.
			for _, revName := range reviewerNames {
				revProv, revErr := getProvider(revName, cfg, runner)
				if revErr != nil {
					// Unknown reviewer — skip.
					providerStatus[revName] = "error"
					partial = true
					if rc.IsTTY && !rc.IsJSON() {
						stderrRefineSkip(rc, revName, revErr)
					}
					continue
				}

				// Review step.
				slog.Debug("refine: reviewer starting", "reviewer", revName, "version", currentVersion)
				reviewPrompt := buildReviewPrompt(currentContent, author, anonymous)
				if rc.IsTTY && !rc.IsJSON() {
					stderrRefineStep(rc, revName, fmt.Sprintf("reviewing v%d", currentVersion), true)
				}
				reviewResult, revErr := revProv.Invoke(ctx, reviewPrompt, invokeOpts)
				if revErr != nil {
					providerStatus[revName] = "error"
					partial = true
					if rc.IsTTY && !rc.IsJSON() {
						stderrRefineSkip(rc, revName, revErr)
					}
					if errors.Is(revErr, context.DeadlineExceeded) {
						// On timeout, output what we have.
						break
					}
					continue
				}
				if rc.IsTTY && !rc.IsJSON() {
					stderrRefineDone(rc, revName, fmt.Sprintf("reviewing v%d", currentVersion), reviewResult.Duration)
				}
				slog.Info("refine: review complete", "reviewer", revName, "version", currentVersion,
					"duration", reviewResult.Duration, "content_len", len(reviewResult.Content))
				providerStatus[revName] = "ok"
				totalCost += reviewResult.CostUSD
				totalIn += reviewResult.TokensIn
				totalOut += reviewResult.TokensOut

				// Incorporate step.
				slog.Debug("refine: incorporating feedback", "author", author, "reviewer", revName)
				incorporatePrompt := buildIncorporatePrompt(currentContent, reviewResult.Content, revName, anonymous)
				if rc.IsTTY && !rc.IsJSON() {
					nextVersion := currentVersion + 1
					stderrRefineStep(rc, author, fmt.Sprintf("incorporating → v%d", nextVersion), true)
				}
				incResult, incErr := authorProv.Invoke(ctx, incorporatePrompt, invokeOpts)
				if incErr != nil {
					// Author failed to incorporate — keep current version.
					partial = true
					if rc.IsTTY && !rc.IsJSON() {
						dot := output.ProviderDot(rc, providerColor(author))
						fmt.Fprintf(os.Stderr, "\r\033[K  %s %-8s incorporating failed, keeping v%d\n", dot, author, currentVersion) //nolint:errcheck
					}
					// Still record the review in versions.
					versions = append(versions, toRefineVersionJSON(
						currentVersion, author, nil, revName, reviewResult.Content,
						&providers.ProviderResult{Duration: reviewResult.Duration},
					))
					if errors.Is(incErr, context.DeadlineExceeded) {
						break
					}
					continue
				}
				currentVersion++
				currentContent = incResult.Content
				totalCost += incResult.CostUSD
				totalIn += incResult.TokensIn
				totalOut += incResult.TokensOut

				if rc.IsTTY && !rc.IsJSON() {
					stderrRefineDone(rc, author, fmt.Sprintf("incorporating → v%d", currentVersion), incResult.Duration)
				}

				// Record version with review info.
				vj := toRefineVersionJSON(currentVersion, author, incResult, revName, reviewResult.Content, nil)
				vj.DurationMs = (reviewResult.Duration + incResult.Duration).Milliseconds()
				if reviewResult.CostUSD+incResult.CostUSD > 0 {
					c := reviewResult.CostUSD + incResult.CostUSD
					vj.CostUSD = &c
				}
				tIn := reviewResult.TokensIn + incResult.TokensIn
				if tIn > 0 {
					vj.TokensIn = &tIn
				}
				tOut := reviewResult.TokensOut + incResult.TokensOut
				if tOut > 0 {
					vj.TokensOut = &tOut
				}
				versions = append(versions, vj)
			}

			totalDuration := time.Since(totalStart)

			// Output.
			if rc.IsJSON() {
				return renderRefineJSON(versions, currentVersion, currentContent, anonymous, totalDuration, totalCost, totalIn, totalOut, providerStatus)
			}

			renderRefineTTY(rc, currentContent, currentVersion, len(reviewerNames), author, totalDuration, totalCost, totalIn, totalOut)

			if partial {
				return &ExitError{Code: 5, Message: "partial result: one or more reviewers failed"}
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&author, "author", "claude", "Agent that produces and refines content")
	f.String("kartaa", "", "Alias for --author (कर्ता)")
	_ = f.MarkHidden("kartaa")
	f.StringVar(&reviewersRaw, "reviewers", "codex,gemini", "Ordered comma-separated reviewer list")
	f.String("pareekshak", "", "Alias for --reviewers (परीक्षक)")
	_ = f.MarkHidden("pareekshak")
	f.BoolVar(&anonymous, "anonymous", true, "Anonymize prompts (reviewer doesn't see author name)")
	f.Bool("gupt", true, "Alias for --anonymous (गुप्त)")
	_ = f.MarkHidden("gupt")
	f.StringVar(&model, "model", "", "Override model for all agents")

	return cmd
}

// parseReviewerList splits a comma-separated reviewer string into a slice.
func parseReviewerList(raw string) []string {
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// buildReviewPrompt constructs the review prompt for a reviewer.
func buildReviewPrompt(content, authorName string, anonymous bool) string {
	truncated := core.TruncateString(content, 32*1024)
	if anonymous {
		return fmt.Sprintf(
			"Review the following content. Identify strengths, weaknesses, factual errors, and areas for improvement. Be specific and actionable.\n\n%s",
			truncated,
		)
	}
	return fmt.Sprintf(
		"Review the following output from %s. Identify strengths, weaknesses, factual errors, and areas for improvement. Be specific and actionable.\n\n%s",
		authorName, truncated,
	)
}

// buildIncorporatePrompt constructs the incorporation prompt for the author.
func buildIncorporatePrompt(currentContent, review, reviewerName string, anonymous bool) string {
	truncatedContent := core.TruncateString(currentContent, 32*1024)
	truncatedReview := core.TruncateString(review, 32*1024)
	if anonymous {
		return fmt.Sprintf(
			"You previously wrote the following content:\n\n%s\n\nA reviewer provided this feedback:\n\n%s\n\nProduce an improved version that incorporates the valid feedback. Output only the improved content, not commentary.",
			truncatedContent, truncatedReview,
		)
	}
	return fmt.Sprintf(
		"You previously wrote the following content:\n\n%s\n\n%s provided this feedback:\n\n%s\n\nProduce an improved version that incorporates the valid feedback. Output only the improved content, not commentary.",
		truncatedContent, reviewerName, truncatedReview,
	)
}

// --- TTY rendering ---

func stderrRefineStep(rc *output.RenderContext, provider, label string, started bool) {
	if !started {
		return
	}
	dot := output.ProviderDot(rc, providerColor(provider))
	fmt.Fprintf(os.Stderr, "  %s %-8s %s ...\r", dot, provider, label) //nolint:errcheck
}

func stderrRefineDone(rc *output.RenderContext, provider, label string, d time.Duration) {
	dot := output.ProviderDot(rc, providerColor(provider))
	check := output.StatusOK(rc)
	fmt.Fprintf(os.Stderr, "\r\033[K  %s %-8s %-36s %5.1fs %s\n", dot, provider, label, d.Seconds(), check) //nolint:errcheck
}

func stderrRefineSkip(rc *output.RenderContext, provider string, err error) {
	dot := output.ProviderDot(rc, providerColor(provider))
	fmt.Fprintf(os.Stderr, "\r\033[K  %s %-8s skipped: %s\n", dot, provider, err) //nolint:errcheck
}

func renderRefineTTY(rc *output.RenderContext, finalContent string, finalVersion, reviewCount int, authorName string, totalDuration time.Duration, totalCost float64, totalIn, totalOut int) {
	// Content separator (between progress and content).
	if sep := output.ContentSeparator(rc); sep != "" {
		fmt.Fprintln(os.Stdout)      //nolint:errcheck
		fmt.Fprintln(os.Stdout, sep) //nolint:errcheck
	}

	// Final content.
	fmt.Fprintf(os.Stdout, "\n%s\n", finalContent) //nolint:errcheck

	if rc.IsTTY {
		// Footer separator.
		fmt.Fprintln(os.Stdout, output.ContentSeparator(rc)) //nolint:errcheck

		// Metrics.
		parts := []string{fmt.Sprintf("%.1fs", totalDuration.Seconds())}
		if totalCost > 0 {
			parts = append(parts, fmt.Sprintf("$%.3f", totalCost))
		}
		if totalIn > 0 || totalOut > 0 {
			parts = append(parts, fmt.Sprintf("%s in · %s out", fmtTokens(totalIn), fmtTokens(totalOut)))
		}
		fmt.Fprintln(os.Stdout, output.FooterMetrics(rc, parts...)) //nolint:errcheck

		// Version/review summary.
		parts2 := []string{
			fmt.Sprintf("versions: %d", finalVersion),
			fmt.Sprintf("reviews: %d", reviewCount),
			fmt.Sprintf("author: %s ✓", authorName),
		}
		fmt.Fprintln(os.Stdout, output.FooterMetrics(rc, parts2...)) //nolint:errcheck
	}
}

// --- JSON rendering ---

func toRefineVersionJSON(version int, provider string, result *providers.ProviderResult, reviewer, review string, _ *providers.ProviderResult) refineVersionJSON {
	vj := refineVersionJSON{
		Version:  version,
		Provider: provider,
		Reviewer: reviewer,
		Review:   review,
	}
	if result != nil {
		vj.Content = result.Content
		vj.DurationMs = result.Duration.Milliseconds()
		if result.CostUSD > 0 {
			cost := result.CostUSD
			vj.CostUSD = &cost
		}
		if result.TokensIn > 0 {
			t := result.TokensIn
			vj.TokensIn = &t
		}
		if result.TokensOut > 0 {
			t := result.TokensOut
			vj.TokensOut = &t
		}
	}
	return vj
}

func renderRefineJSON(versions []refineVersionJSON, finalVersion int, finalContent string, anonymous bool, totalDuration time.Duration, totalCost float64, totalIn, totalOut int, providerStatus map[string]string) error {
	data := refineJSON{
		Versions: versions,
		Final: refineFinalJSON{
			Version: finalVersion,
			Content: finalContent,
		},
		Meta: refineMetaJSON{
			SchemaVersion:  1,
			Strategy:       "refine",
			Anonymous:      anonymous,
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
