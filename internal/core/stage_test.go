package core_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

// stageAgent records the stage named on each context it is invoked with.
type stageAgent struct {
	name string
	fail bool
	mu   sync.Mutex
	seen []string
}

func (a *stageAgent) Name() string { return a.name }
func (a *stageAgent) Invoke(ctx context.Context, _ string, _ core.InvokeOptions) (*core.InvokeResult, error) {
	a.mu.Lock()
	a.seen = append(a.seen, core.StageOf(ctx))
	a.mu.Unlock()
	if a.fail {
		return nil, errors.New("chair down")
	}
	return &core.InvokeResult{Content: a.name + " out"}, nil
}

// A progress reader is told which stage a call belongs to. The engine is the
// only thing that knows, so it names the stage on the context it hands each
// agent — including the fallback chair, which runs as synthesis too.
func TestEngineNamesTheStageOfEveryCall(t *testing.T) {
	chair := &stageAgent{name: "claude", fail: true}
	other := &stageAgent{name: "codex"}
	cfg := defaultCfg()
	cfg.Council.Chair = "claude"
	eng := core.NewEngine([]core.Agent{chair, other}, cfg)
	ctx := context.Background()

	ds, err := eng.Dispatch(ctx, "p", core.InvokeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Peer review needs two healthy dispatches; the chair failing comes later.
	ds[0].Error = nil
	ds[0].Content = "claude out"
	rs, err := eng.PeerReview(ctx, ds, core.InvokeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Synthesize(ctx, ds, rs, core.InvokeOptions{}); err != nil {
		t.Fatal(err)
	}

	if want := []string{core.StageDispatch, core.StageReview, core.StageSynthesis}; !slices.Equal(chair.seen, want) {
		t.Errorf("chair saw stages %v, want %v", chair.seen, want)
	}
	if want := []string{core.StageDispatch, core.StageReview, core.StageSynthesis}; !slices.Equal(other.seen, want) {
		t.Errorf("fallback saw stages %v, want %v", other.seen, want)
	}
}
