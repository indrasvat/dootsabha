package cli

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/output"
)

// jsonDocWritten records that a command has already emitted a JSON document on
// stdout.
//
// Commands render their own envelope (result, or a detailed provider error), and
// Execute() separately emits an error envelope for any ExitError in JSON mode.
// Without coordination both fire on a failure path and stdout carries TWO JSON
// documents, which `jq`/`json.load` reject with "Extra data" — breaking scripted
// consumers exactly when they most need to read the failure. The command's own
// document wins because it is the more specific one (it names the provider, and
// for council it carries the partial results).
//
// Safe as package state: one dootsabha command runs per process.
var jsonDocWritten bool

// emitJSON writes v to stdout as the command's single JSON document.
func emitJSON(v any) error {
	jsonDocWritten = true
	return output.WriteJSON(os.Stdout, v)
}

// emitErrorJSON writes a structured error as the command's single JSON document.
func emitErrorJSON(provider, errMsg string) {
	jsonDocWritten = true
	_ = output.WriteErrorJSON(os.Stdout, provider, errMsg)
}

// markJSONWritten records a JSON document written directly (e.g. via
// json.NewEncoder) rather than through the helpers above.
func markJSONWritten() { jsonDocWritten = true }

// usageArgs wraps a Cobra positional-argument validator so a wrong arg count
// surfaces as a typed usage error.
//
// Cobra returns plain errors from Args validators, which would otherwise force
// Execute() to recognise them by matching message text. Typing them here keeps
// the classification structural.
func usageArgs(v cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := v(cmd, args); err != nil {
			return &ExitError{Code: core.ExitUsage, Message: err.Error()}
		}
		return nil
	}
}

// stageExitCode maps a pipeline-stage failure to an exit code, honouring the
// documented precedence rather than reporting whichever symptom surfaced last.
//
// A deadline outranks a provider failure (4 > 3 > 5): when the context expires,
// downstream stages fail as a *consequence* — a council whose agent hangs would
// otherwise report "synthesis failed" (3) and hide the real cause. Callers pass
// the code that applies when no deadline was hit.
//
// Codes are combined via core.HighestExitCode so the precedence table is
// actually computed, not merely documented.
func stageExitCode(ctx context.Context, err error, fallback int) int {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return core.HighestExitCode(core.ExitTimeout, fallback)
	}
	return fallback
}
