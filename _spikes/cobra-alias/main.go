// Spike 005: Cobra Alias Behavior
//
// Tests bilingual command/flag aliases for दूतसभा (dūtasabhā).
// Validates: alias invocation, help rendering, unknown-cmd hook,
// flag aliases (pflag ShorthandP vs custom), non-ASCII Devanagari
// rendering, prefix matching, and tab-completion registration.
//
// Run:
//   go run main.go --help
//   go run main.go council --help
//   go run main.go sabha          # alias for council
//   go run main.go consult --help
//   go run main.go paraamarsh     # alias for consult
//   go run main.go status --help
//   go run main.go sthiti         # alias for status
//   go run main.go unknown-cmd    # extension discovery
//   go run main.go council --agent claude
//   go run main.go council --doota claude   # flag alias
//   go run main.go council --timeout 10s
//   go run main.go council --kaalseema 10s  # flag alias
//   go run main.go completion bash          # tab completion

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// ─── Root command ─────────────────────────────────────────────────────────────

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		// The Use field drives the help banner — show bilingual name.
		Use:   "dootsabha",
		Short: "दूतसभा — The Council of Agents",
		Long: `दूतसभा (dūtasabhā) orchestrates AI coding agents through council-mode
deliberation, peer review, and synthesis.

Commands are shown with their Sanskrit aliases in parentheses.`,
		// FINDING: cobra.ArbitraryArgs is REQUIRED for unknown-cmd → RunE routing.
		// Without it, cobra returns "unknown command" error before calling RunE,
		// even when RunE IS defined on root. ArbitraryArgs tells cobra that the root
		// command accepts any positional args (subcommand routing still works first).
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// ── Global persistent flags ────────────────────────────────────────────
	pf := root.PersistentFlags()

	// --json
	pf.Bool("json", false, "Structured JSON output")

	// --verbose / -v
	pf.BoolP("verbose", "v", false, "Increase log verbosity")

	// --quiet / -q
	pf.BoolP("quiet", "q", false, "Suppress non-error output")

	// --timeout / --kaalseema  (flag alias via ShorthandP is only for single-char;
	// for full-word aliases we must use pflag's SetNormalizeFunc trick or a hidden
	// alias flag — both patterns are demonstrated below).
	pf.String("timeout", "5m", "Max time per agent invocation")
	// Hidden alias flag that writes to the same destination via StringVarP pointing
	// at the same variable.  NOTE: pflag does NOT support two long names for one
	// flag natively.  The canonical pattern is a hidden deprecated alias.
	pf.String("kaalseema", "", "Alias for --timeout (काल-सीमा)")
	must(pf.MarkHidden("kaalseema"))

	// --session-timeout / --satra-seema
	pf.String("session-timeout", "30m", "Max total session time")
	pf.String("satra-seema", "", "Alias for --session-timeout (सत्र-सीमा)")
	must(pf.MarkHidden("satra-seema"))

	// ── Unknown command → extension discovery ─────────────────────────────
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// This RunE is reached only when cobra can't route to a subcommand
		// (i.e., cobra.EnableCommandSorting is irrelevant here — we set
		// RunE on root to catch unknowns after cobra exhausts subcommands).
		return extensionDiscovery(cmd, args[0])
	}

	// cobra.OnInitialize not needed for spike.

	// Add subcommands.
	root.AddCommand(newCouncilCmd())
	root.AddCommand(newConsultCmd())
	root.AddCommand(newStatusCmd())

	// Enable prefix matching (e.g. "coun" matches "council").
	// FINDING: This also matches aliases, but only if the alias prefix is
	// unambiguous across ALL commands+aliases in the set.
	cobra.EnablePrefixMatching = true

	return root
}

// ─── council (sabha) ─────────────────────────────────────────────────────────

