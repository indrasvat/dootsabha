package core_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// Budget is the fix for GitHub issue #20: `review`/`refine`/`council` shared ONE
// context.WithTimeout across every provider call, so a slow author consumed the
// reviewer's budget and the reviewer died with a misleading provider timeout.
//
// These tests assert deadlines rather than waiting for them wherever possible —
// a wall-clock assertion is deterministic, a sleep race is not.

// remaining reports how much budget a context has left, and whether it is bounded.
func remaining(t *testing.T, ctx context.Context) (time.Duration, bool) {
	t.Helper()
	dl, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	return time.Until(dl), true
}

// assertWindow fails unless ctx has a deadline within tolerance of want.
func assertWindow(t *testing.T, ctx context.Context, want, tolerance time.Duration, what string) {
	t.Helper()
	got, ok := remaining(t, ctx)
	if !ok {
		t.Fatalf("%s: context has no deadline, want ~%s", what, want)
	}
	if got < want-tolerance || got > want+tolerance {
		t.Errorf("%s: window = %s, want %s ±%s", what, got, want, tolerance)
	}
}

// TestBudgetStepIsFreshEachTime is the core regression guard for issue #20.
//
// Five sequential steps, each burning its whole window, must every one of them
// start with the full per-invocation budget. Under the old shared-context model
// step 2 onwards inherited whatever step 1 left behind — which was nothing.
func TestBudgetStepIsFreshEachTime(t *testing.T) {
	const perInvoke = 200 * time.Millisecond
	b := core.NewBudget(perInvoke, 10*time.Second)
	defer b.Close()

	for i := 1; i <= 5; i++ {
		step, cancel := b.Step()
		assertWindow(t, step, perInvoke, 60*time.Millisecond, "step "+string(rune('0'+i)))
		cancel()
	}
}

// TestBudgetStepFreshAfterPredecessorExpires is the same property proven the
// expensive way: let the first step actually hit its deadline, then check the
// next one still opens with a full window.
func TestBudgetStepFreshAfterPredecessorExpires(t *testing.T) {
	const perInvoke = 120 * time.Millisecond
	b := core.NewBudget(perInvoke, 10*time.Second)
	defer b.Close()

	first, cancelFirst := b.Step()
	<-first.Done()
	cancelFirst()
	if !errors.Is(first.Err(), context.DeadlineExceeded) {
		t.Fatalf("first step Err() = %v, want DeadlineExceeded", first.Err())
	}

	second, cancelSecond := b.Step()
	defer cancelSecond()
	if err := second.Err(); err != nil {
		t.Fatalf("second step already dead: %v — a fresh invocation must start clean", err)
	}
	assertWindow(t, second, perInvoke, 60*time.Millisecond, "step after an expired step")
}

// TestBudgetStepClippedBySession keeps the session timeout a hard ceiling: a
// per-invocation window may never outlive the pipeline budget.
func TestBudgetStepClippedBySession(t *testing.T) {
	b := core.NewBudget(10*time.Second, 150*time.Millisecond)
	defer b.Close()

	step, cancel := b.Step()
	defer cancel()
	assertWindow(t, step, 150*time.Millisecond, 60*time.Millisecond, "step under a shorter session")
}

// TestBudgetStepInheritsSessionWhenPerInvokeDisabled — timeout 0 means "no
// per-invocation bound", leaving only the session ceiling.
func TestBudgetStepInheritsSessionWhenPerInvokeDisabled(t *testing.T) {
	b := core.NewBudget(0, 400*time.Millisecond)
	defer b.Close()

	step, cancel := b.Step()
	defer cancel()
	assertWindow(t, step, 400*time.Millisecond, 60*time.Millisecond, "step with per-invoke disabled")
}

// TestBudgetStepUnboundedSession — session 0 means "no pipeline bound"; each
// step still gets its own per-invocation window.
func TestBudgetStepUnboundedSession(t *testing.T) {
	b := core.NewBudget(300*time.Millisecond, 0)
	defer b.Close()

	if _, ok := b.Session().Deadline(); ok {
		t.Error("session context should have no deadline when session timeout is 0")
	}
	step, cancel := b.Step()
	defer cancel()
	assertWindow(t, step, 300*time.Millisecond, 60*time.Millisecond, "step with unbounded session")
}

