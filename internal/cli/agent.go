package cli

import (
	"context"
	"errors"

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
	prov   providers.Provider
	budget *core.Budget
}

func (a *providerAgent) Name() string { return a.prov.Name() }

func (a *providerAgent) Invoke(ctx context.Context, prompt string, opts core.InvokeOptions) (*core.InvokeResult, error) {
	var result *providers.ProviderResult
	scope := a.budget.ScopeOf(ctx)
	err := progress.call(core.StageOf(ctx), a.prov.Name(), scope, func() (err error) {
		result, err = a.prov.Invoke(ctx, prompt, providers.InvokeOptions{
			Model:    opts.Model,
			MaxTurns: opts.MaxTurns,
			Timeout:  opts.Timeout,
		})
		return err
	})
	if err != nil {
		return nil, &scopedError{scope: scope, err: err}
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

// scopedError carries the budget that bounded an engine call back through the
// engine's result types, so the exit message names the limit measured at the
// call. Asked at exit time instead, the budget blames the session for a call
// that hit --timeout whenever the ceiling has passed since.
type scopedError struct {
	scope string
	err   error
}

func (e *scopedError) Error() string { return e.err.Error() }
func (e *scopedError) Unwrap() error { return e.err }

// deadlineScope finds the scope of the call whose error IS the deadline. An
// error can join several calls — a chair that timed out and a fallback that
// failed otherwise — and errors.As would return whichever comes first.
func deadlineScope(err error) string {
	switch e := err.(type) { //nolint:errorlint // node by node on purpose: errors.As stops at the first scopedError, which may not hold the deadline
	case nil:
		return ""
	case *scopedError:
		if errors.Is(e.err, context.DeadlineExceeded) {
			return e.scope
		}
		return ""
	case interface{ Unwrap() []error }:
		for _, inner := range e.Unwrap() {
			if s := deadlineScope(inner); s != "" {
				return s
			}
		}
		return ""
	default:
		return deadlineScope(errors.Unwrap(err))
	}
}