func newCouncilCmd() *cobra.Command {
	var agent, strategy string

	cmd := &cobra.Command{
		// Use includes hint about Sanskrit alias — shown in parent help.
		Use:     "council",
		Aliases: []string{"sabha"}, // सभा
		Short:   "council (sabha) — Run multi-agent council deliberation",
		Long: `Run multi-agent council deliberation.

Sanskrit alias: sabha (सभा — assembly/council)

Invokes all configured agents in parallel, then synthesizes their
responses through a strategy (default: compare).`,
		Example: `  dootsabha council "refactor this function"
  dootsabha sabha   "refactor this function"   # identical`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve flag alias: if --doota set, use it as --agent value.
			doota, _ := cmd.Flags().GetString("doota")
			if doota != "" && agent == "" {
				agent = doota
			}
			// Resolve --kaalseema → --timeout.
			kaalseema, _ := cmd.Root().PersistentFlags().GetString("kaalseema")
			timeout, _ := cmd.Root().PersistentFlags().GetString("timeout")
			if kaalseema != "" {
				timeout = kaalseema
			}

			fmt.Printf("[council] invoked via: %q\n", cmd.CalledAs())
			fmt.Printf("[council] agent=%q  strategy=%q  timeout=%s\n",
				agent, strategy, timeout)
			if len(args) > 0 {
				fmt.Printf("[council] prompt: %s\n", strings.Join(args, " "))
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&agent, "agent", "a", "", "Agent to use (e.g. claude, codex, gemini)")
	// --doota is the Sanskrit flag alias for --agent (दूत = messenger/agent).
	// FINDING: pflag has no first-class "flag alias" feature.  The canonical
	// approach is a hidden flag that copies its value into the primary at RunE.
	f.String("doota", "", "Alias for --agent (दूत — messenger/agent)")
	must(f.MarkHidden("doota"))

	f.StringVar(&strategy, "strategy", "compare", "Synthesis strategy")

	return cmd
}

// ─── consult (paraamarsh) ────────────────────────────────────────────────────

func newConsultCmd() *cobra.Command {
	var agent string

	cmd := &cobra.Command{
		Use:     "consult",
		Aliases: []string{"paraamarsh"}, // परामर्श
		Short:   "consult (paraamarsh) — Query a single agent",
		Long: `Query a single agent and return its response.

Sanskrit alias: paraamarsh (परामर्श — consultation/counsel)`,
		Example: `  dootsabha consult --agent claude "explain this code"
  dootsabha paraamarsh --doota claude "explain this code"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			doota, _ := cmd.Flags().GetString("doota")
			if doota != "" && agent == "" {
				agent = doota
			}
			fmt.Printf("[consult] invoked via: %q\n", cmd.CalledAs())
			fmt.Printf("[consult] agent=%q\n", agent)
			return nil
		},
	}

	cmd.Flags().StringVarP(&agent, "agent", "a", "claude", "Agent to query")
	cmd.Flags().String("doota", "", "Alias for --agent (दूत)")
	must(cmd.Flags().MarkHidden("doota"))

	return cmd
}

// ─── status (sthiti) ─────────────────────────────────────────────────────────

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"sthiti"}, // स्थिति
		Short:   "status (sthiti) — Show agent health and config",
		Long: `Show health status of all configured agents.

Sanskrit alias: sthiti (स्थिति — condition/state)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("[status] invoked via: %q\n", cmd.CalledAs())
			fmt.Println("[status] all agents healthy (spike stub)")
			return nil
		},
	}
	return cmd
}

// ─── Extension discovery ─────────────────────────────────────────────────────

// extensionDiscovery replicates §6.1 behavior:
// unknown cmd → look for dootsabha-{name} on PATH → prompt → exec.
func extensionDiscovery(rootCmd *cobra.Command, name string) error {
	extName := "dootsabha-" + name
	path, err := exec.LookPath(extName)
	if err != nil {
		// Provide closest-match suggestion using cobra's built-in suggestions.
		suggestions := rootCmd.SuggestionsFor(name)
		msg := fmt.Sprintf("unknown command %q", name)
		if len(suggestions) > 0 {
			msg += "\n\nDid you mean one of these?\n"
			for _, s := range suggestions {
				msg += "        " + s + "\n"
			}
		}
		msg += fmt.Sprintf("\nExtension %q not found on PATH.", extName)
		return fmt.Errorf("%s", msg)
	}
	// Found extension.
	fmt.Printf("[extension] found: %s\n", path)
	fmt.Printf("[extension] would exec: %s (trusted check omitted in spike)\n", path)
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
