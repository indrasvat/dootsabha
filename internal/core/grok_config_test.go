package core_test

import (
	"slices"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

// grok must be a known built-in provider so `dootsabha status` lists it and
// `--agent grok` resolves without a config file. This does NOT put grok into any
// default pipeline — council/refine/review defaults are unchanged.
func TestGrokProviderDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := core.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	pc, ok := cfg.Providers["grok"]
	if !ok {
		t.Fatalf("providers.grok missing; got %v", slices.Sorted(maps(cfg.Providers)))
	}
	if pc.Binary != "grok" {
		t.Errorf("binary = %q, want grok", pc.Binary)
	}
	if pc.Model != "grok-4.5" {
		t.Errorf("model = %q, want grok-4.5", pc.Model)
	}
	if len(pc.Flags) == 0 {
		t.Error("flags should carry the default reasoning effort")
	}
}

// `config show --commented` asserts every key has a description.
func TestGrokConfigComments(t *testing.T) {
	for _, key := range []string{
		"providers.grok.binary",
		"providers.grok.model",
		"providers.grok.flags",
	} {
		if _, ok := core.ConfigComments[key]; !ok {
			t.Errorf("ConfigComments missing %q", key)
		}
	}
}

// Adding grok must not disturb the existing three providers.
func TestGrokAdditionKeepsExistingProviders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := core.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, name := range []string{"claude", "codex", "agy"} {
		if _, ok := cfg.Providers[name]; !ok {
			t.Errorf("provider %q disappeared after adding grok", name)
		}
	}
}

func maps(m map[string]core.ProviderConfig) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}
