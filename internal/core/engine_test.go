package core_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// mockAgent implements core.Agent for unit tests.
type mockAgent struct {
	name    string
	result  *core.InvokeResult
	err     error
	invoked bool
}

func (m *mockAgent) Name() string { return m.name }
func (m *mockAgent) Invoke(_ context.Context, _ string, _ core.InvokeOptions) (*core.InvokeResult, error) {
	m.invoked = true
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func defaultCfg() *core.Config {
	cfg, _ := core.LoadConfig("")
	return cfg
}

func sequentialCfg() *core.Config {
	cfg := defaultCfg()
	cfg.Council.Parallel = false
	return cfg
}

func okAgent(name, content string) *mockAgent {
	return &mockAgent{
		name: name,
		result: &core.InvokeResult{
			Content:   content,
			Model:     name + "-model",
			Duration:  100 * time.Millisecond,
			CostUSD:   0.001,
			TokensIn:  10,
			TokensOut: 5,
		},
	}
}

func failAgent(name string) *mockAgent {
	return &mockAgent{
		name: name,
		err:  fmt.Errorf("%s failed", name),
	}
}

func TestDispatchAllSucceed(t *testing.T) {
	agents := []core.Agent{
		okAgent("claude", "c-out"),
		okAgent("codex", "x-out"),
		okAgent("gemini", "g-out"),
	}
	eng := core.NewEngine(agents, defaultCfg())

	results, err := eng.Dispatch(context.Background(), "hello", core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for i, r := range results {
		if r.Error != nil {
			t.Errorf("result[%d] (%s) has error: %v", i, r.Provider, r.Error)
		}
		if r.Content == "" {
			t.Errorf("result[%d] (%s) has empty content", i, r.Provider)
		}
	}
}

func TestDispatchOneFailure(t *testing.T) {
	agents := []core.Agent{
		okAgent("claude", "c-out"),
		failAgent("codex"),
		okAgent("gemini", "g-out"),
	}
	eng := core.NewEngine(agents, defaultCfg())

	results, err := eng.Dispatch(context.Background(), "hello", core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	good := 0
	bad := 0
	for _, r := range results {
		if r.Error != nil {
			bad++
		} else {
			good++
		}
	}
	if good != 2 {
		t.Errorf("good results = %d, want 2", good)
	}
	if bad != 1 {
		t.Errorf("bad results = %d, want 1", bad)
	}
}

func TestDispatchContextCancelled(t *testing.T) {
	ctxAgent := &mockAgent{
		name: "claude",
		err:  context.Canceled,
	}
	agents := []core.Agent{ctxAgent}
	eng := core.NewEngine(agents, defaultCfg())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, err := eng.Dispatch(ctx, "hello", core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected Dispatch error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestDispatchSequentialMode(t *testing.T) {
	agents := []core.Agent{
		okAgent("claude", "c-out"),
		okAgent("codex", "x-out"),
	}
	eng := core.NewEngine(agents, sequentialCfg())

	results, err := eng.Dispatch(context.Background(), "hello", core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("result %s has error: %v", r.Provider, r.Error)
		}
	}
}

func TestDispatchExceedsMaxAgents(t *testing.T) {
	agents := make([]core.Agent, 6)
	for i := range agents {
		agents[i] = okAgent(fmt.Sprintf("agent-%d", i), "out")
	}
	eng := core.NewEngine(agents, defaultCfg())

	_, err := eng.Dispatch(context.Background(), "hello", core.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for >5 agents")
	}
}

func TestDispatchNoProviders(t *testing.T) {
	eng := core.NewEngine(nil, defaultCfg())

	_, err := eng.Dispatch(context.Background(), "hello", core.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error for no providers")
	}
}

func TestDispatchProgressCallback(t *testing.T) {
	agents := []core.Agent{okAgent("claude", "out")}
	eng := core.NewEngine(agents, defaultCfg())

	var events []core.ProgressEvent
	eng.SetProgress(func(_ string, event core.ProgressEvent) {
		events = append(events, event)
	})

	_, err := eng.Dispatch(context.Background(), "hello", core.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0] != core.ProgressStarted {
		t.Errorf("events[0] = %d, want ProgressStarted", events[0])
	}
	if events[1] != core.ProgressDone {
		t.Errorf("events[1] = %d, want ProgressDone", events[1])
	}
}
