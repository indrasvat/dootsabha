package core_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// writeTempConfig writes YAML content to a temp file and returns its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

// isolateHome keeps tests independent of the developer's real
// ~/.config/dootsabha/config.yaml.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestConfigDefaults(t *testing.T) {
	isolateHome(t)

	cfg, err := core.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout: got %v, want 5m", cfg.Timeout)
	}
	if cfg.SessionTimeout != 30*time.Minute {
		t.Errorf("SessionTimeout: got %v, want 30m", cfg.SessionTimeout)
	}
	if cfg.Source.Type != "built-in" || cfg.Source.Path != "" {
		t.Errorf("Source: got %+v, want built-in with empty path", cfg.Source)
	}
	if cfg.Council.Chair != "claude" {
		t.Errorf("Council.Chair: got %q, want %q", cfg.Council.Chair, "claude")
	}
	if !cfg.Council.Parallel {
		t.Error("Council.Parallel: want true")
	}
	if cfg.Council.Rounds != 1 {
		t.Errorf("Council.Rounds: got %d, want 1", cfg.Council.Rounds)
	}

	claude, ok := cfg.Providers["claude"]
	if !ok {
		t.Fatal("providers.claude missing from defaults")
	}
	if claude.Binary != "claude" {
		t.Errorf("claude.Binary: got %q, want %q", claude.Binary, "claude")
	}
	if claude.Model != "claude-opus-4-8" {
		t.Errorf("claude.Model: got %q, want %q", claude.Model, "claude-opus-4-8")
	}
	if len(claude.Flags) == 0 {
		t.Error("claude.Flags: want at least one flag")
	}

	if _, ok := cfg.Providers["codex"]; !ok {
		t.Error("providers.codex missing from defaults")
	}
	if cfg.Providers["codex"].Model != "gpt-5.5" {
		t.Errorf("codex.Model: got %q, want %q", cfg.Providers["codex"].Model, "gpt-5.5")
	}
	if _, ok := cfg.Providers["agy"]; !ok {
		t.Error("providers.agy missing from defaults")
	}
	if cfg.Providers["agy"].Model != "Gemini 3.5 Flash (High)" {
		t.Errorf("agy.Model: got %q, want %q", cfg.Providers["agy"].Model, "Gemini 3.5 Flash (High)")
	}
}

func TestConfigAutoLoadsDefaultUserFile(t *testing.T) {
	home := isolateHome(t)
	configDir := filepath.Join(home, ".config", "dootsabha")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	path := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
providers:
  agy:
    binary: agy
    model: agy-from-home-config
    flags: ["--dangerously-skip-permissions"]
timeout: 10m
`), 0o600); err != nil {
		t.Fatalf("write default config: %v", err)
	}

	cfg, err := core.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Timeout: got %v, want 10m", cfg.Timeout)
	}
	if cfg.Source.Type != "file" || cfg.Source.Path != path {
		t.Errorf("Source: got %+v, want file %q", cfg.Source, path)
	}
	if cfg.Providers["agy"].Model != "agy-from-home-config" {
		t.Errorf("agy.Model: got %q, want agy-from-home-config", cfg.Providers["agy"].Model)
	}
}

func TestConfigExplicitFileOverridesDefaultUserFile(t *testing.T) {
	home := isolateHome(t)
	configDir := filepath.Join(home, ".config", "dootsabha")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(`
timeout: 10m
providers:
  agy:
    model: agy-from-home-config
`), 0o600); err != nil {
		t.Fatalf("write default config: %v", err)
	}
	explicit := writeTempConfig(t, `
timeout: 20m
providers:
  agy:
    model: agy-from-explicit-config
`)

	cfg, err := core.LoadConfig(explicit)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Timeout != 20*time.Minute {
		t.Errorf("Timeout: got %v, want 20m", cfg.Timeout)
	}
	if cfg.Source.Type != "file" || cfg.Source.Path != explicit {
		t.Errorf("Source: got %+v, want file %q", cfg.Source, explicit)
	}
	if cfg.Providers["agy"].Model != "agy-from-explicit-config" {
		t.Errorf("agy.Model: got %q, want agy-from-explicit-config", cfg.Providers["agy"].Model)
	}
}

func TestConfigMissingDefaultUserFileFallsBackToBuiltins(t *testing.T) {
	isolateHome(t)

	cfg, err := core.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout: got %v, want built-in 5m", cfg.Timeout)
	}
	if cfg.Source.Type != "built-in" || cfg.Source.Path != "" {
		t.Errorf("Source: got %+v, want built-in with empty path", cfg.Source)
	}
	if cfg.Providers["agy"].Model != "Gemini 3.5 Flash (High)" {
		t.Errorf("agy.Model: got %q, want built-in Gemini 3.5 Flash (High)", cfg.Providers["agy"].Model)
	}
}

func TestConfigFromFile(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  claude:
    binary: claude
    model: claude-opus-4-7
    flags: ["--dangerously-skip-permissions"]
council:
  chair: codex
  parallel: false
  rounds: 2
timeout: 10m
session_timeout: 1h
`)

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Council.Chair != "codex" {
		t.Errorf("Council.Chair: got %q, want %q", cfg.Council.Chair, "codex")
	}
	if cfg.Council.Parallel {
		t.Error("Council.Parallel: want false")
	}
	if cfg.Council.Rounds != 2 {
		t.Errorf("Council.Rounds: got %d, want 2", cfg.Council.Rounds)
	}
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Timeout: got %v, want 10m", cfg.Timeout)
	}
	if cfg.SessionTimeout != time.Hour {
		t.Errorf("SessionTimeout: got %v, want 1h", cfg.SessionTimeout)
	}
	if cfg.Source.Type != "file" || cfg.Source.Path != path {
		t.Errorf("Source: got %+v, want file %q", cfg.Source, path)
	}

	claude := cfg.Providers["claude"]
	if claude.Model != "claude-opus-4-7" {
		t.Errorf("claude.Model: got %q, want %q", claude.Model, "claude-opus-4-7")
	}
}

