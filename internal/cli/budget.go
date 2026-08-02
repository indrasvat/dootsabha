package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
)

// defaultInvokeTimeout bounds a single provider invocation when neither the
// --timeout flag nor the config supplies one.
const defaultInvokeTimeout = 5 * time.Minute

// resolveTimeouts picks the per-invocation and session budgets, preferring the
// flag, then the config, then the built-in default.
//
// sessionSet distinguishes "--session-timeout was not given" from
// "--session-timeout 0" — the latter is how a user disables the pipeline
// ceiling, and must not silently fall through to the config value.
func resolveTimeouts(flagInvoke, flagSession time.Duration, sessionSet bool, cfg *core.Config) (perInvoke, session time.Duration) {
	perInvoke = flagInvoke
	if perInvoke <= 0 {
		perInvoke = cfg.Timeout
	}
	if perInvoke <= 0 {
		perInvoke = defaultInvokeTimeout
	}

	session = cfg.SessionTimeout
	if sessionSet {
		session = flagSession
	}
	if session < 0 {
		session = 0
	}
	return perInvoke, session
}

// sessionTimeoutSet reports whether the user asked for a specific session
// ceiling, through either spelling of the flag.
func sessionTimeoutSet(cmd *cobra.Command) bool {
	pf := cmd.Root().PersistentFlags()
	return pf.Changed("session-timeout") || pf.Changed("satra-seema")
}

// newBudget builds the two-deadline budget a multi-step command runs under.
// Callers must defer Close.
func newBudget(cmd *cobra.Command, cfg *core.Config) *core.Budget {
	perInvoke, session := resolveTimeouts(globalTimeout, sessionTimeout, sessionTimeoutSet(cmd), cfg)
	if w := budgetInversionWarning(perInvoke, session); w != "" && !jsonOutput && !quiet {
		fmt.Fprintln(os.Stderr, "Warning: "+w) //nolint:errcheck
	}
	return core.NewBudget(perInvoke, session)
}

// budgetInversionWarning describes a per-invocation budget that cannot fit
// inside the session ceiling, or "" when the pair is coherent.
//
// The combination is not an error — the ceiling simply wins — but it is almost
// never what the user meant, and staying silent means finding out only after the
// pipeline has been cut short.
func budgetInversionWarning(perInvoke, session time.Duration) string {
	if session <= 0 || perInvoke <= session {
		return ""
	}
	return fmt.Sprintf(
		"--timeout %s is larger than --session-timeout %s, so every invocation is cut short at %s — raise --session-timeout (or session_timeout in config)",
		perInvoke, session, session,
	)
}

// timeoutMessage renders a deadline failure so the reader knows which limit
// fired and which knob to turn.
//
// Issue #20's whole user-visible symptom was one message shape for both cases:
// a run that blew its 8m pipeline budget reported "timeout after 8m0s" against
// the reviewer, which read as "the reviewer is broken" when the reviewer had
// been healthy all along.
func timeoutMessage(b *core.Budget, err error) string {
	scope := b.TimeoutScope()
	limit := b.PerInvoke()
	if scope == core.TimeoutScopeSession {
		limit = b.SessionLimit()
	}
	if limit <= 0 {
		return fmt.Sprintf("%s timeout: %s", scope, err)
	}
	return fmt.Sprintf("%s timeout after %s: %s", scope, limit, err)
}

// firstDeadline returns the first deadline error among errs, or nil.
//
// Exit code 4 means "at least one agent timed out" and outranks the partial
// result it causes. With per-invocation budgets a single agent can hit its
// deadline while the session is still healthy, so the session context alone no
// longer answers the question.
func firstDeadline(errs ...error) error {
	for _, err := range errs {
		if errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	return nil
}
