package core

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "go.yaml.in/yaml/v3"
)

// retiredProvider is the provider name that no longer exists as a built-in.
// Google sunset the Gemini CLI on 2026-06-18; the Antigravity CLI (`agy`) replaces it.
const (
	retiredProvider     = "gemini"
	replacementProvider = "agy"
)

// agy provider defaults applied during migration.
const (
	agyBinary = "agy"
	agyModel  = "Gemini 3.5 Flash (High)"
)

var agyFlags = []string{"--dangerously-skip-permissions"}

// NeedsMigration reports whether the loaded config still references the retired
// gemini provider — either as a provider entry or as the council chair.
func NeedsMigration(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	if _, ok := cfg.Providers[retiredProvider]; ok {
		return true
	}
	return cfg.Council.Chair == retiredProvider
}

// UserConfigPath returns the standard user config file path regardless of whether
// it exists. Unlike defaultConfigPath, it does not stat the file.
func UserConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "dootsabha", "config.yaml"), nil
}

// MigrationResult summarizes the changes a migration made to a config file.
type MigrationResult struct {
	Path            string // config file that was migrated
	BackupPath      string // path of the .bak copy written before changes
	ProviderRenamed bool   // providers.gemini was replaced by providers.agy
	ChairUpdated    bool   // council.chair gemini -> agy
}

// Changed reports whether the migration altered anything.
func (r *MigrationResult) Changed() bool {
	return r.ProviderRenamed || r.ChairUpdated
}

// MigrateConfigFile rewrites the YAML config at path, replacing every reference to
// the retired gemini provider with the Antigravity (agy) provider:
//
//   - providers.gemini  → providers.agy (with agy binary/model/flags defaults)
//   - council.chair: gemini → council.chair: agy
//
// A timestamp-free "<path>.bak" backup is written before any change. Comments and
// key ordering on retained nodes are preserved (yaml.Node round-trip). If the file
// has no gemini references, no backup is written and Changed() reports false.
func MigrateConfigFile(path string) (*MigrationResult, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the user's own config
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config %q is not a YAML mapping", path)
	}
	root := doc.Content[0]

	result := &MigrationResult{Path: path}

	if providers := mappingValue(root, "providers"); providers != nil {
		if renameProviderNode(providers) {
			result.ProviderRenamed = true
		}
	}
	if council := mappingValue(root, "council"); council != nil {
		if chair := mappingValue(council, "chair"); chair != nil && chair.Value == retiredProvider {
			chair.SetString(replacementProvider)
			result.ChairUpdated = true
		}
	}

	if !result.Changed() {
		return result, nil
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("re-encode config: %w", err)
	}

	// Back up the original before overwriting.
	backup := path + ".bak"
	if err := os.WriteFile(backup, data, 0o600); err != nil {
		return nil, fmt.Errorf("write backup %q: %w", backup, err)
	}
	result.BackupPath = backup

	// Atomic replace: write temp in the same dir, then rename.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("chmod temp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("replace config %q: %w", path, err)
	}

	return result, nil
}

// renameProviderNode renames a `gemini:` key under a providers mapping to `agy:` and
// replaces its value with agy defaults. Returns true if a rename happened.
func renameProviderNode(providers *yaml.Node) bool {
	for i := 0; i+1 < len(providers.Content); i += 2 {
		keyNode := providers.Content[i]
		if keyNode.Value != retiredProvider {
			continue
		}
		// If an agy entry already exists, drop the gemini pair entirely.
		if mappingValue(providers, replacementProvider) != nil {
			providers.Content = append(providers.Content[:i], providers.Content[i+2:]...)
			return true
		}
		keyNode.SetString(replacementProvider)
		providers.Content[i+1] = newAgyProviderNode()
		return true
	}
	return false
}

// newAgyProviderNode builds the YAML mapping node for the agy provider defaults.
func newAgyProviderNode() *yaml.Node {
	var node yaml.Node
	// Encode never fails for plain map/string/slice values.
	_ = node.Encode(map[string]any{
		"binary": agyBinary,
		"model":  agyModel,
		"flags":  agyFlags,
	})
	return &node
}

// mappingValue returns the value node for key in a mapping node, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
