package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

// The exit-code contract these tests pinned belonged to stageExitCode, the
// helper that turned ONE pipeline-stage failure into a code. Outcome.Exit is now
// the single place that decision is made — for every command, over every call —
// so the same properties are asserted against it.

// A deadline outranks whatever downstream symptom it caused. Before this was
// pinned, a hung agent surfaced as "synthesis failed" (3) — the consequence
// reported instead of the cause.
func TestExitDeadlineOutranksEveryOtherOutcome(t *testing.T) {
	deadline := fmt.Errorf("invoke claude: %w", context.DeadlineExceeded)

	for _, other := range []error{
		nil,
		errors.New("provider exploded"),
		errors.New("no healthy agents available for synthesis"),
		context.Canceled,
	} {
		o := outcomeWith(t, call{provider: "claude", err: deadline}, call{provider: "codex", err: other})
		if got := exitCodeOf(t, o.Exit("x")); got != core.ExitTimeout {
			t.Errorf("with a deadline alongside %v: exit = %d, want %d", other, got, core.ExitTimeout)
		}
	}
}

// Recognised by errors.Is, not equality — every real deadline arrives wrapped
// in the provider and subprocess layers that produced it.
func TestExitRecognisesADeeplyWrappedDeadline(t *testing.T) {
	wrapped := fmt.Errorf("synthesis: %w",
		fmt.Errorf("invoke claude: %w",
			fmt.Errorf("subprocess %q: %w", "claude", context.DeadlineExceeded)))

	o := outcomeWith(t, call{provider: "claude", err: wrapped})
	if got := exitCodeOf(t, o.Exit("x")); got != core.ExitTimeout {
		t.Errorf("wrapped deadline = %d, want %d", got, core.ExitTimeout)
	}
}

// Without any deadline the classification falls to whether output survived.
func TestExitWithoutADeadlineClassifiesByUsableOutput(t *testing.T) {
	boom := errors.New("provider exploded")

	partial := outcomeWith(t, call{provider: "claude"}, call{provider: "codex", err: boom})
	if got := exitCodeOf(t, partial.Exit("x")); got != core.ExitPartial {
		t.Errorf("some failed = %d, want %d", got, core.ExitPartial)
	}

	total := outcomeWith(t, call{provider: "claude", err: boom}, call{provider: "codex", err: boom})
	if got := exitCodeOf(t, total.Exit("x")); got != core.ExitProvider {
		t.Errorf("all failed = %d, want %d", got, core.ExitProvider)
	}
}

// Guards the normalisation: every exit code in the CLI must come from a
// core.Exit* constant. Bare numbers are how `status` silently drifted to
// returning 5 for a config error while five other commands returned 6.
func TestNoBareNumericExitCodes(t *testing.T) {
	bare := regexp.MustCompile(`ExitError\{Code:\s*\d`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if bare.MatchString(line) {
				t.Errorf("%s:%d uses a bare numeric exit code — use a core.Exit* constant:\n  %s",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// providerInstalled distinguishes "never installed" from "installed but broken",
// which is what stops an absent opt-in provider failing `status`.
func TestProviderInstalled(t *testing.T) {
	cfg := &core.Config{Providers: map[string]core.ProviderConfig{
		"real":    {Binary: "sh"},                // certainly on PATH
		"missing": {Binary: "/nonexistent/nope"}, // absolute, absent
		"blank":   {Binary: ""},                  // falls back to the provider name
	}}

	if !providerInstalled(cfg, "real") {
		t.Error(`providerInstalled("real") = false, want true (sh is on PATH)`)
	}
	if providerInstalled(cfg, "missing") {
		t.Error(`providerInstalled("missing") = true, want false`)
	}
	if providerInstalled(cfg, "definitely-not-a-binary-xyz") {
		t.Error("an unconfigured, nonexistent provider must not report installed")
	}
}

func TestProviderInstalledNilConfig(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil config panicked: %v", r)
		}
	}()
	_ = providerInstalled(nil, "sh")
}

// Every command rejects an unknown agent name with ExitUsage — refine used to
// treat one as a failed reviewer instead, downgrading a typo to a "partial
// result" (5) AFTER spending a real model call on the author. A misspelled
// --reviewers should fail before any work starts, like --agents and --chair do.
func TestValidateAgentNamesRejectsUnknown(t *testing.T) {
	err := validateAgentNames([]string{"claude", "definitely-not-an-agent"}, "--reviewers")
	if err == nil {
		t.Fatal("an unknown agent name must be rejected")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != core.ExitUsage {
		t.Fatalf("err = %v, want *ExitError{Code:%d}", err, core.ExitUsage)
	}
	if !strings.Contains(ee.Message, "definitely-not-an-agent") {
		t.Errorf("message %q must name the offending value", ee.Message)
	}
	if !strings.Contains(ee.Message, "--reviewers") {
		t.Errorf("message %q must name the flag it came from", ee.Message)
	}
}

func TestValidateAgentNamesAcceptsKnown(t *testing.T) {
	if err := validateAgentNames([]string{"claude", "codex", "agy", "grok"}, "--agents"); err != nil {
		t.Errorf("all known agents must validate, got %v", err)
	}
}

func TestValidateAgentNamesEmptyEntry(t *testing.T) {
	// "codex,,agy" — a stray comma must be a usage error, not a silent skip.
	err := validateAgentNames([]string{"codex", "", "agy"}, "--agents")
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != core.ExitUsage {
		t.Errorf("empty agent name must be ExitUsage, got %v", err)
	}
}

// `dootsabha` with no args prints help and exits 0 — conventional. But
// `dootsabha --json` is an agent asking for a machine-readable answer; printing
// help text to stdout breaks the one-document guarantee and reports success.
func TestRootNoSubcommandJSONIsUsageError(t *testing.T) {
	err := rootNoCommandError(true)
	if err == nil {
		t.Fatal("--json with no subcommand must be a usage error")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != core.ExitUsage {
		t.Errorf("err = %v, want ExitUsage", err)
	}
}

func TestRootNoSubcommandTextShowsHelp(t *testing.T) {
	if err := rootNoCommandError(false); err != nil {
		t.Errorf("bare invocation should fall through to help, got %v", err)
	}
}

// The AUTH column showed ✓ purely because `--version` exited 0 — it asserted
// something never checked. A stub that exits 0 with no output reported
// "Healthy: true, Auth: ✓". The column now says what is actually known:
// the CLI is reachable.
func TestHealthRowReachableNotAuth(t *testing.T) {
	row := healthRow{Name: "grok", Healthy: true, Installed: true, Reachable: "✓"}
	if row.Reachable != "✓" {
		t.Errorf("Reachable = %q", row.Reachable)
	}

	// The struct must not carry an `Auth` field claiming verified credentials.
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), `"Auth"`) {
		t.Errorf("JSON still exposes an Auth field, which nothing verifies: %s", b)
	}
	if !strings.Contains(string(b), `"Reachable"`) {
		t.Errorf("JSON should expose Reachable: %s", b)
	}
}

// The chair must be validated after every source is resolved. Validating only
// on `--chair` meant an unknown chair from YAML or DOOTSABHA_COUNCIL_CHAIR
// slipped through, and synthesis silently fell back to another agent while
// reporting success.
func TestValidateChairCoversResolvedValue(t *testing.T) {
	if err := validateChair("definitely-not-an-agent"); err == nil {
		t.Fatal("an unknown chair must be rejected regardless of where it came from")
	}
	for _, ok := range []string{"", "claude", "grok"} {
		if err := validateChair(ok); err != nil {
			t.Errorf("validateChair(%q) = %v, want nil", ok, err)
		}
	}
}
