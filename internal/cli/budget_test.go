package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// GitHub issue #20. `--timeout` is the budget for ONE provider invocation;
// `--session-timeout` is the ceiling for the whole pipeline. These tests pin the
// resolution order, the flag surface, and the message that tells a user which of
// the two deadlines actually fired.

func cfgWith(timeout, session time.Duration) *core.Config {
	return &core.Config{Timeout: timeout, SessionTimeout: session}
}

func TestResolveTimeoutsPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		flagInvoke  time.Duration
		flagSession time.Duration
		sessionSet  bool
		cfg         *core.Config
		wantInvoke  time.Duration
		wantSession time.Duration
	}{
		{
			name:        "flags win over config",
			flagInvoke:  8 * time.Minute,
			flagSession: 20 * time.Minute,
			sessionSet:  true,
			cfg:         cfgWith(5*time.Minute, 30*time.Minute),
			wantInvoke:  8 * time.Minute,
			wantSession: 20 * time.Minute,
		},
		{
			name:        "config fills unset flags",
			cfg:         cfgWith(5*time.Minute, 30*time.Minute),
			wantInvoke:  5 * time.Minute,
			wantSession: 30 * time.Minute,
		},
		{
			name:        "built-in default when config is empty",
			cfg:         cfgWith(0, 0),
			wantInvoke:  defaultInvokeTimeout,
			wantSession: 0,
		},
		{
			name:        "invocation flag alone leaves the config session ceiling",
			flagInvoke:  90 * time.Second,
			cfg:         cfgWith(5*time.Minute, 30*time.Minute),
			wantInvoke:  90 * time.Second,
			wantSession: 30 * time.Minute,
		},
		{
			name:        "session flag alone leaves the config invocation budget",
			flagSession: 45 * time.Minute,
			sessionSet:  true,
			cfg:         cfgWith(5*time.Minute, 30*time.Minute),
			wantInvoke:  5 * time.Minute,
			wantSession: 45 * time.Minute,
		},
		{
			name:        "explicit --session-timeout 0 disables the ceiling",
			sessionSet:  true,
			flagSession: 0,
			cfg:         cfgWith(5*time.Minute, 30*time.Minute),
			wantInvoke:  5 * time.Minute,
			wantSession: 0,
		},
		{
			name:        "unset --session-timeout does NOT disable the ceiling",
			cfg:         cfgWith(5*time.Minute, 30*time.Minute),
			wantInvoke:  5 * time.Minute,
			wantSession: 30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInvoke, gotSession := resolveTimeouts(tt.flagInvoke, tt.flagSession, tt.sessionSet, tt.cfg)
			if gotInvoke != tt.wantInvoke {
				t.Errorf("per-invocation = %s, want %s", gotInvoke, tt.wantInvoke)
			}
			if gotSession != tt.wantSession {
				t.Errorf("session = %s, want %s", gotSession, tt.wantSession)
			}
		})
	}
}

// TestResolveTimeoutsNeverReturnsExpiredBudget — a negative duration from a
// hand-edited config must not produce a pipeline that dies on its first call.
func TestResolveTimeoutsNeverReturnsExpiredBudget(t *testing.T) {
	invoke, session := resolveTimeouts(0, 0, false, cfgWith(-1*time.Second, -1*time.Second))
	if invoke < 0 {
		t.Errorf("per-invocation = %s, want a non-negative duration", invoke)
	}
	if session < 0 {
		t.Errorf("session = %s, want a non-negative duration", session)
	}
}

// TestTimeoutMessageNamesTheDeadlineThatFired is the heart of issue #20's user
// experience: the old message blamed the reviewer for the author's slowness.
func TestTimeoutMessageNamesTheDeadlineThatFired(t *testing.T) {
	invocation := core.NewBudget(50*time.Millisecond, 10*time.Second)
	defer invocation.Close()
	msg := timeoutMessage(invocation, "", context.DeadlineExceeded)
	if !strings.Contains(msg, "invocation timeout") {
		t.Errorf("message %q should say which deadline fired (invocation)", msg)
	}
	if !strings.Contains(msg, "50ms") {
		t.Errorf("message %q should carry the per-invocation limit", msg)
	}
	if strings.Contains(msg, "session timeout") {
		t.Errorf("message %q blames the session for an invocation timeout", msg)
	}

	session := core.NewBudget(10*time.Second, 60*time.Millisecond)
	defer session.Close()
	<-session.Session().Done()
	msg = timeoutMessage(session, "", context.DeadlineExceeded)
	if !strings.Contains(msg, "session timeout") {
		t.Errorf("message %q should say which deadline fired (session)", msg)
	}
	if !strings.Contains(msg, "60ms") {
		t.Errorf("message %q should carry the session limit", msg)
	}
}

