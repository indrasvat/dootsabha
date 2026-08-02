package cli

import (
	"context"
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
	msg := timeoutMessage(invocation, context.DeadlineExceeded)
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
	msg = timeoutMessage(session, context.DeadlineExceeded)
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
	msg := timeoutMessage(b, err)
	if !strings.Contains(msg, "claude invoke") {
		t.Errorf("message %q dropped the underlying provider error", msg)
	}
}

// TestTimeoutMessageWithUnboundedLimits — "0 = disabled" must not print as "0s".
func TestTimeoutMessageWithUnboundedLimits(t *testing.T) {
	b := core.NewBudget(0, 0)
	defer b.Close()
	msg := timeoutMessage(b, context.DeadlineExceeded)
	if strings.Contains(msg, "after 0s") {
		t.Errorf("message %q reports a 0s limit for a disabled timeout", msg)
	}
}

// TestBudgetInversionWarning — a per-invocation budget larger than the session
// ceiling silently truncates every call. Say so before 30 minutes are burned.
func TestBudgetInversionWarning(t *testing.T) {
	if w := budgetInversionWarning(40*time.Minute, 30*time.Minute); w == "" {
		t.Error("no warning when --timeout exceeds --session-timeout")
	} else if !strings.Contains(w, "session-timeout") {
		t.Errorf("warning %q should name the flag to raise", w)
	}
	if w := budgetInversionWarning(8*time.Minute, 30*time.Minute); w != "" {
		t.Errorf("unexpected warning for a sane budget: %q", w)
	}
	if w := budgetInversionWarning(40*time.Minute, 0); w != "" {
		t.Errorf("unexpected warning when the session ceiling is disabled: %q", w)
	}
	if w := budgetInversionWarning(30*time.Minute, 30*time.Minute); w != "" {
		t.Errorf("unexpected warning when the budgets are equal: %q", w)
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
