package cli

import (
	"context"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// providerAgent adapts providers.Provider to core.Agent, breaking the import
// cycle between core and providers.
//
// This is the one place a provider is invoked outside Outcome.Invoke, and it
// lives here rather than in a command file so that rule stays absolute: the
// pipeline guards in outcome_test.go scan command files, and a command that
// called a provider directly would be missed by the exit decision.
//
// Nothing is missed here. The engine owns the step context for these calls, and
// every outcome comes back on the DispatchResult, ReviewResult and
// SynthesisResult the engine returns — which council folds into the ledger with
// Outcome.AddDispatches, AddReviews and AddSynthesis. Their completeness is what
// TestOutcomeConsumesEveryResultError enforces.
type providerAgent struct {
	prov providers.Provider
}

func (a *providerAgent) Name() string { return a.prov.Name() }

func (a *providerAgent) Invoke(ctx context.Context, prompt string, opts core.InvokeOptions) (*core.InvokeResult, error) {
	result, err := a.prov.Invoke(ctx, prompt, providers.InvokeOptions{
		Model:    opts.Model,
		MaxTurns: opts.MaxTurns,
		Timeout:  opts.Timeout,
	})
	if err != nil {
		return nil, err
	}
	return &core.InvokeResult{
		Content:   result.Content,
		Model:     result.Model,
		Duration:  result.Duration,
		CostUSD:   result.CostUSD,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	}, nil
}
