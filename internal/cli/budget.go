package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
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
//
// steps is how many provider invocations the pipeline will make at most; it is
// used only to warn about a ceiling too small to fit them.
func newBudget(cmd *cobra.Command, cfg *core.Config, steps int) *core.Budget {
	perInvoke, session := resolveTimeouts(globalTimeout, sessionTimeout, sessionTimeoutSet(cmd), cfg)
	// Written to stderr even in JSON mode: stdout stays exactly one document,
	// and a budget that silently truncates every call is worth a line in the
	// log of a scripted run too.
	if w := budgetWarning(perInvoke, session, steps); w != "" && !quiet {
		fmt.Fprintln(os.Stderr, "Warning: "+w) //nolint:errcheck
	}
	return core.NewBudget(perInvoke, session)
}

// budgetWarning describes a session ceiling too small for the pipeline it is
// about to bound, or "" when the pair is coherent.
//
// Neither case is an error — the ceiling simply wins — but both are almost
// never what the user meant, and staying silent means finding out only after
// the pipeline has been cut short. Two shapes are caught:
//
//   - one call cannot fit: `--timeout 40m --session-timeout 30m`
//   - the calls cannot fit together: `refine --reviewers a,b` is 5 calls, so
//     the default 5m × 5 overruns the default 30m ceiling
func budgetWarning(perInvoke, session time.Duration, steps int) string {
	if session <= 0 {
		return ""
	}
	if perInvoke > session {
		return fmt.Sprintf(
			"--timeout %s is larger than --session-timeout %s, so every invocation is cut short at %s — raise --session-timeout (or session_timeout in config)",
			perInvoke, session, session,
		)
	}
	if steps > 1 && perInvoke*time.Duration(steps) > session {
		return fmt.Sprintf(
			"this pipeline makes up to %d calls of %s (%s), more than the --session-timeout of %s — it may be cut short partway; raise --session-timeout (or session_timeout in config)",
			steps, perInvoke, perInvoke*time.Duration(steps), session,
		)
	}
	return ""
}

// invokeStep runs one provider call inside a fresh per-invocation context.
//
// Every provider call in a multi-step pipeline goes through here (issue #20):
// it is what guarantees each call a full window rather than the remains of a
// shared deadline. It returns the scope that bounded the call, so a deadline is
// reported against the right knob even if the session expires moments later,
// and it releases the timer through defer so a panicking provider cannot leak
// one.
func invokeStep(b *core.Budget, prov providers.Provider, prompt string, opts providers.InvokeOptions) (*providers.ProviderResult, string, error) {
	ctx, cancel := b.Step()
	defer cancel()
	scope := b.ScopeOf(ctx)
	res, err := prov.Invoke(ctx, prompt, opts)
	return res, scope, err
}

// timeoutMessage renders a deadline failure so the reader knows which limit
// fired and which knob to turn. An empty scope asks the budget to classify.
//
// Issue #20's whole user-visible symptom was one message shape for both cases:
// a run that blew its 8m pipeline budget reported "timeout after 8m0s" against
// the reviewer, which read as "the reviewer is broken" when the reviewer had
// been healthy all along.
func timeoutMessage(b *core.Budget, scope string, err error) string {
	if scope == "" {
		scope = b.TimeoutScope()
	}
	limit := b.PerInvoke()
	if scope == core.TimeoutScopeSession {
		limit = b.SessionLimit()
	}
	if limit <= 0 {
		return fmt.Sprintf("%s timeout: %s", scope, err)
	}
	return fmt.Sprintf("%s timeout after %s: %s", scope, limit, err)
}

// stageFailureMessage labels a failed pipeline stage, naming the budget that
// fired when the cause was a deadline.
func stageFailureMessage(b *core.Budget, stage string, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("%s: %s", stage, timeoutMessage(b, "", err))
	}
	return fmt.Sprintf("%s: %s", stage, err)
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
