package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/output"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"vinyaas", "विन्यास"},
		Short:   "config (vinyaas) — Manage दूतसभा configuration",
		Long: `View and manage दूतसभा configuration.

विन्यास (vinyaas) — दूतसभा विन्यास प्रबंधित करें।`,
		Args:         usageArgs(cobra.NoArgs),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigMigrateCmd())
	return cmd
}

// newConfigMigrateCmd migrates a config file off the retired gemini provider.
func newConfigMigrateCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "migrate",
		Aliases: []string{"sthaanaantaran", "स्थानांतरण"},
		Short:   "Migrate config off the retired gemini provider to Antigravity (agy)",
		Long: `Detect and rewrite gemini references in your dootsabha config.

Google is retiring the Gemini CLI on 2026-06-18; the Antigravity CLI (agy) replaces it.
This rewrites providers.gemini → providers.agy and council.chair: gemini → agy,
backing up the original before any change. The gemini provider's binary/model/flags
do not carry over to agy and are replaced with agy defaults (the backup preserves them).

स्थानांतरण (sthaanaantaran) — gemini से agy में विन्यास स्थानांतरित करें।`,
		Args:         usageArgs(cobra.NoArgs),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the bilingual --pareekshan alias for --dry-run.
			if p, _ := cmd.Flags().GetBool("pareekshan"); p {
				dryRun = true
			}

			explicit := configFile != ""
			path := configFile
			if path == "" {
				p, err := core.UserConfigPath()
				if err != nil {
					return &ExitError{Code: 1, Message: err.Error()}
				}
				path = p
			}

			rc := output.NewRenderContext(os.Stdout, jsonOutput)

			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					// An explicitly-named --config that doesn't exist is an error;
					// a missing default user file just means there's nothing to migrate.
					if explicit {
						return &ExitError{Code: core.ExitConfig, Message: fmt.Sprintf("config file not found: %s", path)}
					}
					return emitMigrateResult(rc, migrateResultView{
						Path:    path,
						Status:  "no-config",
						Message: "no config file found — nothing to migrate (defaults already use agy)",
					})
				}
				return &ExitError{Code: core.ExitConfig, Message: fmt.Sprintf("stat config: %s", err)}
			}

			if dryRun {
				// Plan against the raw file (not the merged/env config) so the preview
				// matches what an actual migration would do.
				plan, err := core.PlanMigrationFile(path)
				if err != nil {
					return &ExitError{Code: core.ExitConfig, Message: fmt.Sprintf("inspect config: %s", err)}
				}
				view := migrateResultView{
					Path:            path,
					DryRun:          true,
					ProviderRenamed: plan.ProviderRenamed,
					ChairUpdated:    plan.ChairUpdated,
				}
				if plan.Changed() {
					view.Status = "needs-migration"
					view.Message = "gemini references found — run `dootsabha config migrate` to fix"
				} else {
					view.Status = "already-migrated"
					view.Message = "no gemini references found — nothing to migrate"
				}
				return emitMigrateResult(rc, view)
			}

			res, err := core.MigrateConfigFile(path)
			if err != nil {
				return &ExitError{Code: core.ExitConfig, Message: fmt.Sprintf("migrate config: %s", err)}
			}

			view := migrateResultView{
				Path:            res.Path,
				BackupPath:      res.BackupPath,
				ProviderRenamed: res.ProviderRenamed,
				ChairUpdated:    res.ChairUpdated,
			}
			if res.Changed() {
				view.Status = "migrated"
				view.Message = "migrated gemini → agy (Antigravity)"
			} else {
				view.Status = "already-migrated"
				view.Message = "no gemini references found — nothing to migrate"
			}
			return emitMigrateResult(rc, view)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&dryRun, "dry-run", false, "Report what would change without modifying the file")
	f.Bool("pareekshan", false, "Alias for --dry-run (परीक्षण)")
	_ = f.MarkHidden("pareekshan")

	return cmd
}

