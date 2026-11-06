package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// hangProvider returns only when its context ends.
type hangProvider struct{}

func (hangProvider) Name() string { return "grok" }
func (hangProvider) Invoke(ctx context.Context, _ string, _ providers.InvokeOptions) (*providers.ProviderResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangProvider) HealthCheck(context.Context) (*providers.HealthStatus, error) { return nil, nil }

// An engine call that ran out of its own --timeout must be reported as an
// invocation timeout even when the session ceiling has expired by the time the
// exit is decided. Council asked the budget at exit time and blamed the
// session, while the progress file — measured at the call — said invocation.
func TestEngineCallKeepsTheScopeMeasuredAtTheCall(t *testing.T) {
	b := core.NewBudget(30*time.Millisecond, 150*time.Millisecond)
	t.Cleanup(b.Close)
	agent := &providerAgent{prov: hangProvider{}, budget: b}

	step, cancel := core.StepContext(b.Session(), b.PerInvoke())
	_, err := agent.Invoke(step, "p", core.InvokeOptions{})
	cancel()
	<-b.Session().Done() // the session ceiling passes before the run ends

	o := newOutcome(b)
	o.AddDispatches([]core.DispatchResult{{Provider: "grok", Error: fmt.Errorf("invoke grok: %w", err)}})
	msg := o.Exit("x").Error()
	if !strings.HasPrefix(msg, core.TimeoutScopeInvocation+" timeout") {
		t.Errorf("exit message %q blames the wrong limit; the call hit --timeout", msg)
	}
}
