package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	// Installed reports whether the provider's binary was resolvable on $PATH.
	// It distinguishes "you never installed this" from "it is installed but
	// broken" — only the latter is a problem for an opt-in provider.
	Installed bool
}

// optionalProviders are opt-in agents: they are listed by `status` for
// discoverability but belong to no default pipeline, so a user who never
// installed one must not be told their setup is unhealthy.
//
// Promoting a provider to a council default means removing it from this set.
var optionalProviders = map[string]bool{
	"grok": true,
}

func isOptionalProvider(name string) bool { return optionalProviders[name] }

// statusLabelPlain is the un-styled STATUS cell for a row. Kept separate from
// rendering so the wording is testable without a RenderContext.
func statusLabelPlain(r healthRow) string {
	switch {
	case r.Healthy:
		return "OK"
	case isOptionalProvider(r.Name) && !r.Installed:
		return "not installed (optional)"
	default:
		if r.Error != "" {
			return "FAIL " + r.Error
		}
		return "FAIL"
	}
}

// statusExitError applies the exit-code rule for `dootsabha status`, using the
// same contract as every other command:
//
//	0  every agent the user cares about is usable
//	5  degraded — some are broken, but at least one still works
//	3  nothing is usable
//
// A provider counts as a *problem* when it is unhealthy and either required, or
// installed (so the user clearly meant to use it). An opt-in provider that was
// never installed is informational and never degrades the result — but it also
// cannot make the setup usable, which is why "nothing healthy" is judged
// separately.
func statusExitError(rows []healthRow) error {
	healthy, problems := 0, 0
	for _, r := range rows {
		switch {
		case r.Healthy:
			healthy++
		case isOptionalProvider(r.Name) && !r.Installed:
			// opt-in and never installed — not the user's problem
		default:
			problems++
		}
	}

	switch {
	case healthy == 0:
		// 3 means "nothing usable" — true even if the only entries were opt-in
		// providers that were never installed.
		return &ExitError{Code: core.ExitProvider, Message: "no usable providers"}
	case problems > 0:
		return &ExitError{Code: core.ExitPartial, Message: "one or more providers are unhealthy"}
	default:
		return nil
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"sthiti", "स्थिति"},
		Short:   "status (sthiti) — Show agent health and config",
		Long: `Show the health status of all configured AI agents.

स्थिति (sthiti) — सभी AI एजेंटों की स्थिति दिखाएं।

Exit codes: 0 all usable, 5 degraded (some broken, others work), 3 nothing usable,
2 bad command, 6 config error. An opt-in provider that is simply not installed is
reported but does not degrade the result.`,
		Args:         usageArgs(cobra.NoArgs),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := core.LoadConfig(configFile)
			if err != nil {
				return &ExitError{Code: core.ExitConfig, Message: fmt.Sprintf("load config: %s", err)}
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
				if err := emitJSON(rows); err != nil {
					return &ExitError{Code: core.ExitError, Message: err.Error()}
				}
			} else {
				renderStatusTable(rc, rows)
			}

			// The exit code is the same in both modes. --json is the mode agents
			// are told to use, so it must not be the one that under-reports.
			return statusExitError(rows)
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
		installed := providerInstalled(cfg, name)

		prov, err := getProvider(name, cfg, runner)
		if err != nil {
			// Provider name is in config but not a known built-in. This is a
			// config error regardless of installation, so Installed stays true
			// to keep it failing the status check.
			rows = append(rows, healthRow{
				Name:      name,
				Error:     fmt.Sprintf("unknown provider type: %s", name),
				Installed: true,
			})
			continue
		}

		status, err := prov.HealthCheck(ctx)
		if err != nil {
			rows = append(rows, healthRow{
				Name:      name,
				Error:     err.Error(),
				Installed: installed,
			})
			continue
		}

		authStr := "—"
		if status.AuthValid {
			authStr = "✓"
		}

		rows = append(rows, healthRow{
			Name:      name,
			Healthy:   status.Healthy,
			Version:   status.CLIVersion,
			Model:     status.Model,
			Auth:      authStr,
			Error:     status.Error,
			Installed: installed,
		})
	}
	return rows
}

// providerInstalled reports whether the provider's configured binary resolves on
// $PATH (or as an explicit path). Used to tell "never installed" apart from
// "installed but broken".
func providerInstalled(cfg *core.Config, name string) bool {
	binary := name
	if cfg != nil {
		if pc, ok := cfg.Providers[name]; ok && pc.Binary != "" {
			binary = pc.Binary
		}
	}
	_, err := exec.LookPath(binary)
	return err == nil
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
		switch {
		case r.Healthy:
			// keep OK
		case isOptionalProvider(r.Name) && !r.Installed:
			// Opt-in and never installed: nothing is broken, so don't shout FAIL
			// or dump a fork/exec error at the user.
			status = "not installed (optional)"
		default:
			status = output.StatusFail(rc)
			if r.Error != "" {
				status += " " + r.Error
			}
		}

		tbl.Row(name, r.Version, r.Model, r.Auth, status)
	}

	tbl.Render(os.Stdout)
}
