package cli

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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

// The guards below parse each pipeline command's RunE with go/ast rather than
// grepping for substrings.
//
// The first versions matched text, and all three reviewers broke them on paper
// in ways that are trivial in practice: rename a variable from `authorProv` to
// `p` and the direct-invoke ban stops seeing it; use context.WithDeadline, or a
// parent other than Background, and the shared-deadline ban stops seeing it;
// write `&ExitError{Code: 5}` and the code ban stops seeing it. A guard that
// can be evaded by renaming a variable is decoration.

// runEBody returns the body of a command file's RunE, which is where the exit
// decision is made. Rendering helpers elsewhere in the file are not subject to
// these rules — they legitimately return nil.
func runEBody(t *testing.T, file string) (*ast.FuncLit, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var runE *ast.FuncLit
	ast.Inspect(f, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "RunE" {
			if lit, ok := kv.Value.(*ast.FuncLit); ok {
				runE = lit
			}
		}
		return true
	})
	if runE == nil {
		t.Fatalf("%s has no RunE — the detector matched a file it should not have", file)
	}
	return runE, fset
}

// TestPipelineCommandsReachExitThroughOutcome — a pipeline command may not
// decide its own exit code. Three commands doing so is how they ended up with
// three different answers.
//
// Two rules, because either alone leaks. Banning only the owned constants lets
// `return nil` through on a branch that has recorded failures — which is the
// exact regression the first version of this guard missed. Banning only
// `return nil` lets a hand-minted ExitPartial through.
func TestPipelineCommandsReachExitThroughOutcome(t *testing.T) {
	owned := map[string]bool{"ExitProvider": true, "ExitTimeout": true, "ExitPartial": true}

	for _, file := range pipelineCommandFiles(t) {
		runE, fset := runEBody(t, file)
		sawExit := false

		ast.Inspect(runE, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ReturnStmt:
				if len(v.Results) == 1 {
					if id, ok := v.Results[0].(*ast.Ident); ok && id.Name == "nil" {
						t.Errorf("%s: `return nil` inside RunE — every exit must come from "+
							"outcome.Exit, which knows whether anything failed",
							fset.Position(v.Pos()))
					}
				}
			case *ast.SelectorExpr:
				if x, ok := v.X.(*ast.Ident); ok && x.Name == "core" && owned[v.Sel.Name] {
					t.Errorf("%s: mints core.%s itself — that decision belongs to Outcome.Exit, "+
						"which weighs it against every other call in the run",
						fset.Position(v.Pos()), v.Sel.Name)
				}
			case *ast.CallExpr:
				if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Exit" {
					if x, ok := sel.X.(*ast.Ident); ok && x.Name == "outcome" {
						sawExit = true
					}
				}
			}
			return true
		})

		if !sawExit {
			t.Errorf("%s never calls outcome.Exit — it is deciding its own exit code", file)
		}
	}
}

// TestPipelineCommandsInvokeOnlyThroughOutcome — a provider call the ledger does
// not see cannot affect the exit code. Matched on the call shape, so renaming
// the receiver does not evade it.
func TestPipelineCommandsInvokeOnlyThroughOutcome(t *testing.T) {
	for _, file := range pipelineCommandFiles(t) {
		runE, fset := runEBody(t, file)
		ast.Inspect(runE, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Invoke" {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "outcome" {
				return true
			}
			t.Errorf("%s: calls Invoke outside the ledger — route it through outcome.Invoke "+
				"so it gets its own window AND is seen by the exit decision",
				fset.Position(call.Pos()))
			return true
		})
	}
}

// TestPipelineCommandsDoNotShareOneDeadline bans the original issue #20 shape:
// one deadline created up front and reused for every provider call. Any
// context deadline built inside RunE is banned, whatever its parent — the
// budget is the only thing allowed to hand out windows.
func TestPipelineCommandsDoNotShareOneDeadline(t *testing.T) {
	for _, file := range pipelineCommandFiles(t) {
		runE, fset := runEBody(t, file)
		ast.Inspect(runE, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || x.Name != "context" {
				return true
			}
			if sel.Sel.Name == "WithTimeout" || sel.Sel.Name == "WithDeadline" {
				t.Errorf("%s: builds a context.%s inside RunE — that is one deadline for the "+
					"whole pipeline (issue #20); take a fresh window per call from core.Budget",
					fset.Position(call.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
}

// pipelineCommandFiles finds every multi-step command by what it IS — a cobra
// RunE that takes a budget — rather than by a hard-coded list a new command
// would miss. Both signals are required: budget.go takes a budget without being
// a command, and a command without a budget makes no provider calls to weigh.
func pipelineCommandFiles(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var files []string
	for _, file := range matches {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		var hasRunE, takesBudget bool
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.KeyValueExpr:
				if key, ok := v.Key.(*ast.Ident); ok && key.Name == "RunE" {
					if _, ok := v.Value.(*ast.FuncLit); ok {
						hasRunE = true
					}
				}
			case *ast.CallExpr:
				// Both spellings: the CLI helper, and core.NewBudget directly.
				switch fn := v.Fun.(type) {
				case *ast.Ident:
					if fn.Name == "newBudget" {
						takesBudget = true
					}
				case *ast.SelectorExpr:
					if x, ok := fn.X.(*ast.Ident); ok && x.Name == "core" && fn.Sel.Name == "NewBudget" {
						takesBudget = true
					}
				}
			}
			return true
		})
		if hasRunE && takesBudget {
			files = append(files, file)
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
