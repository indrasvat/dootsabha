package cli

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/output"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// healthRow aggregates a single provider's health result for rendering.
type healthRow struct {
	Name    string
	Healthy bool
	Version string
	Model   string
	Auth    string
	Error   string
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"sthiti", "स्थिति"},
		Short:   "status (sthiti) — Show agent health and config",
		Long: `Show the health status of all configured AI agents.

स्थिति (sthiti) — सभी AI एजेंटों की स्थिति दिखाएं।

Exit codes: 0 all healthy, 1 error, 3 one or more providers unhealthy`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := core.LoadConfig(configFile)
			if err != nil {
				return &ExitError{Code: 5, Message: fmt.Sprintf("load config: %s", err)}
			}

			timeout := globalTimeout
			if timeout == 0 {
				timeout = cfg.Timeout
			}
			if timeout == 0 {
				timeout = 30 * 1_000_000_000 // 30 seconds for health checks
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			runner := &core.SubprocessRunner{}
			rows := collectHealthRows(ctx, cfg, runner)

			rc := output.NewRenderContext(os.Stdout, jsonOutput)

			if rc.IsJSON() {
				return output.WriteJSON(os.Stdout, rows)
			}

			renderStatusTable(rc, rows)

			// Exit 3 if any provider is unhealthy.
			for _, r := range rows {
				if !r.Healthy {
					return &ExitError{Code: 3, Message: "one or more providers are unhealthy"}
				}
			}
			return nil
		},
	}
}

// collectHealthRows runs HealthCheck for each known provider in cfg and returns
// results in deterministic order (sorted by provider name).
func collectHealthRows(ctx context.Context, cfg *core.Config, runner providers.Runner) []healthRow {
	// Collect provider names in sorted order for deterministic output.
	names := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]healthRow, 0, len(names))
	for _, name := range names {
		prov, err := getProvider(name, cfg, runner)
		if err != nil {
			// Provider name is in config but not a known built-in.
			rows = append(rows, healthRow{
				Name:  name,
				Error: fmt.Sprintf("unknown provider type: %s", name),
			})
			continue
		}

		status, err := prov.HealthCheck(ctx)
		if err != nil {
			rows = append(rows, healthRow{
				Name:  name,
				Error: err.Error(),
			})
			continue
		}

		authStr := "—"
		if status.AuthValid {
			authStr = "✓"
		}

		rows = append(rows, healthRow{
			Name:    name,
			Healthy: status.Healthy,
			Version: status.CLIVersion,
			Model:   status.Model,
			Auth:    authStr,
			Error:   status.Error,
		})
	}
	return rows
}

// renderStatusTable writes the health table to stdout using the output package helpers.
//
// TTY + color: lipgloss bordered table with provider dots and semantic colors.
// TTY + NO_COLOR: plain lipgloss table, no color.
// Piped: tab-separated rows, no ANSI.
func renderStatusTable(rc *output.RenderContext, rows []healthRow) {
	tbl := output.NewTable(rc).
		Headers("PROVIDER", "VERSION", "MODEL", "AUTH", "STATUS")

	for _, r := range rows {
		dot := output.ProviderDot(rc, providerColor(r.Name))
		name := dot + " " + r.Name

		status := output.StatusOK(rc)
		if !r.Healthy {
			status = output.StatusFail(rc)
			if r.Error != "" {
				status += " " + r.Error
			}
		}

		tbl.Row(name, r.Version, r.Model, r.Auth, status)
	}

	tbl.Render(os.Stdout)
}