// TestBudgetBothDisabled — neither bound set means nothing expires. Cancellation
// must still work, so callers can always release resources.
func TestBudgetBothDisabled(t *testing.T) {
	b := core.NewBudget(0, 0)
	defer b.Close()

	step, cancel := b.Step()
	if _, ok := step.Deadline(); ok {
		t.Error("step should have no deadline when both timeouts are 0")
	}
	cancel()
	if !errors.Is(step.Err(), context.Canceled) {
		t.Errorf("cancelled step Err() = %v, want Canceled", step.Err())
	}
}

// TestBudgetNegativeDurationsDisable treats a negative duration like zero rather
// than handing out an already-expired context.
func TestBudgetNegativeDurationsDisable(t *testing.T) {
	b := core.NewBudget(-1*time.Second, -1*time.Second)
	defer b.Close()

	if err := b.Session().Err(); err != nil {
		t.Fatalf("session with negative timeout is already dead: %v", err)
	}
	step, cancel := b.Step()
	defer cancel()
	if err := step.Err(); err != nil {
		t.Fatalf("step with negative timeout is already dead: %v", err)
	}
	if _, ok := step.Deadline(); ok {
		t.Error("negative per-invocation timeout should mean unbounded, not expired")
	}
}

// TestBudgetStepsAreIndependent — cancelling one provider's context must not
// take down a sibling. review/refine cancel each step as they go.
func TestBudgetStepsAreIndependent(t *testing.T) {
	b := core.NewBudget(5*time.Second, 10*time.Second)
	defer b.Close()

	one, cancelOne := b.Step()
	two, cancelTwo := b.Step()
	defer cancelTwo()

	cancelOne()
	if one.Err() == nil {
		t.Fatal("cancelled step should be done")
	}
	if err := two.Err(); err != nil {
		t.Errorf("sibling step died with %v when its neighbour was cancelled", err)
	}
	if err := b.Session().Err(); err != nil {
		t.Errorf("session died with %v when a single step was cancelled", err)
	}
}

// TestBudgetCloseCancelsOutstandingSteps — Close is the caller's deferred
// cleanup; it must stop in-flight provider subprocesses, not orphan them.
func TestBudgetCloseCancelsOutstandingSteps(t *testing.T) {
	b := core.NewBudget(5*time.Second, 10*time.Second)
	step, cancel := b.Step()
	defer cancel()

	b.Close()
	if !errors.Is(step.Err(), context.Canceled) {
		t.Errorf("step Err() after Close = %v, want Canceled", step.Err())
	}
	if !errors.Is(b.Session().Err(), context.Canceled) {
		t.Errorf("session Err() after Close = %v, want Canceled", b.Session().Err())
	}
}

// TestBudgetSessionExpiryPropagatesToStep — once the pipeline budget is spent,
// no further provider call may start with a live window.
func TestBudgetSessionExpiryPropagatesToStep(t *testing.T) {
	b := core.NewBudget(10*time.Second, 100*time.Millisecond)
	defer b.Close()

	<-b.Session().Done()

	if !b.SessionExpired() {
		t.Error("SessionExpired() = false after the session deadline passed")
	}
	step, cancel := b.Step()
	defer cancel()
	if !errors.Is(step.Err(), context.DeadlineExceeded) {
		t.Errorf("step started after session expiry has Err() = %v, want DeadlineExceeded", step.Err())
	}
}

// TestBudgetSessionExpiredIsFalseWhileLive guards the classifier's negative case:
// a per-invocation timeout must not be reported as a session timeout.
func TestBudgetSessionExpiredIsFalseWhileLive(t *testing.T) {
	b := core.NewBudget(80*time.Millisecond, 10*time.Second)
	defer b.Close()

	step, cancel := b.Step()
	defer cancel()
	<-step.Done()

	if !errors.Is(step.Err(), context.DeadlineExceeded) {
		t.Fatalf("step Err() = %v, want DeadlineExceeded", step.Err())
	}
	if b.SessionExpired() {
		t.Error("SessionExpired() = true after only the per-invocation window expired")
	}
}

