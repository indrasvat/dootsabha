package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

func TestConfigMigrateCmdRewritesGemini(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`providers:
  gemini:
    binary: gemini
    model: gemini-3.1-pro-preview
council:
  chair: gemini
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Point the global --config at our temp file for the duration of the test.
	prev := configFile
	configFile = path
	t.Cleanup(func() { configFile = prev })

	cmd := newConfigMigrateCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("reload migrated config: %v", err)
	}
	if core.NeedsMigration(cfg) {
		t.Error("config still needs migration after running migrate")
	}
	if _, ok := cfg.Providers["agy"]; !ok {
		t.Error("agy provider missing after migrate")
	}
	if cfg.Council.Chair != "agy" {
		t.Errorf("chair = %q, want agy", cfg.Council.Chair)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("expected backup file: %v", err)
	}
}

func TestConfigMigrateCmdDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := `providers:
  gemini:
    binary: gemini
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	prev := configFile
	configFile = path
	t.Cleanup(func() { configFile = prev })

	cmd := newConfigMigrateCmd()
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}

	// Dry-run must not modify the file or write a backup.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != original {
		t.Error("dry-run modified the config file")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Error("dry-run should not write a backup")
	}
	if !strings.Contains(string(got), "gemini") {
		t.Error("config unexpectedly changed")
	}
}

func TestConfigMigrateCmdPareekshanAliasIsDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := "providers:\n  gemini:\n    binary: gemini\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	prev := configFile
	configFile = path
	t.Cleanup(func() { configFile = prev })

	cmd := newConfigMigrateCmd()
	cmd.SetArgs([]string{"--pareekshan"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pareekshan run failed: %v", err)
	}

	// --pareekshan must behave like --dry-run: no write, no backup.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != original {
		t.Error("--pareekshan modified the config (should be dry-run)")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Error("--pareekshan should not write a backup")
	}
}

func TestConfigMigrateCmdExplicitMissingConfigErrors(t *testing.T) {
	prev := configFile
	configFile = filepath.Join(t.TempDir(), "does-not-exist.yaml")
	t.Cleanup(func() { configFile = prev })

	cmd := newConfigMigrateCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for an explicit --config that does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should mention the missing file", err.Error())
	}
}

// Every command carries a Devanagari alias; `config show` was the one that did
// not, which made the rule in CLAUDE.md an aspiration rather than an invariant.
// Found by a दूतसभा council reading the instructions cold.
func TestConfigSubcommandsHaveDevanagariAliases(t *testing.T) {
	want := map[string][]string{
		"show":    {"pradarshan", "प्रदर्शन"},
		"migrate": {"sthaanaantaran", "स्थानांतरण"},
	}
	cmd := newConfigCmd()
	for _, sub := range cmd.Commands() {
		aliases, ok := want[sub.Name()]
		if !ok {
			t.Errorf("config subcommand %q has no expected alias set — add one", sub.Name())
			continue
		}
		for _, alias := range aliases {
			if !slices.Contains(sub.Aliases, alias) {
				t.Errorf("config %s: missing alias %q (has %v)", sub.Name(), alias, sub.Aliases)
			}
		}
	}
}
