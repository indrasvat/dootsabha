package core

import (
	"fmt"
	"os"
	"path/filepath"
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
	"providers.agy.binary":    "CLI executable name (must be on $PATH)",
	"providers.agy.model":     "Default model for agy (Antigravity) invocations",
	"providers.agy.flags":     "Flags passed to every agy invocation",
	"providers.grok.binary":   "CLI executable name (must be on $PATH)",
	"providers.grok.model":    "Default model for grok (xAI) invocations",
	"providers.grok.flags":    "Flags passed to every grok invocation (pinned flags are enforced by the provider)",
	"config_source.type":      "Where the base configuration was loaded from",
	"config_source.path":      "Configuration file path when type is file",
	"council.chair":           "Agent that synthesizes final output (fallback: first healthy non-chair)",
	"council.parallel":        "Run dispatch phase in parallel (false = sequential)",
	"council.rounds":          "Number of deliberation rounds (max 5)",
	"timeout":                 "Budget for one agent call; each call in a pipeline gets its own window (e.g. 30s, 5m, 1h; 0 = use the 5m default)",
	"session_timeout":         "Ceiling for a whole pipeline across every agent call (e.g. 30m, 1h; 0 = unbounded)",
}

// Config holds the resolved दूतसभा configuration.
type Config struct {
	Providers      map[string]ProviderConfig
	Council        CouncilConfig
	Timeout        time.Duration
	SessionTimeout time.Duration
	Source         ConfigSource
	v              *viper.Viper // unexported; used by RedactedView
}

// ConfigSource describes where the base configuration came from.
type ConfigSource struct {
	Type string `json:"type"`
	Path string `json:"path"`
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

	source := ConfigSource{Type: "built-in"}
	if cfgFile == "" {
		defaultPath, err := defaultConfigPath()
		if err != nil {
			return nil, err
		}
		if defaultPath != "" {
			cfgFile = defaultPath
		}
	}

	if cfgFile != "" {
		source = ConfigSource{Type: "file", Path: cfgFile}
		if err := checkRegularFile(cfgFile); err != nil {
			return nil, err
		}
		v.SetConfigFile(cfgFile)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config %q: %w", cfgFile, err)
		}
	}

	// Validate on EVERY path, not only when a file was read. Environment
	// overrides (DOOTSABHA_TIMEOUT, DOOTSABHA_COUNCIL_ROUNDS, …) apply to the
	// built-in defaults too, and viper coerces a bad value to zero just the same.
	if err := validateRawConfig(v); err != nil {
		if cfgFile != "" {
			return nil, fmt.Errorf("invalid config %q: %w", cfgFile, err)
		}
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return buildConfig(v, source), nil
}

// checkRegularFile rejects anything that is not a regular file.
//
// Viper reads the path to EOF, so a character device or FIFO (`--config
// /dev/zero`, or `--config <(gen-config)` fed by an endless producer) never
// returns. A config that cannot be read in bounded time is not a config.
func checkRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("read config %q: is a directory, expected a regular file", path)
	}
	if !info.Mode().IsRegular() {
		// /dev/null is the conventional "ignore my user config" idiom and reads
		// instantly to EOF. The hazard being guarded against is an UNBOUNDED
		// read (/dev/zero, a FIFO with a live writer), not character devices.
		if isDevNull(path) {
			return nil
		}
		return fmt.Errorf("read config %q: not a regular file (mode %s)", path, info.Mode().Type())
	}
	return nil
}

// validateRawConfig rejects values viper would otherwise coerce to a zero value.
//
// Without this, `timeout: 5 minutes` parses as a YAML string, coerces to 0, and
// silently falls back to the built-in default — the run behaves differently from
// what the file says, with no signal. The exit-code contract reserves 6 for a
// config that is "missing, unreadable, or invalid"; these are invalid.
func validateRawConfig(v *viper.Viper) error {
	for _, key := range []string{"timeout", "session_timeout"} {
		raw := v.Get(key)
		if raw == nil {
			continue
		}
		if s, ok := raw.(string); ok {
			if _, err := time.ParseDuration(s); err != nil {
				return fmt.Errorf("%s: %q is not a duration (try 30s, 5m, 1h)", key, s)
			}
			continue
		}
		if _, ok := raw.(int); !ok {
			return fmt.Errorf("%s: expected a duration string, got %T", key, raw)
		}
	}

	if raw := v.Get("council.rounds"); raw != nil {
		switch raw.(type) {
		case int, int32, int64, float64:
		default:
			return fmt.Errorf("council.rounds: expected a number, got %T", raw)
		}
	}

	if raw := v.Get("providers"); raw != nil {
		m, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("providers: expected a mapping, got %T", raw)
		}
		for name, entry := range m {
			if _, ok := entry.(map[string]any); !ok {
				return fmt.Errorf("providers.%s: expected a mapping, got %T", name, entry)
			}
		}
	}

	return nil
}

// defaultConfigPath returns the standard user config file when it exists.
func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}
	path := filepath.Join(home, ".config", "dootsabha", "config.yaml")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat default config %q: %w", path, err)
	}
	return "", nil
}

// setDefaults sets default values for all known configuration keys.
func setDefaults(v *viper.Viper) {
	v.SetDefault("providers.claude.binary", "claude")
	v.SetDefault("providers.claude.model", "claude-opus-4-8")
	v.SetDefault("providers.claude.flags", []string{"--dangerously-skip-permissions", "--no-session-persistence"})
	v.SetDefault("providers.codex.binary", "codex")
	v.SetDefault("providers.codex.model", "gpt-5.5")
	v.SetDefault("providers.codex.flags", []string{"--sandbox", "danger-full-access", "--ephemeral", "--skip-git-repo-check", "-c", "model_reasoning_effort=medium"})
	v.SetDefault("providers.agy.binary", "agy")
	v.SetDefault("providers.agy.model", "Gemini 3.5 Flash (High)")
	v.SetDefault("providers.agy.flags", []string{"--dangerously-skip-permissions"})
	v.SetDefault("providers.grok.binary", "grok")
	v.SetDefault("providers.grok.model", "grok-4.5")
	// Correctness-critical flags (--output-format, --sandbox, --permission-mode, -m,
	// --no-plan) are pinned by the provider and stripped from this list, so only
	// user-tunable settings belong here.
	v.SetDefault("providers.grok.flags", []string{"--reasoning-effort", "high"})
	v.SetDefault("council.chair", "claude")
	v.SetDefault("council.parallel", true)
	v.SetDefault("council.rounds", 1)
	v.SetDefault("timeout", "5m")
	v.SetDefault("session_timeout", "30m")
}

// defaultProviderNames are the built-in AI providers. Membership here makes a
// provider *known* (so `status` lists it and `--agent <name>` resolves); it does
// NOT enrol it in any default pipeline — council/refine/review defaults are set
// independently in the CLI layer.
var defaultProviderNames = []string{"claude", "codex", "agy", "grok"}

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
func buildConfig(v *viper.Viper, source ConfigSource) *Config {
	cfg := &Config{
		Timeout:        v.GetDuration("timeout"),
		SessionTimeout: v.GetDuration("session_timeout"),
		Council: CouncilConfig{
			Chair:    v.GetString("council.chair"),
			Parallel: v.GetBool("council.parallel"),
			Rounds:   v.GetInt("council.rounds"),
		},
		Providers: make(map[string]ProviderConfig),
		Source:    source,
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
	raw["config_source"] = map[string]any{
		"type": c.Source.Type,
		"path": c.Source.Path,
	}
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

// isDevNull reports whether path is the null device.
func isDevNull(path string) bool {
	return path == os.DevNull
}
