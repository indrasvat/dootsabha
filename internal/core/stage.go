package core

import "context"

// The stages of a council, as the engine names them on the context it hands
// each agent. A progress reader uses them to say what a run is waiting on.
const (
	StageDispatch  = "dispatch"
	StageReview    = "review"
	StageSynthesis = "synthesis"
)

type stageKey struct{}

func withStage(ctx context.Context, stage string) context.Context {
	return context.WithValue(ctx, stageKey{}, stage)
}

// StageOf returns the stage the engine named on ctx, or "" if none.
func StageOf(ctx context.Context) string {
	s, _ := ctx.Value(stageKey{}).(string)
	return s
}
