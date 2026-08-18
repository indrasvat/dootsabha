package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

// A provider's `binary` and `model` are strings. When one is written as a list or
// a mapping, viper's GetString cast fails, returns "", and the provider silently
// substitutes its built-in default — so दूतसभा runs a DIFFERENT model (or a
// different binary off $PATH) than the config names, while `config show` still
// displays what the user wrote. Exit 0, no warning.
//
// `model: [grok-4.5]` is a plausible typo precisely because the sibling `flags`
// key IS a list. Silently ignoring the pin defeats the one guarantee the
// grok-4.6 default bump makes: an explicit pin is never rewritten.
//
// Per PRD §6.1 an invalid config is exit 6 — fix the config — not a silent
// fallback. Found by an adversarial agent driving the real binary.
func TestProviderLeafTypesMustBeStrings(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		want string
	}{
		{"model as list", "providers:\n  grok:\n    model: [grok-4.5]\n", "providers.grok.model"},
		{"model as mapping", "providers:\n  grok:\n    model:\n      name: grok-4.5\n", "providers.grok.model"},
		{"binary as list", "providers:\n  grok:\n    binary: [/usr/bin/grok]\n", "providers.grok.binary"},
		{"model as list on another provider", "providers:\n  codex:\n    model: [gpt-5.5]\n", "providers.codex.model"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := core.LoadConfig(path)
			if err == nil {
				t.Fatalf("a non-string %s loaded without error — the pin is silently dropped "+
					"and the built-in default runs instead", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q should name the offending key %q", err.Error(), tc.want)
			}
		})
	}
}

// Valid configs must keep loading — `flags` is legitimately a list, and an
// absent key is not an error.
func TestProviderLeafTypesAcceptValidShapes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "providers:\n  grok:\n    binary: /usr/local/bin/grok\n    model: grok-4.5\n" +
		"    flags:\n      - --reasoning-effort\n      - xhigh\n  codex:\n    model: gpt-5.5\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("a well-formed config must load: %v", err)
	}
	if got := cfg.Providers["grok"].Model; got != "grok-4.5" {
		t.Errorf("model = %q, want grok-4.5", got)
	}
	if got := cfg.Providers["grok"].Binary; got != "/usr/local/bin/grok" {
		t.Errorf("binary = %q, want the configured path", got)
	}
}
