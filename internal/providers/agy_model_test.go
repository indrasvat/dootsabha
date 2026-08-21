package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
)

// agyModel returns the `--model` value AgyProvider emits for cfg.
func agyModel(t *testing.T, cfg *core.Config) string {
	t.Helper()
	runner := &mockRunner{}
	p := providers.NewAgyProvider(cfg, runner)
	_, _ = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	return flagValue(runner.capturedArgs, "--model")
}

// The default agy model was declared in six unsynced places. Three now READ
// providers.AgyDefaultModel (plugin capabilities, extension context, provider
// fallback) and core's two (viper default, migration writer) share one constant,
// so those cannot drift by construction. What remains is guarded here:
//
//   - core's constant, which cannot import providers (core sits below it);
//   - configs/default.yaml, a static file nothing loads at runtime.
//
// A drift guard, not a bump test: it asserts the sources AGREE, whatever the value.
func TestAgyDefaultModelSourcesAgree(t *testing.T) {
	// A nil config exercises the provider's own constant, bypassing viper.
	want := agyModel(t, nil)
	if want != providers.AgyDefaultModel {
		t.Fatalf("provider emits --model %q but AgyDefaultModel is %q", want, providers.AgyDefaultModel)
	}

	t.Run("viper default", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		cfg, err := core.LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if got := cfg.Providers["agy"].Model; got != want {
			t.Errorf("viper default is %q, the provider constant is %q — "+
				"bump defaultAgyModel in internal/core too", got, want)
		}
	})

	// What `dootsabha config migrate` writes into a user's file. A stale value
	// here silently pins every migrated user to the old model.
	t.Run("migration writer", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte("providers:\n  gemini:\n    binary: gemini\n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, err := core.MigrateConfigFile(path); err != nil {
			t.Fatalf("MigrateConfigFile: %v", err)
		}
		v := viper.New()
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			t.Fatalf("reread: %v", err)
		}
		if got := v.GetString("providers.agy.model"); got != want {
			t.Errorf("config migrate writes %q, the provider default is %q", got, want)
		}
	})

	// The skeleton `dootsabha config init` writes and users copy. Nothing loads it
	// at runtime, so it is the surface most likely to be forgotten.
	t.Run("configs/default.yaml skeleton", func(t *testing.T) {
		path := filepath.Join("..", "..", "configs", "default.yaml")

		// Read the FILE ONLY. core.LoadConfig applies setDefaults first, which
		// would paper over a deleted or mis-nested key.
		v := viper.New()
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			t.Fatalf("parse skeleton: %v", err)
		}
		switch got := v.GetString("providers.agy.model"); {
		case got == "":
			t.Errorf("configs/default.yaml has no providers.agy.model key — deleted or mis-nested")
		case got != want:
			t.Errorf("configs/default.yaml ships model %q, the provider default is %q — "+
				"the skeleton is not loaded at runtime, so nothing else would catch this", got, want)
		}

		t.Setenv("HOME", t.TempDir())
		if _, err := core.LoadConfig(path); err != nil {
			t.Errorf("shipped skeleton no longer loads: %v", err)
		}
	})
}

// A default bump is not a migration. `agy models` still lists 3.6/3.5/3.1, so a
// user who pinned one must keep getting it.
func TestAgyExplicitModelPinIsHonored(t *testing.T) {
	// The real-world case, plus a synthetic one so this can never pass merely
	// because the pin happens to equal the current default.
	for _, pin := range []string{"Gemini 3.5 Flash (High)", "Gemini 9.9 Canary (Low)"} {
		t.Run(pin, func(t *testing.T) {
			cfg := defaultConfig(t)
			pc := cfg.Providers["agy"]
			pc.Model = pin
			cfg.Providers["agy"] = pc

			if got := agyModel(t, cfg); got != pin {
				t.Errorf("--model = %q, want the pinned %q", got, pin)
			}
		})
	}
}
