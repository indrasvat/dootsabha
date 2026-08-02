package core

import (
	"context"
	"errors"
	"time"
)

// TimeoutScope names which of a pipeline's two deadlines fired. A timeout is
// only actionable if the caller knows which knob to turn.
const (
	TimeoutScopeInvocation = "invocation"
	TimeoutScopeSession    = "session"
)

// Budget separates the two deadlines a multi-agent pipeline needs: `timeout` is
// what ONE provider invocation may take, `session_timeout` is what the whole
// pipeline may take.
//
// Sharing a single deadline across every call — the behaviour before GitHub
// issue #20 — turns the budget into a race between stages. A `review` run with
// `--timeout 8m` gave the author 5m38s and the reviewer whatever was left, then
// reported the reviewer as the thing that timed out. The author was the cost;
// the reviewer was the symptom.
//
// Step hands out a fresh per-invocation context each time, parented to the
// session context so the pipeline ceiling still binds. A zero or negative
// duration disables that bound.
type Budget struct {
	ctx       context.Context
	cancel    context.CancelFunc
	perInvoke time.Duration
	session   time.Duration
}

// NewBudget creates a pipeline budget. Either duration may be 0 (or negative,
// which is normalised to 0) to disable that bound.
func NewBudget(perInvoke, session time.Duration) *Budget {
	if perInvoke < 0 {
		perInvoke = 0
	}
	if session < 0 {
		session = 0
	}
	b := &Budget{perInvoke: perInvoke, session: session}
	if session > 0 {
		b.ctx, b.cancel = context.WithTimeout(context.Background(), session)
	} else {
		b.ctx, b.cancel = context.WithCancel(context.Background())
	}
	return b
}

// Session returns the pipeline-wide context. Pass it to anything that spans
// stages; pass Step's context to a single provider call.
func (b *Budget) Session() context.Context { return b.ctx }

// PerInvoke returns the per-invocation limit (0 = unbounded).
func (b *Budget) PerInvoke() time.Duration { return b.perInvoke }

// SessionLimit returns the whole-pipeline limit (0 = unbounded).
func (b *Budget) SessionLimit() time.Duration { return b.session }

// Step returns a fresh context for one provider invocation. The caller must
// call the returned cancel when that invocation finishes — deferring it to the
// end of a multi-step pipeline would hold one timer per step.
//
// Safe for concurrent use: council dispatches agents in parallel.
func (b *Budget) Step() (context.Context, context.CancelFunc) {
	return StepContext(b.ctx, b.perInvoke)
}

// Close cancels the session context and, with it, every outstanding step. It is
// the caller's deferred cleanup, so a killed pipeline takes its provider
// subprocesses down with it rather than orphaning them.
func (b *Budget) Close() { b.cancel() }

// SessionExpired reports whether the pipeline-wide deadline fired, as opposed to
// the budget simply having been closed.
func (b *Budget) SessionExpired() bool {
	return errors.Is(b.ctx.Err(), context.DeadlineExceeded)
}

// TimeoutScope names the deadline responsible for a failure. A step context
// inherits DeadlineExceeded from an expired session, so the two are told apart
// by asking the session directly rather than by inspecting the step's error.
func (b *Budget) TimeoutScope() string {
	if b.SessionExpired() {
		return TimeoutScopeSession
	}
	return TimeoutScopeInvocation
}

// StepContext derives a per-invocation context from parent, bounded by d.
// d <= 0 means no per-invocation bound — parent's deadline, if any, still binds.
//
// Context semantics do the clipping: the effective deadline is always the
// earlier of parent's and d, so a per-invocation budget can never outlive the
// session ceiling.
func StepContext(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, d)
}
