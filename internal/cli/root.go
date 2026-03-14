package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/observability"
	"github.com/indrasvat/dootsabha/internal/output"
	"github.com/indrasvat/dootsabha/internal/plugin"
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
	verbosity      int
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
		// Initialize structured logger.
		logger := observability.SetupDefaultLogger(verbosity, jsonOutput)
		traceID := observability.NewTraceID()
		slog.SetDefault(logger.With("session_id", traceID))
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// Check for extension binary (dootsabha-{name} on $PATH or plugins/).
		ext, found := plugin.FindExtension(args[0], plugin.ExtensionDirs()...)
		if found {
			return execExtension(ext, args[1:])
		}
		return fmt.Errorf("unknown command %q — run 'dootsabha --help' for usage", args[0])
	},
	SilenceUsage: true,
}

// Execute runs the root command. Called from main().
func Execute() {
	setupSIGPIPE()
	// Always silence Cobra's built-in "Error: ..." stderr line — we handle
	// error display ourselves. This prevents unstructured stderr in JSON mode
	// while still showing errors to TTY users (GitHub issue #4, bug 3).
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			if jsonOutput {
				// Emit JSON error so automation always gets parseable stdout.
				_ = output.WriteErrorJSON(os.Stdout, "", exitErr.Message)
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s\n", exitErr.Message) //nolint:errcheck
			}
			os.Exit(exitErr.Code)
		}
		// Non-ExitError (e.g., unknown command, flag parse) — always show.
		if jsonOutput {
			_ = output.WriteErrorJSON(os.Stdout, "", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err) //nolint:errcheck
		}
		os.Exit(1)
	}
}

// execExtension runs an extension binary, forwarding args and stdio.
// It creates a Tier 2 context file with session info and cleans up after execution.
func execExtension(ext plugin.Extension, args []string) error {
	// Generate Tier 2 context file.
	traceID := observability.NewTraceID()
	isTTY := isatty.IsTerminal(os.Stdout.Fd())
	ctxFile := plugin.DefaultContextFile(traceID, isTTY, 80)
	ctxPath, err := plugin.WriteContextFile(ctxFile)
	if err != nil {
		slog.Warn("failed to create context file", "error", err)
	} else {
		defer func() { _ = os.Remove(ctxPath) }()
	}

	env := plugin.ExtensionEnv()
	if ctxPath != "" {
		env = append(env, "DOOTSABHA_CONTEXT_FILE="+ctxPath)
	}

	cmd := exec.Command(ext.Path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &ExitError{
				Code:    exitErr.ExitCode(),
				Message: fmt.Sprintf("extension %q exited with code %d", ext.Name, exitErr.ExitCode()),
			}
		}
		return fmt.Errorf("extension %q: %w", ext.Name, err)
	}
	return nil
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
	// Detect Claude Code session early — before command registration — so that
	// commands can adjust their defaults (e.g., council agents).
	core.DetectAndCleanClaude()

	cobra.EnablePrefixMatching = true

	f := rootCmd.PersistentFlags()
	f.BoolVar(&jsonOutput, "json", false, "Output as JSON (agent-friendly)")
	f.CountVarP(&verbosity, "verbose", "v", "Verbosity level (-v info, -vv debug, -vvv debug+source)")
	f.BoolVarP(&quiet, "quiet", "q", false, "Suppress all non-essential output")
	f.DurationVar(&globalTimeout, "timeout", 0, "Global invocation timeout (e.g. 5m, 30s)")
	f.Duration("kaalseema", 0, "Alias for --timeout (कालसीमा)")
	_ = f.MarkHidden("kaalseema")
	f.DurationVar(&sessionTimeout, "session-timeout", 0, "Max session duration (e.g. 30m)")
	f.StringVar(&configFile, "config", "", "Path to config file (YAML)")

	rootCmd.AddCommand(newConsultCmd())
	rootCmd.AddCommand(newCouncilCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newReviewCmd())
	rootCmd.AddCommand(newRefineCmd())
	rootCmd.AddCommand(newPluginCmd())
}
