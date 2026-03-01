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

func TestConfigDefaults(t *testing.T) {
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
	if claude.Model != "claude-sonnet-4-6" {
		t.Errorf("claude.Model: got %q, want %q", claude.Model, "claude-sonnet-4-6")
	}
	if len(claude.Flags) == 0 {
		t.Error("claude.Flags: want at least one flag")
	}

	if _, ok := cfg.Providers["codex"]; !ok {
		t.Error("providers.codex missing from defaults")
	}
	if _, ok := cfg.Providers["gemini"]; !ok {
		t.Error("providers.gemini missing from defaults")
	}
}

func TestConfigFromFile(t *testing.T) {
	path := writeTempConfig(t, `
providers:
  claude:
    binary: claude
    model: claude-opus-4-6
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

	claude := cfg.Providers["claude"]
	if claude.Model != "claude-opus-4-6" {
		t.Errorf("claude.Model: got %q, want %q", claude.Model, "claude-opus-4-6")
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

	t.Setenv("DOOTSABHA_PROVIDERS_CLAUDE_MODEL", "opus-4-6")

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	claude := cfg.Providers["claude"]
	if claude.Model != "opus-4-6" {
		t.Errorf("claude.Model: got %q, want %q (env should override file)", claude.Model, "opus-4-6")
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
	// Default: providers.claude.model = "claude-sonnet-4-6"
	// File: providers.claude.model = "claude-haiku-4-5"
	// Env:  DOOTSABHA_PROVIDERS_CLAUDE_MODEL = "opus-4-6"
	// Result should be "opus-4-6"
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

	t.Setenv("DOOTSABHA_PROVIDERS_CLAUDE_MODEL", "opus-4-6")

	cfg, err := core.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Env takes precedence over file
	if cfg.Providers["claude"].Model != "opus-4-6" {
		t.Errorf("merge order: got %q, want %q (env > file > default)", cfg.Providers["claude"].Model, "opus-4-6")
	}
}