func TestConfigEnvOverride(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  claude:
    binary: claude
    model: claude-sonnet-4-6
    flags: []
council:
  chair: claude
  parallel: true
  rounds: 1
timeout: 5m
session_timeout: 30m
`)

	t.Setenv("DOOTSABHA_PROVIDERS_CLAUDE_MODEL", "claude-haiku-4-5-20251001")

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	claude := cfg.Providers["claude"]
	if claude.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("claude.Model: got %q, want %q (env should override file)", claude.Model, "claude-haiku-4-5-20251001")
	}
}

func TestConfigUnknownKeys(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  claude:
    binary: claude
    model: claude-sonnet-4-6
    flags: []
unknown_key: should_be_ignored
future_feature:
  nested: value
council:
  chair: claude
  parallel: true
  rounds: 1
timeout: 5m
session_timeout: 30m
`)

	_, err := core.LoadConfig(path)
	if err != nil {
		t.Errorf("LoadConfig with unknown keys should not error: %v", err)
	}
}

func TestConfigRedaction(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  claude:
    binary: claude
    model: claude-sonnet-4-6
    flags: []
    api_key: secret-api-key-value
council:
  chair: claude
  parallel: true
  rounds: 1
timeout: 5m
session_timeout: 30m
auth_token: my-auth-token
`)

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	view := cfg.RedactedView(false)

	// auth_token at top level should be redacted
	if tok, ok := view["auth_token"]; ok {
		if tok != "[REDACTED]" {
			t.Errorf("auth_token: got %v, want [REDACTED]", tok)
		}
	} else {
		t.Error("auth_token missing from view")
	}

	// providers.claude.api_key should be redacted
	providers, ok := view["providers"].(map[string]any)
	if !ok {
		t.Fatal("providers missing or wrong type in view")
	}
	claude, ok := providers["claude"].(map[string]any)
	if !ok {
		t.Fatal("providers.claude missing or wrong type in view")
	}
	if apiKey, ok := claude["api_key"]; ok {
		if apiKey != "[REDACTED]" {
			t.Errorf("api_key: got %v, want [REDACTED]", apiKey)
		}
	} else {
		t.Error("api_key missing from providers.claude view")
	}

	// Non-sensitive key should not be redacted
	if model, ok := claude["model"]; ok {
		if model == "[REDACTED]" {
			t.Error("model should not be redacted")
		}
	}

	source, ok := view["config_source"].(map[string]any)
	if !ok {
		t.Fatal("config_source missing or wrong type in view")
	}
	if source["type"] != "file" || source["path"] != path {
		t.Errorf("config_source = %#v, want file %q", source, path)
	}
}

func TestConfigReveal(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  claude:
    binary: claude
    model: claude-sonnet-4-6
    flags: []
    auth_token: my-secret-token
council:
  chair: claude
  parallel: true
  rounds: 1
timeout: 5m
session_timeout: 30m
`)

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Without reveal: auth_token should be redacted
	view := cfg.RedactedView(false)
	providers := view["providers"].(map[string]any)
	claude := providers["claude"].(map[string]any)
	if claude["auth_token"] != "[REDACTED]" {
		t.Errorf("auth_token (redacted): got %v, want [REDACTED]", claude["auth_token"])
	}

	// With reveal: actual value returned
	revealView := cfg.RedactedView(true)
	providers2 := revealView["providers"].(map[string]any)
	claude2 := providers2["claude"].(map[string]any)
	if claude2["auth_token"] != "my-secret-token" {
		t.Errorf("auth_token (revealed): got %v, want my-secret-token", claude2["auth_token"])
	}
}