// TestBudgetSessionExpiredIsFalseAfterClose — Close cancels, it does not expire.
// Reporting a user Ctrl-C as a session timeout would send them to the wrong knob.
func TestBudgetSessionExpiredIsFalseAfterClose(t *testing.T) {
	b := core.NewBudget(time.Second, time.Second)
	b.Close()
	if b.SessionExpired() {
		t.Error("SessionExpired() = true after Close — cancellation is not a deadline")
	}
}

// TestBudgetTimeoutScope names the knob the user must turn. Issue #20's whole
// symptom was a message that blamed the reviewer for the author's slowness.
func TestBudgetTimeoutScope(t *testing.T) {
	live := core.NewBudget(50*time.Millisecond, 10*time.Second)
	defer live.Close()
	if got := live.TimeoutScope(); got != core.TimeoutScopeInvocation {
		t.Errorf("TimeoutScope() with a live session = %q, want %q", got, core.TimeoutScopeInvocation)
	}

	spent := core.NewBudget(10*time.Second, 60*time.Millisecond)
	defer spent.Close()
	<-spent.Session().Done()
	if got := spent.TimeoutScope(); got != core.TimeoutScopeSession {
		t.Errorf("TimeoutScope() with a spent session = %q, want %q", got, core.TimeoutScopeSession)
	}
}

// TestBudgetAccessors — the CLI prints these limits in its timeout messages, so
// they must survive normalisation of zero/negative input.
func TestBudgetAccessors(t *testing.T) {
	b := core.NewBudget(8*time.Minute, 20*time.Minute)
	defer b.Close()

	if got := b.PerInvoke(); got != 8*time.Minute {
		t.Errorf("PerInvoke() = %s, want 8m", got)
	}
	if got := b.SessionLimit(); got != 20*time.Minute {
		t.Errorf("SessionLimit() = %s, want 20m", got)
	}

	off := core.NewBudget(-5*time.Second, -5*time.Second)
	defer off.Close()
	if got := off.PerInvoke(); got != 0 {
		t.Errorf("PerInvoke() with a negative timeout = %s, want 0", got)
	}
	if got := off.SessionLimit(); got != 0 {
		t.Errorf("SessionLimit() with a negative timeout = %s, want 0", got)
	}
}

// TestBudgetConcurrentSteps — council dispatches in parallel, so Step must be
// safe to call from several goroutines and hand each one its own window.
func TestBudgetConcurrentSteps(t *testing.T) {
	const perInvoke = 400 * time.Millisecond
	b := core.NewBudget(perInvoke, 10*time.Second)
	defer b.Close()

	var wg sync.WaitGroup
	windows := make([]time.Duration, 8)
	for i := range windows {
		wg.Go(func() {
			step, cancel := b.Step()
			defer cancel()
			if dl, ok := step.Deadline(); ok {
				windows[i] = time.Until(dl)
			}
		})
	}
	wg.Wait()

	for i, w := range windows {
		if w < perInvoke-100*time.Millisecond {
			t.Errorf("concurrent step %d got a %s window, want ~%s", i, w, perInvoke)
		}
	}
}

// TestStepContext covers the free function the engine uses to bound one agent
// call, including the "no bound requested" path.
func TestStepContext(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	bounded, cancelBounded := core.StepContext(parent, 250*time.Millisecond)
	defer cancelBounded()
	assertWindow(t, bounded, 250*time.Millisecond, 60*time.Millisecond, "StepContext with a timeout")

	unbounded, cancelUnbounded := core.StepContext(parent, 0)
	defer cancelUnbounded()
	if _, ok := unbounded.Deadline(); ok {
		t.Error("StepContext(parent, 0) should not add a deadline")
	}

	// A derived context must still follow its parent, or a cancelled pipeline
	// would leave provider subprocesses running.
	cancelParent()
	if unbounded.Err() == nil {
		t.Error("StepContext result ignored parent cancellation")
	}
	if bounded.Err() == nil {
		t.Error("bounded StepContext result ignored parent cancellation")
	}
}

// TestStepContextClipsToParentDeadline — the shorter of the two deadlines wins.
func TestStepContextClipsToParentDeadline(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelParent()

	child, cancelChild := core.StepContext(parent, 10*time.Second)
	defer cancelChild()
	assertWindow(t, child, 100*time.Millisecond, 60*time.Millisecond, "StepContext under a shorter parent")
}