// migrateResultView is the rendered/JSON shape of a migration outcome.
type migrateResultView struct {
	Path            string `json:"path"`
	BackupPath      string `json:"backup_path,omitempty"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	ProviderRenamed bool   `json:"provider_renamed,omitempty"`
	ChairUpdated    bool   `json:"chair_updated,omitempty"`
	DryRun          bool   `json:"dry_run,omitempty"`
}

func emitMigrateResult(rc *output.RenderContext, v migrateResultView) error {
	if rc.IsJSON() {
		return emitJSON(v)
	}

	switch v.Status {
	case "migrated":
		fmt.Fprintf(os.Stdout, "%s %s\n", output.StatusOK(rc), v.Message) //nolint:errcheck
		printMigrationChanges(v)
		fmt.Fprintf(os.Stdout, "  · prior gemini settings preserved in %s\n", v.BackupPath) //nolint:errcheck
		fmt.Fprintf(os.Stdout, "  · config: %s\n", v.Path)                                  //nolint:errcheck
	case "needs-migration":
		fmt.Fprintf(os.Stdout, "%s\n", v.Message)            //nolint:errcheck
		fmt.Fprintln(os.Stdout, "  would change (dry-run):") //nolint:errcheck
		printMigrationChanges(v)
		fmt.Fprintf(os.Stdout, "  · config: %s\n", v.Path) //nolint:errcheck
	default:
		fmt.Fprintf(os.Stdout, "%s\n", v.Message) //nolint:errcheck
		if v.Path != "" {
			fmt.Fprintf(os.Stdout, "  · config: %s\n", v.Path) //nolint:errcheck
		}
	}
	return nil
}

// printMigrationChanges lists the per-key changes (applied or planned).
func printMigrationChanges(v migrateResultView) {
	if v.ProviderRenamed {
		fmt.Fprintln(os.Stdout, "  · providers.gemini → providers.agy") //nolint:errcheck
	}
	if v.ChairUpdated {
		fmt.Fprintln(os.Stdout, "  · council.chair: gemini → agy") //nolint:errcheck
	}
}

func newConfigShowCmd() *cobra.Command {
	var (
		showJSON     bool
		showComments bool
		reveal       bool
	)

	cmd := &cobra.Command{
		Use:          "show",
		Short:        "Display merged configuration (with redaction by default)",
		Args:         usageArgs(cobra.NoArgs),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := core.LoadConfig(configFile)
			if err != nil {
				return &ExitError{Code: core.ExitConfig, Message: fmt.Sprintf("load config: %s", err)}
			}

			view := cfg.RedactedView(reveal)

			useJSON := showJSON || jsonOutput
			rc := output.NewRenderContext(os.Stdout, useJSON)

			if rc.IsJSON() || showJSON {
				return emitJSON(view)
			}

			renderConfigView(view, showComments)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&showJSON, "json", false, "Output as JSON")
	f.BoolVar(&showComments, "commented", false, "Include field descriptions as comments")
	f.BoolVar(&reveal, "reveal", false, "Reveal sensitive values (disables redaction)")

	return cmd
}

// renderConfigView prints the config map as indented key=value lines.
// Keys are sorted for deterministic output.
func renderConfigView(view map[string]any, withComments bool) {
	printMap(view, "", withComments)
}

// printMap recursively prints map entries with dot-separated key paths.
func printMap(m map[string]any, prefix string, withComments bool) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := m[k]
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		switch nested := v.(type) {
		case map[string]any:
			printMap(nested, fullKey, withComments)
		default:
			// Format slices as JSON for readability.
			formatted, err := json.Marshal(v)
			if err != nil {
				formatted = fmt.Appendf(nil, "%v", v)
			}
			if withComments {
				if comment, ok := core.ConfigComments[fullKey]; ok {
					fmt.Fprintf(os.Stdout, "%s = %s  # %s\n", fullKey, formatted, comment) //nolint:errcheck
				} else {
					fmt.Fprintf(os.Stdout, "%s = %s\n", fullKey, formatted) //nolint:errcheck
				}
			} else {
				fmt.Fprintf(os.Stdout, "%s = %s\n", fullKey, formatted) //nolint:errcheck
			}
		}
	}
}
