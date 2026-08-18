package providers_test

import (
	"context"
	"path/filepath"
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

// The default grok model used to be declared in four places that nothing kept in
// sync: the provider constant, the viper default, the plugin's advertised
// capabilities and the YAML skeleton. Bumping one and forgetting the others is
// silent — `status` would claim one model while `--agent grok` ran another.
//
// The plugin and the extension-context defaults now READ providers.GrokDefaultModel
// instead of repeating it, so those two can no longer drift by construction. Two
// independent sources remain and are guarded here:
//
//   - the viper default, which lives in internal/core and cannot import providers
//     (core sits below providers, so the dependency would cycle);
//   - configs/default.yaml, which is a static file shipped to users.
//
// This is a drift guard, not a bump test: it asserts the sources AGREE, whatever
// the value is.
func TestGrokDefaultModelSourcesAgree(t *testing.T) {
	// A nil config exercises the provider's own constant, bypassing viper entirely.
	want := grokModel(t, nil)
	if want != providers.GrokDefaultModel {
		t.Fatalf("provider emits -m %q but GrokDefaultModel is %q", want, providers.GrokDefaultModel)
	}

	t.Run("viper default", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		cfg, err := core.LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if got := cfg.Providers["grok"].Model; got != want {
			t.Errorf("viper default is %q, the provider constant is %q — "+
				"bump internal/core/config.go setDefaults too", got, want)
		}
	})

	// The YAML skeleton is what `dootsabha config init` writes and what users copy.
	// Nothing loads it at runtime, so it is the surface most likely to be forgotten
	// — and forgetting it hands every new install a stale model.
	t.Run("configs/default.yaml skeleton", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		// Loaded through the app's own loader, so this also proves the shipped
		// skeleton still parses — not just that a string appears in the file.
		skeleton, err := core.LoadConfig(filepath.Join("..", "..", "configs", "default.yaml"))
		if err != nil {
			t.Fatalf("load skeleton: %v", err)
		}
		if got := skeleton.Providers["grok"].Model; got != want {
			t.Errorf("configs/default.yaml ships model %q, the provider default is %q — "+
				"the skeleton is not loaded at runtime, so nothing else would catch this", got, want)
		}
	})
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