// TestBudgetScopeOf decides which budget bounded a step from the deadlines
// themselves, so a session expiring later — e.g. while a killed subprocess is
// still in its SIGTERM grace period — cannot rewrite the diagnosis after the
// fact. TimeoutScope alone had exactly that race.
func TestBudgetScopeOf(t *testing.T) {
	// Per-invocation window is the shorter one: the step's own timer bounds it.
	invocation := core.NewBudget(100*time.Millisecond, 10*time.Second)
	defer invocation.Close()
	step, cancel := invocation.Step()
	defer cancel()
	if got := invocation.ScopeOf(step); got != core.TimeoutScopeInvocation {
		t.Errorf("ScopeOf(short step) = %q, want %q", got, core.TimeoutScopeInvocation)
	}
	// And it stays "invocation" even once the session is spent.
	<-step.Done()
	if got := invocation.ScopeOf(step); got != core.TimeoutScopeInvocation {
		t.Errorf("ScopeOf after the step expired = %q, want %q", got, core.TimeoutScopeInvocation)
	}

	// Session ceiling is the shorter one: it is what actually bounds the call.
	session := core.NewBudget(10*time.Second, 100*time.Millisecond)
	defer session.Close()
	clipped, cancelClipped := session.Step()
	defer cancelClipped()
	if got := session.ScopeOf(clipped); got != core.TimeoutScopeSession {
		t.Errorf("ScopeOf(clipped step) = %q, want %q", got, core.TimeoutScopeSession)
	}

	// No session ceiling at all — nothing but the invocation can have fired.
	unbounded := core.NewBudget(100*time.Millisecond, 0)
	defer unbounded.Close()
	free, cancelFree := unbounded.Step()
	defer cancelFree()
	if got := unbounded.ScopeOf(free); got != core.TimeoutScopeInvocation {
		t.Errorf("ScopeOf with no session ceiling = %q, want %q", got, core.TimeoutScopeInvocation)
	}

	// Per-invocation disabled under a bounded session: the session bounds it.
	sessionOnly := core.NewBudget(0, time.Second)
	defer sessionOnly.Close()
	inherited, cancelInherited := sessionOnly.Step()
	defer cancelInherited()
	if got := sessionOnly.ScopeOf(inherited); got != core.TimeoutScopeSession {
		t.Errorf("ScopeOf with per-invocation disabled = %q, want %q", got, core.TimeoutScopeSession)
	}
}

// TestSynthesisRecordsChairTimeout — a chair that blew its window is still "an
// agent timed out" even when the fallback rescued the answer. Dropping the
// error let a council report exit 0 for a run that hit a deadline.
func TestSynthesisRecordsChairTimeout(t *testing.T) {
	chair := &probeAgent{name: "claude", sleep: 5 * time.Second}
	fallback := &probeAgent{name: "codex"}
	eng := core.NewEngine([]core.Agent{chair, fallback}, councilCfg("claude", false))

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "a"},
		{Provider: "codex", Content: "b"},
	}
	synth, err := eng.Synthesize(context.Background(), dispatches, nil,
		core.InvokeOptions{Timeout: 120 * time.Millisecond})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if synth.ChairFallback != "codex" {
		t.Fatalf("ChairFallback = %q, want codex", synth.ChairFallback)
	}
	if !errors.Is(synth.ChairError, context.DeadlineExceeded) {
		t.Errorf("ChairError = %v, want DeadlineExceeded — a timed-out chair must reach the exit code", synth.ChairError)
	}
}

// TestSynthesisChairErrorIsNilOnSuccess — the negative case, so a healthy
// council is never downgraded to "partial".
func TestSynthesisChairErrorIsNilOnSuccess(t *testing.T) {
	chair := &probeAgent{name: "claude"}
	eng := core.NewEngine([]core.Agent{chair}, councilCfg("claude", false))

	synth, err := eng.Synthesize(context.Background(), []core.DispatchResult{{Provider: "claude"}}, nil,
		core.InvokeOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if synth.ChairError != nil {
		t.Errorf("ChairError = %v on a successful chair, want nil", synth.ChairError)
	}
	if synth.ChairFallback != "" {
		t.Errorf("ChairFallback = %q on a successful chair, want empty", synth.ChairFallback)
	}
}
