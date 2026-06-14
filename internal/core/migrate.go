package core

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "go.yaml.in/yaml/v3"
)

// retiredProvider is the provider name that no longer exists as a built-in.
// Google is retiring the Gemini CLI on 2026-06-18; the Antigravity CLI (`agy`)
// replaces it.
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

// MigrationResult summarizes the changes a migration made (or would make) to a
// config file.
type MigrationResult struct {
	Path            string // config file inspected (as passed in)
	BackupPath      string // path of the backup written before changes (empty on dry-run / no-op)
	ProviderRenamed bool   // providers.gemini was/would be replaced by providers.agy
	ChairUpdated    bool   // council.chair gemini -> agy
}

// Changed reports whether the migration altered (or would alter) anything.
func (r *MigrationResult) Changed() bool {
	return r.ProviderRenamed || r.ChairUpdated
}

// PlanMigrationFile parses the YAML config file at path and reports what a
// migration would change, WITHOUT writing anything. It reads the raw file (not the
// merged/env-overridden config), so its result matches MigrateConfigFile exactly.
func PlanMigrationFile(path string) (*MigrationResult, error) {
	_, result, err := planMigration(path)
	return result, err
}

// planMigration reads and parses the config file, applies the gemini→agy rewrite to
// an in-memory YAML node tree, and returns the (possibly mutated) document plus a
// result describing what changed. It does not write to disk.
func planMigration(path string) (*yaml.Node, *MigrationResult, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the user's own config
	if err != nil {
		return nil, nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("config %q is not a YAML mapping", path)
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

	return &doc, result, nil
}

// MigrateConfigFile rewrites the YAML config at path, replacing every reference to
// the retired gemini provider with the Antigravity (agy) provider:
//
//   - providers.gemini  → providers.agy (with agy binary/model/flags defaults)
//   - council.chair: gemini → council.chair: agy
//
// gemini-specific binary/model/flags cannot carry over to agy (different binary,
// model namespace, and flags), so the block is replaced with agy defaults; the
// original is preserved in the backup.
//
// A backup is written before any change. If a "<path>.bak" already exists, a
// numbered suffix (.bak.1, .bak.2, …) is used so existing backups are never
// clobbered. Symlinks are resolved so the real target is migrated in place rather
// than the link being replaced by a regular file. Comments and key ordering on
// retained nodes are preserved (yaml.Node round-trip). If the file has no gemini
// references, no backup is written and Changed() reports false.
func MigrateConfigFile(path string) (*MigrationResult, error) {
	doc, result, err := planMigration(path)
	if err != nil {
		return nil, err
	}
	if !result.Changed() {
		return result, nil
	}

	// Resolve symlinks so we migrate the real target in place, not replace the link.
	target := path
	if resolved, rerr := filepath.EvalSymlinks(path); rerr == nil {
		target = resolved
	}

	orig, err := os.ReadFile(target) //nolint:gosec // user's own config
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", target, err)
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("re-encode config: %w", err)
	}

	// Back up the original without clobbering any existing backup.
	backup, err := writeBackup(target, orig)
	if err != nil {
		return nil, err
	}
	result.BackupPath = backup

	// Atomic replace: write temp in the same dir, then rename onto the real target.
	if err := atomicWrite(target, out); err != nil {
		return nil, err
	}

	return result, nil
}

// writeBackup writes data to "<target>.bak", or "<target>.bak.N" if earlier
// backups exist, using exclusive creation so nothing is overwritten.
func writeBackup(target string, data []byte) (string, error) {
	base := target + ".bak"
	for i := 0; ; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s.%d", base, i)
		}
		f, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create backup %q: %w", candidate, err)
		}
		if _, werr := f.Write(data); werr != nil {
			_ = f.Close()
			return "", fmt.Errorf("write backup %q: %w", candidate, werr)
		}
		if cerr := f.Close(); cerr != nil {
			return "", fmt.Errorf("close backup %q: %w", candidate, cerr)
		}
		return candidate, nil
	}
}

// atomicWrite writes data to a temp file in target's directory and renames it onto
// target, preserving 0600 permissions.
func atomicWrite(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace config %q: %w", target, err)
	}
	return nil
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
