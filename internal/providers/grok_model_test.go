package providers_test

import (
	"context"
	"slices"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// grokModel returns the `-m` value GrokProvider emits for cfg.
func grokModel(t *testing.T, cfg *core.Config) string {
	t.Helper()
	runner := &mockRunner{}
	p := providers.NewGrokProvider(cfg, runner)
	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	return flagValue(runner.capturedArgs, "-m")
}

// The default grok model is declared in four places that nothing keeps in sync:
// the provider constant, the viper default, the plugin's advertised capabilities
// and the YAML skeleton. Bumping one and forgetting the others is silent —
// `--agent grok` would run 4.6 while `status` and the plugin still claimed 4.5.
//
// This is a drift guard, not a bump test: it asserts the two Go-side sources
// AGREE, whatever the value is.
func TestGrokDefaultModelSourcesAgree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := core.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	viperDefault := cfg.Providers["grok"].Model
	// A nil config exercises the provider's own built-in constant, bypassing viper.
	providerDefault := grokModel(t, nil)

	if viperDefault != providerDefault {
		t.Errorf("default model drift: viper says %q, the provider constant says %q — "+
			"both must be bumped together", viperDefault, providerDefault)
	}
}

// A default bump is not a migration. grok-4.5 is still a live model (`grok models`
// lists it), so a user who pinned it must keep getting it — दूतसभा must never
// silently move someone onto a model they did not choose.
func TestGrokExplicitModelPinIsHonored(t *testing.T) {
	// grok-4.5: the real-world case a user hits after this bump ships.
	// grok-9.9-canary: synthetic, so this test can never pass merely because the
	// pinned value happens to equal the current default.
	for _, pin := range []string{"grok-4.5", "grok-9.9-canary"} {
		t.Run(pin, func(t *testing.T) {
			cfg := defaultConfig(t)
			pc := cfg.Providers["grok"]
			pc.Model = pin
			cfg.Providers["grok"] = pc

			if got := grokModel(t, cfg); got != pin {
				t.Errorf("-m = %q, want the pinned %q — a pinned model must never be rewritten", got, pin)
			}
		})
	}
}

// grok-4.6 added `xhigh` to the reasoning-effort set (verified against the real
// binary: "use one of: xhigh, high, medium, low"). दूतसभा does not validate the
// level — the CLI rejects bad values with a clear message — so the contract is
// that whatever the user configures reaches argv unaltered.
func TestGrokEffortPassthroughIncludesXhigh(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		want  string
	}{
		{"xhigh space form", []string{"--reasoning-effort", "xhigh"}, "xhigh"},
		{"xhigh equals form", []string{"--reasoning-effort=xhigh"}, "xhigh"},
		{"xhigh via --effort alias", []string{"--effort", "xhigh"}, "xhigh"},
		{"low still works", []string{"--reasoning-effort", "low"}, "low"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultConfig(t)
			pc := cfg.Providers["grok"]
			pc.Flags = tc.flags
			cfg.Providers["grok"] = pc

			runner := &mockRunner{}
			p := providers.NewGrokProvider(cfg, runner)
			_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})

			if got := flagValue(runner.capturedArgs, "--reasoning-effort"); got != tc.want {
				t.Errorf("--reasoning-effort = %q, want %q", got, tc.want)
			}
			// The effort flag is re-applied by the provider, never duplicated.
			if n := countFlag(runner.capturedArgs, "--reasoning-effort"); n != 1 {
				t.Errorf("--reasoning-effort appears %d times, want 1: %v", n, runner.capturedArgs)
			}
			if slices.Contains(runner.capturedArgs, "--effort") {
				t.Errorf("--effort alias must be normalised away: %v", runner.capturedArgs)
			}
		})
	}
}