// TestTimeoutMessageCarriesTheUnderlyingError — the provider and subprocess
// detail must survive, so a reader can still see which CLI was killed.
func TestTimeoutMessageCarriesTheUnderlyingError(t *testing.T) {
	b := core.NewBudget(time.Second, time.Minute)
	defer b.Close()
	err := fmt.Errorf("claude invoke: %w", context.DeadlineExceeded)
	msg := timeoutMessage(b, "", err)
	if !strings.Contains(msg, "claude invoke") {
		t.Errorf("message %q dropped the underlying provider error", msg)
	}
}

// TestTimeoutMessageWithUnboundedLimits — "0 = disabled" must not print as "0s".
func TestTimeoutMessageWithUnboundedLimits(t *testing.T) {
	b := core.NewBudget(0, 0)
	defer b.Close()
	msg := timeoutMessage(b, "", context.DeadlineExceeded)
	if strings.Contains(msg, "after 0s") {
		t.Errorf("message %q reports a 0s limit for a disabled timeout", msg)
	}
}

// TestBudgetInversionWarning — a per-invocation budget larger than the session
// ceiling silently truncates every call. Say so before 30 minutes are burned.
func TestBudgetWarning(t *testing.T) {
	// One call cannot fit the ceiling.
	if w := budgetWarning(40*time.Minute, 30*time.Minute, 2); w == "" {
		t.Error("no warning when --timeout exceeds --session-timeout")
	} else if !strings.Contains(w, "session-timeout") {
		t.Errorf("warning %q should name the flag to raise", w)
	}
	// The calls cannot fit TOGETHER — `refine --reviewers a,b,c` is 7 calls, so
	// the shipped defaults (5m each, 30m ceiling) overrun by 5 minutes. This is
	// the shape that would otherwise truncate a pipeline halfway with no notice.
	if w := budgetWarning(5*time.Minute, 30*time.Minute, 7); w == "" {
		t.Error("no warning when N calls of --timeout exceed --session-timeout")
	} else if !strings.Contains(w, "session-timeout") {
		t.Errorf("warning %q should name the flag to raise", w)
	}
	// A council of 3 agents over 2 rounds is 14 calls.
	if w := budgetWarning(5*time.Minute, 30*time.Minute, 14); w == "" {
		t.Error("no warning for a multi-round council under the default ceiling")
	}
	// Coherent budgets stay silent.
	for _, tc := range []struct {
		name             string
		perInvoke, sessn time.Duration
		steps            int
	}{
		{"fits comfortably", 8 * time.Minute, 30 * time.Minute, 2},
		{"ceiling disabled", 40 * time.Minute, 0, 5},
		{"exactly equal", 30 * time.Minute, 30 * time.Minute, 1},
		{"N calls exactly fit", 6 * time.Minute, 30 * time.Minute, 5},
		{"refine with two reviewers fits the defaults", 5 * time.Minute, 30 * time.Minute, 5},
		{"single call, no multiplication", 20 * time.Minute, 30 * time.Minute, 1},
	} {
		if w := budgetWarning(tc.perInvoke, tc.sessn, tc.steps); w != "" {
			t.Errorf("%s: unexpected warning %q", tc.name, w)
		}
	}
}

// TestTimeoutMessageHonoursExplicitScope — the scope is decided when the step
// is created, so a session that expires later (e.g. while a killed subprocess
// is in its SIGTERM grace period) cannot retroactively rewrite the diagnosis.
func TestTimeoutMessageHonoursExplicitScope(t *testing.T) {
	b := core.NewBudget(10*time.Second, 60*time.Millisecond)
	defer b.Close()
	<-b.Session().Done() // session is spent; TimeoutScope() would say "session"

	msg := timeoutMessage(b, core.TimeoutScopeInvocation, context.DeadlineExceeded)
	if !strings.Contains(msg, "invocation timeout") {
		t.Errorf("message %q ignored the scope recorded when the step ran", msg)
	}
	if !strings.Contains(msg, "10s") {
		t.Errorf("message %q should carry the per-invocation limit for an invocation scope", msg)
	}
}

