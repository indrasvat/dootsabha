package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/output"
	"github.com/indrasvat/dootsabha/internal/plugin"
)

// pluginEntry represents a discovered plugin or extension for list/inspect rendering.
type pluginEntry struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plugin",
		Aliases: []string{"vistaarak", "विस्तारक"},
		Short:   "plugin (vistaarak) — List and inspect plugins & extensions",
		Long: `Discover and inspect available plugins and PATH extensions.

विस्तारक (vistaarak) — प्लगइन्स और एक्सटेंशन की सूची और जानकारी।`,
	}

	cmd.AddCommand(newPluginListCmd())
	cmd.AddCommand(newPluginInspectCmd())

	return cmd
}

func newPluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Aliases:      []string{"soochi", "सूची"},
		Short:        "List all plugins and extensions",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries := discoverAll()

			rc := output.NewRenderContext(os.Stdout, jsonOutput)

			if rc.IsJSON() {
				return output.WriteJSON(os.Stdout, entries)
			}

			if len(entries) == 0 {
				fmt.Fprintln(os.Stderr, "No plugins or extensions found.")
				fmt.Fprintln(os.Stderr, "  Install plugins to plugins/bin/ or add dootsabha-{name} to $PATH")
				return nil
			}

			renderPluginTable(rc, entries)
			return nil
		},
	}
}

func newPluginInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "inspect [name]",
		Aliases:      []string{"parikshan", "परीक्षण"},
		Short:        "Inspect a plugin or extension",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			entries := discoverAll()

			var found *pluginEntry
			for i := range entries {
				if entries[i].Name == name {
					found = &entries[i]
					break
				}
			}

			if found == nil {
				return fmt.Errorf("plugin or extension %q not found", name)
			}

			rc := output.NewRenderContext(os.Stdout, jsonOutput)

			if rc.IsJSON() {
				return output.WriteJSON(os.Stdout, found)
			}

			renderPluginInspect(rc, *found)
			return nil
		},
	}
}

// discoverAll finds gRPC plugin binaries and PATH extensions.
func discoverAll() []pluginEntry {
	var entries []pluginEntry

	// 1. Scan for gRPC plugin binaries relative to the dootsabha executable.
	entries = append(entries, discoverGRPCPlugins()...)

	// 2. Scan for PATH extensions.
	exts := plugin.DiscoverExtensions(plugin.ExtensionDirs()...)
	for _, ext := range exts {
		entries = append(entries, pluginEntry{
			Name:   ext.Name,
			Type:   "extension",
			Path:   ext.Path,
			Status: "available",
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type < entries[j].Type
		}
		return entries[i].Name < entries[j].Name
	})

	return entries
}

// discoverGRPCPlugins scans the plugins/bin/ directory relative to the
// dootsabha binary for gRPC plugin binaries. Plugin type is inferred from
// the binary name suffix (-provider, -strategy, -hook).
func discoverGRPCPlugins() []pluginEntry {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	binDir := filepath.Join(filepath.Dir(exePath), "..", "plugins", "bin")

	// Resolve symlinks to get the real path.
	binDir, err = filepath.EvalSymlinks(binDir)
	if err != nil {
		return nil
	}

	dirEntries, err := os.ReadDir(binDir)
	if err != nil {
		return nil
	}

	var entries []pluginEntry
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}

		name := de.Name()
		pluginType := inferPluginType(name)
		if pluginType == "" {
			continue
		}

		entries = append(entries, pluginEntry{
			Name:   name,
			Type:   pluginType,
			Path:   filepath.Join(binDir, name),
			Status: "installed",
		})
	}

	return entries
}

// inferPluginType determines plugin type from binary name suffix.
func inferPluginType(name string) string {
	switch {
	case strings.HasSuffix(name, "-provider"):
		return "provider"
	case strings.HasSuffix(name, "-strategy"):
		return "strategy"
	case strings.HasSuffix(name, "-hook"):
		return "hook"
	default:
		return ""
	}
}

// renderPluginTable renders the plugin list as a styled table.
func renderPluginTable(rc *output.RenderContext, entries []pluginEntry) {
	tbl := output.NewTable(rc).
		Headers("NAME", "TYPE", "STATUS", "PATH")

	for _, e := range entries {
		dot := output.ProviderDot(rc, pluginTypeColor(e.Type))
		name := dot + " " + e.Name

		status := output.StatusOK(rc)
		if e.Status == "available" {
			status = output.Styled(rc, lipgloss.NewStyle().Foreground(output.MutedColor), "available")
		}

		tbl.Row(name, e.Type, status, e.Path)
	}

	tbl.Render(os.Stdout)
}

// renderPluginInspect renders detailed plugin info.
func renderPluginInspect(rc *output.RenderContext, e pluginEntry) {
	header := output.CommandHeader(rc, "Plugin: "+e.Name, e.Type+" plugin")
	fmt.Fprintln(os.Stdout, header)                    //nolint:errcheck
	fmt.Fprintln(os.Stdout)                            //nolint:errcheck
	fmt.Fprintf(os.Stdout, "  Name:   %s\n", e.Name)   //nolint:errcheck
	fmt.Fprintf(os.Stdout, "  Type:   %s\n", e.Type)   //nolint:errcheck
	fmt.Fprintf(os.Stdout, "  Path:   %s\n", e.Path)   //nolint:errcheck
	fmt.Fprintf(os.Stdout, "  Status: %s\n", e.Status) //nolint:errcheck
}

// pluginTypeColor returns the lipgloss color for a plugin type.
func pluginTypeColor(t string) lipgloss.Color {
	switch t {
	case "provider":
		return output.AccentColor
	case "strategy":
		return output.SuccessColor
	case "hook":
		return output.WarnColor
	default:
		return output.MutedColor
	}
}
