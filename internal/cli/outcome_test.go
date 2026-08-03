package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// ── The completeness guarantee ────────────────────────────────────────────────

// TestOutcomeConsumesEveryResultError is the freeze guard for the defect class
// this type exists to close.
//
// Four of the six defects in this change were the same one: a failure recorded
// on a result type that the exit decision never read. The result types already
// carry every error; what was missing was anything forcing the aggregator to be
// updated when a new one appears. This asserts that every error-typed field on
// every pipeline result is folded into the ledger by an Add* method.
//
// Adding `ReviewResult.SecondaryError` and forgetting AddReviews now fails the
// build. Modelled on the repo's existing TestProtoFieldCoverage_* guards, which
// are what caught SynthesisResult.ChairError not crossing the plugin boundary.
func TestOutcomeConsumesEveryResultError(t *testing.T) {
	consumed := map[reflect.Type]map[string]bool{
		reflect.TypeFor[core.DispatchResult]():  {"Error": true},
		reflect.TypeFor[core.ReviewResult]():    {"Error": true},
		reflect.TypeFor[core.SynthesisResult](): {"ChairError": true},
	}
	errType := reflect.TypeFor[error]()

	for typ, seen := range consumed {
		for field := range typ.Fields() {
			if !field.IsExported() || field.Type != errType {
				continue
			}
			if !seen[field.Name] {
				t.Errorf("%s.%s is an error the exit decision never reads — fold it into "+
					"Outcome.Add%s, or a failure recorded there will exit 0",
					typ.Name(), field.Name, strings.TrimSuffix(typ.Name(), "Result"))
			}
		}
		// The mirror: a name listed here that no longer exists means the guard
		// has quietly stopped guarding anything.
		for name := range seen {
			if _, ok := typ.FieldByName(name); !ok {
				t.Errorf("%s.%s is listed as consumed but no longer exists", typ.Name(), name)
			}
		}
	}
}

// TestPipelineCommandsReachExitThroughOutcome — every pipeline command must get
// its exit code from the one aggregator. Three commands hand-rolling this is
// how they ended up with three different answers.
//
// The check is that a command may not MINT the codes Outcome owns. Merely
// asserting the file mentions outcome.Exit is too weak: council still mentions
// it on its early-return paths, so a hand-rolled `return nil` on the success
// path slipped straight through when this guard was written that way.
//
// Pre-flight codes (2 usage, 6 config, 1 internal) stay a command's own
// business — they are decided before any provider runs.
func TestPipelineCommandsReachExitThroughOutcome(t *testing.T) {
	owned := []string{"core.ExitProvider", "core.ExitTimeout", "core.ExitPartial"}

	for _, file := range pipelineCommandFiles(t) {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.Contains(string(src), "outcome.Exit(") {
			t.Errorf("%s never calls outcome.Exit — it is deciding its own exit code", file)
		}
		for i, line := range strings.Split(string(src), "\n") {
			for _, code := range owned {
				if strings.Contains(line, code) {
					t.Errorf("%s:%d mints %s itself — that decision belongs to Outcome.Exit, "+
						"which weighs it against every other call in the run:\n\t%s",
						file, i+1, code, strings.TrimSpace(line))
				}
			}
		}
	}
}

// TestPipelineCommandsInvokeOnlyThroughOutcome bans calling a provider outside
// the recording choke point, in ANY pipeline command rather than a fixed list —
// so a fourth command added tomorrow is covered without editing this test.
func TestPipelineCommandsInvokeOnlyThroughOutcome(t *testing.T) {
	for _, file := range pipelineCommandFiles(t) {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "Prov.Invoke(") && !strings.Contains(line, "prov.Invoke(") {
				continue
			}
			t.Errorf("%s:%d calls a provider directly — route it through outcome.Invoke so "+
				"it gets its own window AND is seen by the exit decision:\n\t%s",
				file, i+1, strings.TrimSpace(line))
		}
	}
}