// TestStageFailureMessage — a stage that died on a deadline must name the
// budget; a stage that died for any other reason must not invent one.
func TestStageFailureMessage(t *testing.T) {
	b := core.NewBudget(30*time.Second, 10*time.Minute)
	defer b.Close()

	msg := stageFailureMessage(b, "synthesis", fmt.Errorf("chair: %w", context.DeadlineExceeded))
	if !strings.HasPrefix(msg, "synthesis: ") {
		t.Errorf("message %q lost the stage label", msg)
	}
	if !strings.Contains(msg, "timeout after 30s") {
		t.Errorf("message %q should name the budget that fired", msg)
	}

	msg = stageFailureMessage(b, "peer review", errors.New("provider exploded"))
	if strings.Contains(msg, "timeout") {
		t.Errorf("message %q blamed a timeout for a non-deadline failure", msg)
	}
	if !strings.Contains(msg, "provider exploded") {
		t.Errorf("message %q dropped the underlying error", msg)
	}
}

// TestFirstDeadline — the helper that decides whether exit 4 applies.
func TestFirstDeadline(t *testing.T) {
	other := errors.New("provider exploded")
	wrapped := fmt.Errorf("invoke claude: %w", context.DeadlineExceeded)

	if got := firstDeadline(nil, other, nil); got != nil {
		t.Errorf("firstDeadline with no deadline = %v, want nil", got)
	}
	if got := firstDeadline(nil, other, wrapped); got == nil {
		t.Error("firstDeadline missed a wrapped DeadlineExceeded")
	}
	if got := firstDeadline(); got != nil {
		t.Errorf("firstDeadline() = %v, want nil", got)
	}
	// context.Canceled is NOT a deadline — a user Ctrl-C must not read as
	// "raise your timeout".
	if got := firstDeadline(context.Canceled); got != nil {
		t.Errorf("firstDeadline(Canceled) = %v, want nil", got)
	}
}

// TestSessionTimeoutFlagIsVisible — the flag was hidden because it did nothing
// (TODO(704)). It does something now, so users must be able to discover it.
func TestSessionTimeoutFlagIsVisible(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("session-timeout")
	if f == nil {
		t.Fatal("--session-timeout not registered")
	}
	if f.Hidden {
		t.Error("--session-timeout is still hidden — it is enforced now, so advertise it")
	}
	if !strings.Contains(strings.ToLower(f.Usage), "pipeline") &&
		!strings.Contains(strings.ToLower(f.Usage), "total") {
		t.Errorf("--session-timeout usage %q should distinguish it from --timeout", f.Usage)
	}
}

// TestTimeoutFlagUsageSaysPerInvocation — the two flags are only useful if the
// help text makes the distinction the docs promise.
func TestTimeoutFlagUsageSaysPerInvocation(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("timeout")
	if f == nil {
		t.Fatal("--timeout not registered")
	}
	usage := strings.ToLower(f.Usage)
	if !strings.Contains(usage, "per-agent") && !strings.Contains(usage, "per-invocation") &&
		!strings.Contains(usage, "per agent") && !strings.Contains(usage, "each") {
		t.Errorf("--timeout usage %q should say the budget is per invocation", f.Usage)
	}
}

// TestPipelineCommandsDoNotShareOneDeadline is the freeze guard for issue #20.
//
// The bug was structural: one context.WithTimeout(context.Background(), timeout)
// created up front and reused for every provider call. Any command that goes
// back to that shape reintroduces the bug, whatever its tests say, so the shape
// itself is banned in the multi-step commands.
func TestPipelineCommandsDoNotShareOneDeadline(t *testing.T) {
	for _, file := range []string{"review.go", "refine.go", "council.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(src), "context.WithTimeout(context.Background()") {
			t.Errorf("%s bakes one deadline for the whole pipeline (issue #20) — "+
				"derive a per-invocation context from core.Budget instead", file)
		}
	}
}

// TestSequentialCommandsInvokeOnlyThroughInvokeStep is the stronger half of the
// freeze guard.
//
// Banning the old `context.WithTimeout(context.Background()` spelling does not
// stop the bug coming back a different way — passing `budget.Session()` straight
// to every Invoke reproduces it exactly. review and refine drive their providers
// directly, so the invariant is that EVERY provider call goes through
// invokeStep, which is the only thing that hands out a fresh window.
//
// council is exempt: it invokes through the engine, which derives its own step
// context from InvokeOptions.Timeout (covered by the engine tests).
func TestSequentialCommandsInvokeOnlyThroughInvokeStep(t *testing.T) {
	for _, file := range []string{"review.go", "refine.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "Prov.Invoke(") {
				continue
			}
			t.Errorf("%s:%d calls a provider directly — route it through invokeStep "+
				"so it gets its own window (issue #20):\n\t%s", file, i+1, strings.TrimSpace(line))
		}
		if !strings.Contains(string(src), "invokeStep(budget,") {
			t.Errorf("%s never calls invokeStep — how is it bounding its provider calls?", file)
		}
	}
}
