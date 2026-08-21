package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

func TestNeedsMigration(t *testing.T) {
	tests := []struct {
		name string
		cfg  *core.Config
		want bool
	}{
		{"nil config", nil, false},
		{
			"has gemini provider",
			&core.Config{Providers: map[string]core.ProviderConfig{"gemini": {Binary: "gemini"}}},
			true,
		},
		{
			"gemini chair",
			&core.Config{Providers: map[string]core.ProviderConfig{}, Council: core.CouncilConfig{Chair: "gemini"}},
			true,
		},
		{
			"agy only — clean",
			&core.Config{Providers: map[string]core.ProviderConfig{"agy": {Binary: "agy"}}, Council: core.CouncilConfig{Chair: "claude"}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := core.NeedsMigration(tt.cfg); got != tt.want {
				t.Errorf("NeedsMigration() = %v, want %v", got, tt.want)
			}
		})
	}
}

// writeConfig writes content to a temp config file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestMigrateConfigFileRenamesProvider(t *testing.T) {
	path := writeConfig(t, `providers:
  claude:
    binary: claude
    model: claude-opus-4-8
  gemini:
    binary: gemini
    model: gemini-3.1-pro-preview
    flags:
      - --approval-mode
      - yolo
council:
  chair: claude
timeout: 10m
`)

	res, err := core.MigrateConfigFile(path)
	if err != nil {
		t.Fatalf("MigrateConfigFile: %v", err)
	}
	if !res.Changed() {
		t.Fatal("expected Changed() = true")
	}
	if !res.ProviderRenamed {
		t.Error("expected ProviderRenamed = true")
	}
	if res.ChairUpdated {
		t.Error("ChairUpdated should be false when chair was claude")
	}
	if res.BackupPath == "" {
		t.Error("expected a backup path")
	}
	if _, err := os.Stat(res.BackupPath); err != nil {
		t.Errorf("backup file missing: %v", err)
	}

	// Reload the migrated file and verify it now uses agy, not gemini.
	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("reload migrated config: %v", err)
	}
	if _, ok := cfg.Providers["gemini"]; ok {
		t.Error("gemini provider should be gone after migration")
	}
	agy, ok := cfg.Providers["agy"]
	if !ok {
		t.Fatal("agy provider missing after migration")
	}
	if agy.Binary != "agy" {
		t.Errorf("agy.Binary = %q, want agy", agy.Binary)
	}
	if agy.Model != "Gemini 3.7 Flash (High)" {
		t.Errorf("agy.Model = %q, want Gemini 3.7 Flash (High)", agy.Model)
	}
	if len(agy.Flags) != 1 || agy.Flags[0] != "--dangerously-skip-permissions" {
		t.Errorf("agy.Flags = %v, want [--dangerously-skip-permissions]", agy.Flags)
	}

	// claude block must be preserved untouched.
	if cfg.Providers["claude"].Model != "claude-opus-4-8" {
		t.Errorf("claude provider was altered: %+v", cfg.Providers["claude"])
	}
	if cfg.Timeout.Minutes() != 10 {
		t.Errorf("timeout altered: got %v, want 10m", cfg.Timeout)
	}

	// Backup must still contain the original gemini reference.
	backup, err := os.ReadFile(res.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !strings.Contains(string(backup), "gemini") {
		t.Error("backup should preserve the original gemini config")
	}
}

func TestMigrateConfigFileUpdatesChair(t *testing.T) {
	path := writeConfig(t, `providers:
  gemini:
    binary: gemini
council:
  chair: gemini
`)

	res, err := core.MigrateConfigFile(path)
	if err != nil {
		t.Fatalf("MigrateConfigFile: %v", err)
	}
	if !res.ChairUpdated {
		t.Error("expected ChairUpdated = true")
	}
	if !res.ProviderRenamed {
		t.Error("expected ProviderRenamed = true")
	}

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cfg.Council.Chair != "agy" {
		t.Errorf("chair = %q, want agy", cfg.Council.Chair)
	}
}

func TestMigrateConfigFileIdempotent(t *testing.T) {
	path := writeConfig(t, `providers:
  agy:
    binary: agy
    model: Gemini 3.7 Flash (High)
council:
  chair: claude
`)

	res, err := core.MigrateConfigFile(path)
	if err != nil {
		t.Fatalf("MigrateConfigFile: %v", err)
	}
	if res.Changed() {
		t.Error("expected no changes for an already-migrated config")
	}
	if res.BackupPath != "" {
		t.Error("no backup should be written when nothing changes")
	}
}