func TestConfigDurationParsing(t *testing.T) {
	path := writeTempConfig(t, `
timeout: 2m30s
session_timeout: 45m
council:
  chair: claude
  parallel: true
  rounds: 1
providers:
  claude:
    binary: claude
    model: claude-sonnet-4-6
    flags: []
`)

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := 2*time.Minute + 30*time.Second
	if cfg.Timeout != want {
		t.Errorf("Timeout: got %v, want %v", cfg.Timeout, want)
	}
	if cfg.SessionTimeout != 45*time.Minute {
		t.Errorf("SessionTimeout: got %v, want 45m", cfg.SessionTimeout)
	}
}

func TestConfigMergeOrder(t *testing.T) {
	// Verify precedence: env > file > default
	// Default: providers.claude.model = "claude-opus-4-8"
	// File: providers.claude.model = "claude-haiku-4-5"
	// Env:  DOOTSABHA_PROVIDERS_CLAUDE_MODEL = "claude-haiku-4-5-20251001"
	// Result should be "claude-haiku-4-5-20251001"
	path := writeTempConfig(t, `
providers:
  claude:
    binary: claude
    model: claude-haiku-4-5
    flags: []
council:
  chair: claude
  parallel: true
  rounds: 1
timeout: 5m
session_timeout: 30m
`)

	t.Setenv("DOOTSABHA_PROVIDERS_CLAUDE_MODEL", "claude-haiku-4-5-20251001")

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Env takes precedence over file
	if cfg.Providers["claude"].Model != "claude-haiku-4-5-20251001" {
		t.Errorf("merge order: got %q, want %q (env > file > default)", cfg.Providers["claude"].Model, "claude-haiku-4-5-20251001")
	}
}

func TestConfigCommentsKeys(t *testing.T) {
	// All default config keys should have a comment.
	requiredKeys := []string{
		"providers.claude.binary", "providers.claude.model", "providers.claude.flags",
		"providers.codex.binary", "providers.codex.model", "providers.codex.flags",
		"providers.agy.binary", "providers.agy.model", "providers.agy.flags",
		"config_source.type", "config_source.path",
		"council.chair", "council.parallel", "council.rounds",
		"timeout", "session_timeout",
	}
	for _, key := range requiredKeys {
		if _, ok := core.ConfigComments[key]; !ok {
			t.Errorf("ConfigComments missing key: %q", key)
		}
	}
}

func TestConfigCommentsNotEmpty(t *testing.T) {
	for key, comment := range core.ConfigComments {
		if comment == "" {
			t.Errorf("ConfigComments[%q] is empty", key)
		}
	}
}

func TestConfigNoFileStillWorks(t *testing.T) {
	isolateHome(t)

	// LoadConfig("") should work with zero-config (embedded defaults).
	cfg, err := core.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig with no file: %v", err)
	}
	if len(cfg.Providers) != 3 {
		t.Errorf("providers count = %d, want 3", len(cfg.Providers))
	}
	if cfg.Timeout == 0 {
		t.Error("timeout should have a default value")
	}
}
