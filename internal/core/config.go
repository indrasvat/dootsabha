package core

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ConfigComments maps config keys to human-readable descriptions.
// Used by `config show --commented` to provide inline documentation.
var ConfigComments = map[string]string{
	"providers.claude.binary": "CLI executable name (must be on $PATH)",
	"providers.claude.model":  "Default model for claude invocations",
	"providers.claude.flags":  "Flags passed to every claude invocation",
	"providers.codex.binary":  "CLI executable name (must be on $PATH)",
	"providers.codex.model":   "Default model for codex invocations",
	"providers.codex.flags":   "Flags passed to every codex invocation",
	"providers.gemini.binary": "CLI executable name (must be on $PATH)",
	"providers.gemini.model":  "Default model for gemini invocations",
	"providers.gemini.flags":  "Flags passed to every gemini invocation",
	"council.chair":           "Agent that synthesizes final output (fallback: first healthy non-chair)",
	"council.parallel":        "Run dispatch phase in parallel (false = sequential)",
	"council.rounds":          "Number of deliberation rounds (max 5)",
	"timeout":                 "Global invocation timeout (e.g. 30s, 5m, 1h; 0 = disabled)",
	"session_timeout":         "Max total duration for multi-agent pipelines (e.g. 30m, 1h; 0 = disabled)",
}

// Config holds the resolved दूतसभा configuration.
type Config struct {
	Providers      map[string]ProviderConfig
	Council        CouncilConfig
	Timeout        time.Duration
	SessionTimeout time.Duration
	v              *viper.Viper // unexported; used by RedactedView
}

// ProviderConfig holds per-provider settings.
type ProviderConfig struct {
	Binary string
	Model  string
	Flags  []string
}

// CouncilConfig holds council deliberation settings.
type CouncilConfig struct {
	Chair    string
	Parallel bool
	Rounds   int
}

// LoadConfig loads configuration from file, env vars, and defaults.
// Merge order: defaults → YAML file → env vars (DOOTSABHA_*) → CLI flags.
// Unknown keys in the YAML file are silently ignored (forward-compatible).
func LoadConfig(cfgFile string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("DOOTSABHA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config %q: %w", cfgFile, err)
		}
	}

	return buildConfig(v), nil
}

// setDefaults sets default values for all known configuration keys.
func setDefaults(v *viper.Viper) {
	v.SetDefault("providers.claude.binary", "claude")
	v.SetDefault("providers.claude.model", "claude-sonnet-4-6")
	v.SetDefault("providers.claude.flags", []string{"--dangerously-skip-permissions", "--no-session-persistence"})
	v.SetDefault("providers.codex.binary", "codex")
	v.SetDefault("providers.codex.model", "gpt-5.4")
	v.SetDefault("providers.codex.flags", []string{"--sandbox", "danger-full-access", "--ephemeral", "--skip-git-repo-check", "-c", "model_reasoning_effort=medium"})
	v.SetDefault("providers.gemini.binary", "gemini")
	v.SetDefault("providers.gemini.model", "gemini-3-pro")
	v.SetDefault("providers.gemini.flags", []string{"--approval-mode", "yolo"})
	v.SetDefault("council.chair", "claude")
	v.SetDefault("council.parallel", true)
	v.SetDefault("council.rounds", 1)
	v.SetDefault("timeout", "5m")
	v.SetDefault("session_timeout", "30m")
}

// defaultProviderNames are the three built-in AI providers.
var defaultProviderNames = []string{"claude", "codex", "gemini"}

// collectProviderNames returns all provider names: built-ins plus any from config file.
func collectProviderNames(v *viper.Viper) []string {
	seen := make(map[string]bool)
	var names []string
	for _, name := range defaultProviderNames {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if raw := v.Get("providers"); raw != nil {
		if m, ok := raw.(map[string]any); ok {
			for name := range m {
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}
	sort.Strings(names)
	return names
}

// buildConfig constructs a Config from the Viper instance.
func buildConfig(v *viper.Viper) *Config {
	cfg := &Config{
		Timeout:        v.GetDuration("timeout"),
		SessionTimeout: v.GetDuration("session_timeout"),
		Council: CouncilConfig{
			Chair:    v.GetString("council.chair"),
			Parallel: v.GetBool("council.parallel"),
			Rounds:   v.GetInt("council.rounds"),
		},
		Providers: make(map[string]ProviderConfig),
		v:         v,
	}

	for _, name := range collectProviderNames(v) {
		pfx := "providers." + name + "."
		cfg.Providers[name] = ProviderConfig{
			Binary: v.GetString(pfx + "binary"),
			Model:  v.GetString(pfx + "model"),
			Flags:  v.GetStringSlice(pfx + "flags"),
		}
	}

	return cfg
}

// RedactedView returns the full resolved configuration as a map.
// Sensitive keys (matching *token*, *key*, *secret*, *password*) are replaced
// with "[REDACTED]" unless reveal is true.
func (c *Config) RedactedView(reveal bool) map[string]any {
	raw := c.v.AllSettings()
	if reveal {
		return raw
	}
	return redact(raw)
}

// sensitiveKey reports whether a config key name suggests a sensitive value.
func sensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "key") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password")
}

// redact recursively replaces values at sensitive keys with "[REDACTED]".
func redact(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if sensitiveKey(k) {
			out[k] = "[REDACTED]"
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			out[k] = redact(nested)
			continue
		}
		out[k] = v
	}
	return out
}