func TestMigrateConfigFileDropsGeminiWhenAgyExists(t *testing.T) {
	// Both gemini and agy present → gemini dropped, agy kept as-is.
	path := writeConfig(t, `providers:
  gemini:
    binary: gemini
  agy:
    binary: agy
    model: Gemini 3.1 Pro (High)
`)

	res, err := core.MigrateConfigFile(path)
	if err != nil {
		t.Fatalf("MigrateConfigFile: %v", err)
	}
	if !res.ProviderRenamed {
		t.Error("expected ProviderRenamed = true (gemini dropped)")
	}

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, ok := cfg.Providers["gemini"]; ok {
		t.Error("gemini should be dropped")
	}
	// Existing agy customization must be preserved.
	if cfg.Providers["agy"].Model != "Gemini 3.1 Pro (High)" {
		t.Errorf("agy.Model = %q, want preserved Gemini 3.1 Pro (High)", cfg.Providers["agy"].Model)
	}
}

func TestPlanMigrationFileDoesNotWrite(t *testing.T) {
	original := `providers:
  gemini:
    binary: gemini
council:
  chair: gemini
`
	path := writeConfig(t, original)

	plan, err := core.PlanMigrationFile(path)
	if err != nil {
		t.Fatalf("PlanMigrationFile: %v", err)
	}
	if !plan.ProviderRenamed || !plan.ChairUpdated {
		t.Errorf("plan should report both changes, got %+v", plan)
	}
	if plan.BackupPath != "" {
		t.Error("plan must not write a backup")
	}

	// File must be untouched, and no backup created.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != original {
		t.Error("PlanMigrationFile modified the file")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Error("PlanMigrationFile must not create a backup")
	}
}

func TestMigrateConfigFileDoesNotClobberExistingBackup(t *testing.T) {
	path := writeConfig(t, "providers:\n  gemini:\n    binary: gemini\n")
	// A pre-existing backup must not be overwritten.
	if err := os.WriteFile(path+".bak", []byte("PRECIOUS"), 0o600); err != nil {
		t.Fatalf("seed backup: %v", err)
	}

	res, err := core.MigrateConfigFile(path)
	if err != nil {
		t.Fatalf("MigrateConfigFile: %v", err)
	}
	if res.BackupPath == path+".bak" {
		t.Error("must not reuse the pre-existing .bak path")
	}
	// The original .bak content must survive.
	if b, _ := os.ReadFile(path + ".bak"); string(b) != "PRECIOUS" {
		t.Errorf("pre-existing backup was clobbered: %q", b)
	}
	// The new backup must contain the migrated file's original (gemini) content.
	nb, err := os.ReadFile(res.BackupPath)
	if err != nil {
		t.Fatalf("read new backup: %v", err)
	}
	if !strings.Contains(string(nb), "gemini") {
		t.Errorf("new backup %q should hold the original gemini config", res.BackupPath)
	}
}

func TestMigrateConfigFilePreservesSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.yaml")
	if err := os.WriteFile(real, []byte("providers:\n  gemini:\n    binary: gemini\n"), 0o600); err != nil {
		t.Fatalf("write real: %v", err)
	}
	link := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if _, err := core.MigrateConfigFile(link); err != nil {
		t.Fatalf("MigrateConfigFile via symlink: %v", err)
	}

	// The link must still be a symlink (not replaced by a regular file)...
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink was replaced by a regular file")
	}
	// ...and the real target must be migrated.
	cfg, err := core.LoadConfig(real)
	if err != nil {
		t.Fatalf("reload real: %v", err)
	}
	if _, ok := cfg.Providers["agy"]; !ok {
		t.Error("real target was not migrated to agy")
	}
}

func TestMigrateConfigFileMissing(t *testing.T) {
	_, err := core.MigrateConfigFile(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestUserConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := core.UserConfigPath()
	if err != nil {
		t.Fatalf("UserConfigPath: %v", err)
	}
	want := filepath.Join(home, ".config", "dootsabha", "config.yaml")
	if path != want {
		t.Errorf("UserConfigPath() = %q, want %q", path, want)
	}
}
