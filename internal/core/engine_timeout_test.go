package core_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// Issue #20: the council pipeline ran dispatch → peer review → synthesis under a
// single deadline, so whatever the dispatch stage spent came out of synthesis's
// pocket. InvokeOptions.Timeout existed but no provider or engine ever read it.
//
// The engine now derives a fresh per-invocation context from opts.Timeout for
// every agent call, parented to the caller's session context.

// probeAgent records the deadline it was handed and can burn time or fail.
type probeAgent struct {
	name  string
	sleep time.Duration
	fail  error

	mu      sync.Mutex
	calls   int
	window  time.Duration
	bounded bool
	handed  context.Context
}

func (p *probeAgent) Name() string { return p.name }

func (p *probeAgent) Invoke(ctx context.Context, _ string, _ core.InvokeOptions) (*core.InvokeResult, error) {
	p.mu.Lock()
	p.calls++
	p.handed = ctx
	if dl, ok := ctx.Deadline(); ok {
		p.bounded = true
		p.window = time.Until(dl)
	} else {
		p.bounded = false
		p.window = 0
	}
	p.mu.Unlock()

	if p.sleep > 0 {
		select {
		case <-time.After(p.sleep):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if p.fail != nil {
		return nil, p.fail
	}
	return &core.InvokeResult{Content: p.name + " output", Model: p.name + "-model"}, nil
}

// snapshot returns the recorded window and whether the context was bounded.
func (p *probeAgent) snapshot() (time.Duration, bool, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.window, p.bounded, p.calls
}

// wantWindow fails unless the agent was handed a bounded context close to want.
func (p *probeAgent) wantWindow(t *testing.T, want, tolerance time.Duration) {
	t.Helper()
	got, bounded, calls := p.snapshot()
	if calls == 0 {
		t.Fatalf("%s: never invoked", p.name)
	}
	if !bounded {
		t.Fatalf("%s: handed a context with NO deadline — opts.Timeout was ignored", p.name)
	}
	if got < want-tolerance || got > want+tolerance {
		t.Errorf("%s: window = %s, want %s ±%s", p.name, got, want, tolerance)
	}
}

func councilCfg(chair string, parallel bool) *core.Config {
	cfg := defaultCfg()
	cfg.Council.Chair = chair
	cfg.Council.Parallel = parallel
	return cfg
}

// TestDispatchBoundsEachAgentByOptsTimeout — the per-invocation budget must reach
// the agent as a real deadline, not merely as an unread struct field.
func TestDispatchBoundsEachAgentByOptsTimeout(t *testing.T) {
	a := &probeAgent{name: "claude"}
	b := &probeAgent{name: "codex"}
	eng := core.NewEngine([]core.Agent{a, b}, councilCfg("claude", true))

	if _, err := eng.Dispatch(context.Background(), "hi", core.InvokeOptions{Timeout: 500 * time.Millisecond}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	a.wantWindow(t, 500*time.Millisecond, 150*time.Millisecond)
	b.wantWindow(t, 500*time.Millisecond, 150*time.Millisecond)
}

// TestSequentialDispatchDoesNotShrinkLaterAgents is issue #20 inside one stage:
// a slow first agent must not eat the second agent's window.
func TestSequentialDispatchDoesNotShrinkLaterAgents(t *testing.T) {
	slow := &probeAgent{name: "codex", sleep: 250 * time.Millisecond}
	next := &probeAgent{name: "claude"}
	eng := core.NewEngine([]core.Agent{slow, next}, councilCfg("claude", false))

	if _, err := eng.Dispatch(context.Background(), "hi", core.InvokeOptions{Timeout: 600 * time.Millisecond}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	next.wantWindow(t, 600*time.Millisecond, 150*time.Millisecond)
}

// TestPeerReviewGetsFreshWindowAfterSlowDispatch — stage 2 starts from zero.
func TestPeerReviewGetsFreshWindowAfterSlowDispatch(t *testing.T) {
	one := &probeAgent{name: "claude", sleep: 200 * time.Millisecond}
	two := &probeAgent{name: "codex", sleep: 200 * time.Millisecond}
	cfg := councilCfg("claude", false)
	eng := core.NewEngine([]core.Agent{one, two}, cfg)

	opts := core.InvokeOptions{Timeout: 600 * time.Millisecond}
	dispatches, err := eng.Dispatch(context.Background(), "hi", opts)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	one.sleep, two.sleep = 0, 0

	if _, err := eng.PeerReview(context.Background(), dispatches, opts); err != nil {
		t.Fatalf("peer review: %v", err)
	}
	one.wantWindow(t, 600*time.Millisecond, 150*time.Millisecond)
	two.wantWindow(t, 600*time.Millisecond, 150*time.Millisecond)
}

// TestSynthesizeGetsFreshWindowAfterSlowStages — stage 3 starts from zero. This
// is the council analogue of the reviewer being starved in `review`.
func TestSynthesizeGetsFreshWindowAfterSlowStages(t *testing.T) {
	chair := &probeAgent{name: "claude", sleep: 300 * time.Millisecond}
	other := &probeAgent{name: "codex", sleep: 300 * time.Millisecond}
	eng := core.NewEngine([]core.Agent{chair, other}, councilCfg("claude", false))

	opts := core.InvokeOptions{Timeout: 700 * time.Millisecond}
	dispatches, err := eng.Dispatch(context.Background(), "hi", opts)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	chair.sleep, other.sleep = 0, 0

	if _, err := eng.Synthesize(context.Background(), dispatches, nil, opts); err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	chair.wantWindow(t, 700*time.Millisecond, 150*time.Millisecond)
}

// TestSynthesisFallbackGetsFreshWindow — the chair burning its whole window must
// not leave the fallback with nothing. That would fail the synthesis twice over
// and report the fallback as broken.
func TestSynthesisFallbackGetsFreshWindow(t *testing.T) {
	chair := &probeAgent{name: "claude", fail: errors.New("chair down")}
	fallback := &probeAgent{name: "codex"}
	eng := core.NewEngine([]core.Agent{chair, fallback}, councilCfg("claude", false))

	dispatches := []core.DispatchResult{
		{Provider: "claude", Content: "a"},
		{Provider: "codex", Content: "b"},
	}
	synth, err := eng.Synthesize(context.Background(), dispatches, nil, core.InvokeOptions{Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if synth.ChairFallback != "codex" {
		t.Fatalf("ChairFallback = %q, want codex", synth.ChairFallback)
	}
	fallback.wantWindow(t, 500*time.Millisecond, 150*time.Millisecond)
}

// TestEngineTimeoutZeroLeavesSessionBoundOnly — an unset per-invocation timeout
// must not invent one, and must not strip the caller's session deadline.
func TestEngineTimeoutZeroLeavesSessionBoundOnly(t *testing.T) {
	a := &probeAgent{name: "claude"}
	eng := core.NewEngine([]core.Agent{a}, councilCfg("claude", false))

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if _, err := eng.Dispatch(ctx, "hi", core.InvokeOptions{Timeout: 0}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	a.wantWindow(t, 400*time.Millisecond, 150*time.Millisecond)
}

// TestEngineStepClippedBySessionDeadline — the session context stays a ceiling
// the per-invocation budget cannot punch through.
func TestEngineStepClippedBySessionDeadline(t *testing.T) {
	a := &probeAgent{name: "claude"}
	eng := core.NewEngine([]core.Agent{a}, councilCfg("claude", false))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := eng.Dispatch(ctx, "hi", core.InvokeOptions{Timeout: 10 * time.Second}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	a.wantWindow(t, 200*time.Millisecond, 150*time.Millisecond)
}

// TestPerAgentTimeoutIsolatesTheSlowAgent — one agent blowing its budget must
// leave its peers' results intact. Under a shared deadline the whole round died.
func TestPerAgentTimeoutIsolatesTheSlowAgent(t *testing.T) {
	stuck := &probeAgent{name: "codex", sleep: 5 * time.Second}
	quick := &probeAgent{name: "claude"}
	eng := core.NewEngine([]core.Agent{stuck, quick}, councilCfg("claude", true))

	results, err := eng.Dispatch(context.Background(), "hi", core.InvokeOptions{Timeout: 150 * time.Millisecond})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	byName := map[string]core.DispatchResult{}
	for _, r := range results {
		byName[r.Provider] = r
	}
	if got := byName["codex"]; got.Error == nil {
		t.Error("the stuck agent should have failed on its own deadline")
	} else if !errors.Is(got.Error, context.DeadlineExceeded) {
		t.Errorf("stuck agent error = %v, want DeadlineExceeded", got.Error)
	}
	if got := byName["claude"]; got.Error != nil {
		t.Errorf("healthy agent failed because a peer timed out: %v", got.Error)
	}
}

// TestEngineReleasesStepContexts — every derived context must be cancelled once
// its invocation returns, or a long council leaks one timer per agent per stage.
func TestEngineReleasesStepContexts(t *testing.T) {
	a := &probeAgent{name: "claude"}
	eng := core.NewEngine([]core.Agent{a}, councilCfg("claude", false))

	if _, err := eng.Dispatch(context.Background(), "hi", core.InvokeOptions{Timeout: 30 * time.Second}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	a.mu.Lock()
	handed := a.handed
	a.mu.Unlock()
	if handed.Err() == nil {
		t.Error("per-invocation context still live after Dispatch returned — cancel was not called")
	}
}
