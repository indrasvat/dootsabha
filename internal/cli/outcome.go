package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// Outcome is the fate of every provider call a pipeline made, and the single
// input to that pipeline's exit code.
//
// Before this, review, refine and council each decided their own exit code from
// their own bookkeeping: inline errors.Is at two call sites in one, a
// hand-tracked deadline/partial pair in another, three result slices in the
// third. Six defects came out of that shape in a single change, four of them
// the same defect — a failure recorded somewhere the aggregator did not look:
//
//	a chair that timed out but whose fallback succeeded carried no error at all
//	a chair that timed out with no fallback had its error replaced on the way out
//	a chair that timed out in round 1 was erased by a healthy round 2
//	a peer review that failed for any non-deadline reason was never counted
//
// The fix is not a second place to record failures — the result types already
// carry them. It is that every command must reach its exit code through the
// same function, and that adding a stage or a result type must be impossible to
// half-finish. TestOutcomeConsumesEveryResultError enforces the second half.
type Outcome struct {
	budget *core.Budget
	calls  []call
}

// call is one provider invocation's fate. A nil err is a success, which is what
// separates exit 5 ("some failed, output is usable") from exit 3 ("every
// requested agent failed").
type call struct {
	provider string
	scope    string // which budget bounded it; "" means ask the budget
	err      error
}

// newOutcome starts an empty ledger for one pipeline run.
func newOutcome(b *core.Budget) *Outcome { return &Outcome{budget: b} }

// Invoke runs ONE provider call in a fresh per-invocation window and records
// how it went. It is the only way review and refine may call a provider, so
// neither can produce a result the exit decision never sees.
//
// The scope is captured here, from the context that actually bounded the call,
// rather than by asking the budget afterwards — a subprocess sitting in its
// SIGTERM grace period can let a later session expiry rewrite the diagnosis.
func (o *Outcome) Invoke(prov providers.Provider, name, prompt string, opts providers.InvokeOptions) (*providers.ProviderResult, error) {
	ctx, cancel := o.budget.Step()
	defer cancel()
	scope := o.budget.ScopeOf(ctx)
	res, err := prov.Invoke(ctx, prompt, opts)
	o.record(name, scope, err)
	return res, err
}

// Fail records a failure that happened without an invocation — an agent that
// could not be constructed, or a stage abandoned before it ran. Without this
// such failures are invisible to the exit decision, which is the whole class of
// defect this type exists to close.
func (o *Outcome) Fail(provider string, err error) { o.record(provider, "", err) }

func (o *Outcome) record(provider, scope string, err error) {
	o.calls = append(o.calls, call{provider: provider, scope: scope, err: err})
}

// AddDispatches, AddReviews and AddSynthesis fold the engine's results into the
// ledger. The engine invokes agents itself, so its outcomes arrive as data
// rather than through Invoke — and arriving as data is what lets an
// out-of-process strategy plugin reach the same exit decision, since its gRPC
// response converts to these same types.
//
// Scope is left empty: the engine owns the step contexts, so the budget is
// asked at Exit time instead. Sequential commands, where the distinction is
// most visible to a user, go through Invoke and get it exactly.
func (o *Outcome) AddDispatches(ds []core.DispatchResult) {
	for _, d := range ds {
		o.record(d.Provider, "", d.Error)
	}
}

func (o *Outcome) AddReviews(rs []core.ReviewResult) {
	for _, r := range rs {
		o.record(r.Reviewer, "", r.Error)
	}
}

// AddSynthesis records the chair's fate. A chair that failed and was replaced
// still failed, so ChairError is recorded even when the fallback produced the
// answer; the synthesis itself is recorded as the agent that actually ran.
func (o *Outcome) AddSynthesis(s *core.SynthesisResult) {
	if s == nil {
		return
	}
	if s.ChairError != nil {
		o.record(s.Chair, "", s.ChairError)
	}
	if s.ChairFallback != "" {
		o.record(s.ChairFallback, "", nil)
	} else if s.ChairError == nil {
		o.record(s.Chair, "", nil)
	}
}

// deadline returns the first call that died on a deadline, with the scope
// recorded when it ran.
func (o *Outcome) deadline() (scope string, err error) {
	for _, c := range o.calls {
		if errors.Is(c.err, context.DeadlineExceeded) {
			return c.scope, c.err
		}
	}
	return "", nil
}

// succeeded reports whether any call produced output.
func (o *Outcome) succeeded() bool {
	for _, c := range o.calls {
		if c.err == nil {
			return true
		}
	}
	return false
}

// failed reports whether any call did not.
func (o *Outcome) failed() bool {
	for _, c := range o.calls {
		if c.err != nil {
			return true
		}
	}
	return false
}

// Exit is the single exit-code decision for every pipeline command.
//
//	4  any call hit a deadline — outranks the partial result it causes
//	0  nothing failed
//	5  something failed but a call succeeded, so there is output to use
//	3  every call failed, so there is nothing to use
//
// "Nothing usable" is derived from the ledger rather than judged per command:
// exit 3 means "every requested agent failed", which is exactly "no call
// succeeded". That is one fewer thing for a new command to get wrong.
//
// A session that expires after the last call succeeded is deliberately NOT a
// timeout: the work finished inside its budget, and only rendering crossed the
// line. Every case where the ceiling actually cut work short shows up as a call
// that failed on a deadline, and is caught above.
func (o *Outcome) Exit(what string) error {
	if scope, err := o.deadline(); err != nil {
		return &ExitError{Code: core.ExitTimeout, Message: timeoutMessage(o.budget, scope, err)}
	}
	if !o.failed() {
		return nil
	}
	if o.succeeded() {
		return &ExitError{
			Code:    core.ExitPartial,
			Message: fmt.Sprintf("partial result: %s", what),
		}
	}
	return &ExitError{Code: core.ExitProvider, Message: what}
}
