package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/version"
)

// ExitError carries a specific process exit code from a command's RunE.
// Execute() checks for this type to select the right os.Exit code.
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string { return e.Message }

var (
	jsonOutput     bool
	verbose        bool
	quiet          bool
	globalTimeout  time.Duration
	sessionTimeout time.Duration
	configFile     string
)

var rootCmd = &cobra.Command{
	Use:   "dootsabha",
	Short: "dootsabha (दूतसभा) — AI council orchestrator",
	Long: `दूतसभा (dootsabha) — Council of AI Messengers

Orchestrate multiple AI coding agents (Claude, Codex, Gemini) in
council-mode deliberation, peer review, and synthesis.

दूतसभा — AI दूतों की सभा (Council of AI Messengers)
एकाधिक AI एजेंटों को समन्वित करें — विचार-विमर्श, समीक्षा, संश्लेषण।`,
	Args:    cobra.ArbitraryArgs,
	Version: version.String(),
	// PersistentPreRunE resolves bilingual flag aliases before any subcommand RunE runs.
	// Note: only the most-specific PersistentPreRunE in the chain is called; since no
	// subcommand defines its own, this root hook runs for all commands.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		pf := cmd.Root().PersistentFlags()
		// Resolve --kaalseema → --timeout bilingual alias.
		if kaalseema, _ := pf.GetDuration("kaalseema"); kaalseema != 0 && !pf.Changed("timeout") {
			globalTimeout = kaalseema
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return fmt.Errorf("unknown command %q — run 'dootsabha --help' for usage", args[0])
	},
	SilenceUsage: true,
}

// Execute runs the root command. Called from main().
func Execute() {
	setupSIGPIPE()
	if err := rootCmd.Execute(); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}

// setupSIGPIPE catches SIGPIPE and exits 0 so commands piped to `head` work cleanly.
func setupSIGPIPE() {
	sigpipeCh := make(chan os.Signal, 1)
	signal.Notify(sigpipeCh, syscall.SIGPIPE)
	go func() {
		<-sigpipeCh
		os.Exit(0)
	}()
}

func init() {
	cobra.EnablePrefixMatching = true

	f := rootCmd.PersistentFlags()
	f.BoolVar(&jsonOutput, "json", false, "Output as JSON (agent-friendly)")
	f.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	f.BoolVarP(&quiet, "quiet", "q", false, "Suppress all non-essential output")
	f.DurationVar(&globalTimeout, "timeout", 0, "Global invocation timeout (e.g. 5m, 30s)")
	f.Duration("kaalseema", 0, "Alias for --timeout (कालसीमा)")
	_ = f.MarkHidden("kaalseema")
	f.DurationVar(&sessionTimeout, "session-timeout", 0, "Max session duration (e.g. 30m)")
	f.StringVar(&configFile, "config", "", "Path to config file (YAML)")

	rootCmd.AddCommand(newConsultCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newConfigCmd())
}