// TestPipelineCommandsDoNotShareOneDeadline bans the original issue #20 shape:
// one context.WithTimeout created up front and reused for every provider call.
func TestPipelineCommandsDoNotShareOneDeadline(t *testing.T) {
	for _, file := range pipelineCommandFiles(t) {
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

// pipelineCommandFiles finds every multi-step command by what it does — builds
// a budget — rather than by a hard-coded list that a new command would miss.
func pipelineCommandFiles(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var files []string
	for _, f := range matches {
		if strings.HasSuffix(f, "_test.go") || f == "outcome.go" || f == "budget.go" {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if strings.Contains(string(src), "newBudget(cmd,") {
			files = append(files, f)
		}
	}
	if len(files) < 3 {
		t.Fatalf("found %d pipeline commands (%v), expected at least review/refine/council — "+
			"the detector is broken and these guards are checking nothing", len(files), files)
	}
	return files
}

// ── The decision ─────────────────────────────────────────────────────────────

func outcomeWith(t *testing.T, calls ...call) *Outcome {
	t.Helper()
	b := core.NewBudget(8*time.Minute, 20*time.Minute)
	t.Cleanup(b.Close)
	o := newOutcome(b)
	for _, c := range calls {
		o.record(c.provider, c.scope, c.err)
	}
	return o
}

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return core.ExitSuccess
	}
	var e *ExitError
	if !errors.As(err, &e) {
		t.Fatalf("Exit returned a plain error, not an ExitError: %v", err)
	}
	return e.Code
}

func TestOutcomeExitCodes(t *testing.T) {
	deadline := fmt.Errorf("claude invoke: %w", context.DeadlineExceeded)
	boom := errors.New("provider exploded")

	tests := []struct {
		name  string
		calls []call
		want  int
	}{
		{"nothing ran", nil, core.ExitSuccess},
		{"all succeeded", []call{{provider: "codex"}, {provider: "claude"}}, core.ExitSuccess},
		{
			name:  "some failed, output survives",
			calls: []call{{provider: "codex"}, {provider: "claude", err: boom}},
			want:  core.ExitPartial,
		},
		{
			name:  "every call failed",
			calls: []call{{provider: "codex", err: boom}, {provider: "claude", err: boom}},
			want:  core.ExitProvider,
		},
		{
			name:  "a deadline outranks the partial it causes",
			calls: []call{{provider: "codex"}, {provider: "claude", err: deadline}},
			want:  core.ExitTimeout,
		},
		{
			name:  "a deadline outranks total failure too",
			calls: []call{{provider: "codex", err: boom}, {provider: "claude", err: deadline}},
			want:  core.ExitTimeout,
		},
		{
			name:  "a deadline anywhere counts, not just the last call",
			calls: []call{{provider: "codex", err: deadline}, {provider: "claude"}},
			want:  core.ExitTimeout,
		},
		{
			name:  "cancellation is not a deadline",
			calls: []call{{provider: "codex"}, {provider: "claude", err: context.Canceled}},
			want:  core.ExitPartial,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exitCodeOf(t, outcomeWith(t, tt.calls...).Exit("some agents failed"))
			if got != tt.want {
				t.Errorf("Exit() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestOutcomeExitNamesTheScopeRecordedAtTheCall — the scope captured when the
// call ran wins over whatever the budget would say later.
func TestOutcomeExitNamesTheScopeRecordedAtTheCall(t *testing.T) {
	b := core.NewBudget(10*time.Second, 60*time.Millisecond)
	defer b.Close()
	<-b.Session().Done() // budget would now answer "session"

	o := newOutcome(b)
	o.record("claude", core.TimeoutScopeInvocation, fmt.Errorf("x: %w", context.DeadlineExceeded))

	err := o.Exit("failed")
	if !strings.Contains(err.Error(), "invocation timeout") {
		t.Errorf("message %q ignored the scope recorded at the call", err)
	}
}

// TestOutcomeAddSynthesisRecordsAReplacedChair — the defect that shipped: a
// chair that timed out and was replaced still timed out.
func TestOutcomeAddSynthesisRecordsAReplacedChair(t *testing.T) {
	b := core.NewBudget(time.Minute, time.Hour)
	defer b.Close()
	o := newOutcome(b)
	o.AddDispatches([]core.DispatchResult{{Provider: "claude"}, {Provider: "codex"}})
	o.AddSynthesis(&core.SynthesisResult{
		Chair:         "claude",
		ChairFallback: "codex",
		ChairError:    fmt.Errorf("claude invoke: %w", context.DeadlineExceeded),
	})

	if got := exitCodeOf(t, o.Exit("some agents failed")); got != core.ExitTimeout {
		t.Errorf("Exit() = %d, want %d — a replaced chair still timed out", got, core.ExitTimeout)
	}
}

// TestOutcomeAddSynthesisCleanChairIsNotAFailure — the negative case, so a
// healthy council is never downgraded.
func TestOutcomeAddSynthesisCleanChairIsNotAFailure(t *testing.T) {
	b := core.NewBudget(time.Minute, time.Hour)
	defer b.Close()
	o := newOutcome(b)
	o.AddDispatches([]core.DispatchResult{{Provider: "claude"}})
	o.AddSynthesis(&core.SynthesisResult{Chair: "claude"})

	if got := exitCodeOf(t, o.Exit("x")); got != core.ExitSuccess {
		t.Errorf("Exit() = %d, want 0 for a healthy council", got)
	}
}

// TestOutcomeFailRecordsANonInvocation — an agent that could not be built never
// reaches Invoke, and used to be invisible to the decision.
func TestOutcomeFailRecordsANonInvocation(t *testing.T) {
	o := outcomeWith(t, call{provider: "claude"})
	o.Fail("nosuch", errors.New("unknown agent"))

	if got := exitCodeOf(t, o.Exit("a reviewer could not be built")); got != core.ExitPartial {
		t.Errorf("Exit() = %d, want %d", got, core.ExitPartial)
	}
}

// TestOutcomeAccumulatesAcrossRounds — a failure in an early round must survive
// a healthy later one. Reassigning instead of appending is how the multi-round
// chair timeout was erased.
func TestOutcomeAccumulatesAcrossRounds(t *testing.T) {
	b := core.NewBudget(time.Minute, time.Hour)
	defer b.Close()
	o := newOutcome(b)

	o.AddSynthesis(&core.SynthesisResult{
		Chair: "claude", ChairFallback: "codex",
		ChairError: fmt.Errorf("r1: %w", context.DeadlineExceeded),
	})
	for range 2 {
		o.AddDispatches([]core.DispatchResult{{Provider: "claude"}})
		o.AddSynthesis(&core.SynthesisResult{Chair: "claude"})
	}

	if got := exitCodeOf(t, o.Exit("x")); got != core.ExitTimeout {
		t.Errorf("Exit() = %d, want %d — round 1's timeout was erased", got, core.ExitTimeout)
	}
}
