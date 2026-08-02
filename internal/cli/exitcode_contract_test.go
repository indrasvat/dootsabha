package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// stageExitCode is the single place pipeline failures become exit codes. Before
// it existed, consult/review/refine each mapped context.DeadlineExceeded to 4
// while council did not — so a hung agent surfaced as "synthesis failed" (3),
// reporting the symptom instead of the cause.
func TestStageExitCodeDeadlineOutranksFallback(t *testing.T) {
	ctx := context.Background()

	for _, fallback := range []int{core.ExitProvider, core.ExitPartial, core.ExitError} {
		got := stageExitCode(ctx, context.DeadlineExceeded, fallback)
		if got != core.ExitTimeout {
			t.Errorf("stageExitCode(deadline, fallback=%d) = %d, want %d — a deadline outranks its downstream symptom",
				fallback, got, core.ExitTimeout)
		}
	}
}

// A wrapped deadline must still be recognised — errors.Is, not equality.
func TestStageExitCodeWrappedDeadline(t *testing.T) {
	wrapped := fmt.Errorf("synthesis: %w", context.DeadlineExceeded)
	if got := stageExitCode(context.Background(), wrapped, core.ExitProvider); got != core.ExitTimeout {
		t.Errorf("wrapped deadline = %d, want %d", got, core.ExitTimeout)
	}
}

// The deadline can live on the context rather than the returned error: a
// cancelled context makes downstream stages fail with their own unrelated errors.
func TestStageExitCodeDeadlineOnContext(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), timePast())
	defer cancel()

	got := stageExitCode(ctx, errors.New("no healthy agents available for synthesis"), core.ExitProvider)
	if got != core.ExitTimeout {
		t.Errorf("expired context = %d, want %d — the stage error is a consequence, not the cause", got, core.ExitTimeout)
	}
}

// Without any deadline the caller's own classification stands.
func TestStageExitCodeNoDeadlinePassesFallback(t *testing.T) {
	for _, fallback := range []int{core.ExitProvider, core.ExitPartial} {
		got := stageExitCode(context.Background(), errors.New("provider exploded"), fallback)
		if got != fallback {
			t.Errorf("stageExitCode(plain err, %d) = %d, want %d", fallback, got, fallback)
		}
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

func timePast() time.Time { return time.Now().Add(-time.Hour) }

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
