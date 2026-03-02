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
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newConfigShowCmd())
	return cmd
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
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := core.LoadConfig(configFile)
			if err != nil {
				return &ExitError{Code: 5, Message: fmt.Sprintf("load config: %s", err)}
			}

			view := cfg.RedactedView(reveal)

			useJSON := showJSON || jsonOutput
			rc := output.NewRenderContext(os.Stdout, useJSON)

			if rc.IsJSON() || showJSON {
				return output.WriteJSON(os.Stdout, view)
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
