package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dootsabha/internal/core"
)

// A provider that is merely NOT INSTALLED must be reported, but must not fail the
// whole command when it is opt-in. `grok` is opt-in: most users will never install
// it, and `dootsabha status && ...` must keep working for them.
//
// Regression: adding grok to the built-in provider list made `status` exit 3 on
// every machine without the grok CLI. No test covered the absent-binary path
// because the smoke suite supplies a mock binary for every provider.
func TestStatusExitOptionalProviderNotInstalled(t *testing.T) {
	rows := []healthRow{
		{Name: "claude", Healthy: true, Installed: true},
		{Name: "codex", Healthy: true, Installed: true},
		{Name: "agy", Healthy: true, Installed: true},
		{Name: "grok", Healthy: false, Installed: false, Error: "no such file or directory"},
	}
	if err := statusExitError(rows); err != nil {
		t.Errorf("an absent OPT-IN provider must not fail status, got: %v", err)
	}
}

// An opt-in provider that IS installed but broken (bad auth, wrong version) is a
// real problem the user should hear about.
func TestStatusExitOptionalProviderInstalledButBroken(t *testing.T) {
	rows := []healthRow{
		{Name: "claude", Healthy: true, Installed: true},
		{Name: "grok", Healthy: false, Installed: true, Error: "authentication required"},
	}
	err := statusExitError(rows)
	if err == nil {
		t.Fatal("an installed-but-broken provider must fail status")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 3 {
		t.Errorf("err = %v, want *ExitError{Code:3}", err)
	}
}

// A REQUIRED provider being absent is still a failure — dootsabha cannot do its
// job without its default agents.
func TestStatusExitRequiredProviderNotInstalled(t *testing.T) {
	for _, name := range []string{"claude", "codex", "agy"} {
		rows := []healthRow{
			{Name: name, Healthy: false, Installed: false, Error: "no such file or directory"},
		}
		err := statusExitError(rows)
		if err == nil {
			t.Errorf("absent required provider %q must fail status", name)
			continue
		}
		var ee *ExitError
		if !errors.As(err, &ee) || ee.Code != 3 {
			t.Errorf("%s: err = %v, want *ExitError{Code:3}", name, err)
		}
	}
}

func TestStatusExitAllHealthy(t *testing.T) {
	rows := []healthRow{
		{Name: "claude", Healthy: true, Installed: true},
		{Name: "grok", Healthy: true, Installed: true},
	}
	if err := statusExitError(rows); err != nil {
		t.Errorf("all healthy must exit 0, got: %v", err)
	}
}

// An unknown provider name in config is a config error the user should see,
// regardless of installation.
func TestStatusExitUnknownProviderType(t *testing.T) {
	rows := []healthRow{
		{Name: "claude", Healthy: true, Installed: true},
		{Name: "made-up", Healthy: false, Installed: false, Error: "unknown provider type: made-up"},
	}
	if err := statusExitError(rows); err == nil {
		t.Error("an unknown provider type must fail status even though it is not installed")
	}
}

// The optional set must be explicit, so promoting grok to a default later is a
// deliberate one-line change rather than an accident.
func TestOptionalProvidersMembership(t *testing.T) {
	if !isOptionalProvider("grok") {
		t.Error("grok must be optional — it is opt-in, not a council default")
	}
	for _, name := range []string{"claude", "codex", "agy"} {
		if isOptionalProvider(name) {
			t.Errorf("%q must NOT be optional — it is a council default", name)
		}
	}
}

// An opt-in provider that was never installed should read as "not installed",
// not as a scary FAIL with a fork/exec dump. The distinction matters because
// most users will never install grok and should not think something is broken.
func TestStatusLabelOptionalNotInstalled(t *testing.T) {
	label := statusLabelPlain(healthRow{
		Name: "grok", Healthy: false, Installed: false,
		Error: `subprocess start "/nonexistent/grok": fork/exec /nonexistent/grok: no such file or directory`,
	})
	if !strings.Contains(strings.ToLower(label), "not installed") {
		t.Errorf("label = %q, want it to say 'not installed'", label)
	}
	if strings.Contains(label, "fork/exec") {
		t.Errorf("label = %q must not dump the exec error for a simply-absent optional provider", label)
	}
	if strings.Contains(label, "FAIL") {
		t.Errorf("label = %q must not say FAIL — nothing is broken", label)
	}
}

// An installed-but-broken provider must still surface the real reason.
func TestStatusLabelInstalledButBroken(t *testing.T) {
	label := statusLabelPlain(healthRow{
		Name: "grok", Healthy: false, Installed: true, Error: "authentication required",
	})
	if !strings.Contains(label, "authentication required") {
		t.Errorf("label = %q must carry the real error", label)
	}
}

// A required provider that is absent still reads as a failure with its reason.
func TestStatusLabelRequiredNotInstalled(t *testing.T) {
	label := statusLabelPlain(healthRow{
		Name: "claude", Healthy: false, Installed: false, Error: "no such file or directory",
	})
	if strings.Contains(strings.ToLower(label), "not installed") {
		t.Errorf("label = %q — a REQUIRED provider must not be softened", label)
	}
	if !strings.Contains(label, "no such file or directory") {
		t.Errorf("label = %q must carry the reason", label)
	}
}

func TestStatusLabelHealthy(t *testing.T) {
	if label := statusLabelPlain(healthRow{Name: "grok", Healthy: true, Installed: true}); label != "OK" {
		t.Errorf("label = %q, want OK", label)
	}
}

// --- chair validation -----------------------------------------------------

// `--chair bogus` was silently accepted: council fell back to another agent and
// exited 0, so a typo (or an agent you forgot to install) looked like success and
// the user believed their chosen chair wrote the synthesis. `--agent bogus`
// already errors, so this was also inconsistent.
func TestValidateChairRejectsUnknownName(t *testing.T) {
	err := validateChair("bogus")
	if err == nil {
		t.Fatal("an unknown chair name must be rejected")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error %q should name the offending value", err.Error())
	}
	if !strings.Contains(err.Error(), "grok") {
		t.Errorf("error %q should list the valid providers", err.Error())
	}
}

func TestValidateChairAcceptsKnownNames(t *testing.T) {
	for _, name := range []string{"claude", "codex", "agy", "grok"} {
		if err := validateChair(name); err != nil {
			t.Errorf("validateChair(%q) = %v, want nil", name, err)
		}
	}
}

// An empty chair means "use the config default" and must stay valid.
func TestValidateChairAllowsEmpty(t *testing.T) {
	if err := validateChair(""); err != nil {
		t.Errorf("empty chair must be allowed (config default), got %v", err)
	}
}

// --- usage-error classification -------------------------------------------

// Usage errors must be typed at their source, not recognised afterwards by
// matching Cobra's error text. Substring matching is brittle in both directions:
// an internal error mentioning "invalid argument" would be misreported as a
// usage error, and a usage error Cobra words differently would fall through to
// the internal-error code.
func TestUsageArgsWrapsValidatorErrorAsExitUsage(t *testing.T) {
	validator := usageArgs(cobra.ExactArgs(1))

	// Too few args → must be a typed usage error.
	err := validator(&cobra.Command{}, []string{})
	if err == nil {
		t.Fatal("expected an error for wrong arg count")
	}
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %T, want *ExitError so Execute() need not guess", err)
	}
	if ee.Code != core.ExitUsage {
		t.Errorf("Code = %d, want %d (ExitUsage)", ee.Code, core.ExitUsage)
	}
	if ee.Message == "" {
		t.Error("usage error must carry Cobra's explanation")
	}
}

func TestUsageArgsPassesValidInput(t *testing.T) {
	validator := usageArgs(cobra.ExactArgs(1))
	if err := validator(&cobra.Command{}, []string{"prompt"}); err != nil {
		t.Errorf("valid arg count should pass, got %v", err)
	}
}

func TestUsageArgsNoArgs(t *testing.T) {
	validator := usageArgs(cobra.NoArgs)
	if err := validator(&cobra.Command{}, nil); err != nil {
		t.Errorf("no args should pass NoArgs, got %v", err)
	}
	var ee *ExitError
	if err := validator(&cobra.Command{}, []string{"unexpected"}); !errors.As(err, &ee) || ee.Code != core.ExitUsage {
		t.Errorf("surplus args must yield ExitUsage, got %v", err)
	}
}
