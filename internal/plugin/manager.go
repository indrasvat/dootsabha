package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// PluginType identifies what kind of plugin a binary implements.
type PluginType string

const (
	PluginTypeProvider PluginType = "provider"
	PluginTypeStrategy PluginType = "strategy"
	PluginTypeHook     PluginType = "hook"
)

// PluginInfo describes a discovered plugin binary.
type PluginInfo struct {
	// Name is the plugin's logical name (directory name).
	Name string
	// Type is the plugin kind (provider, strategy, hook).
	Type PluginType
	// Path is the absolute path to the binary.
	Path string
}

// loadedPlugin tracks a running plugin process.
type loadedPlugin struct {
	Info   PluginInfo
	Client *goplugin.Client
	Raw    any // ProviderPlugin, StrategyPlugin, or HookPlugin
}

// Manager discovers, loads, and manages gRPC plugin processes.
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]*loadedPlugin // keyed by name
	logger  hclog.Logger
}

// NewManager creates a plugin manager.
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]*loadedPlugin),
		logger: hclog.New(&hclog.LoggerOptions{
			Name:   "plugin-manager",
			Output: os.Stderr,
			Level:  hclog.Error,
		}),
	}
}

// Discover scans a directory for plugin binaries. Each subdirectory is treated
// as a plugin, with the binary named after the directory. The plugin type is
// determined by probing handshake configs.
func (m *Manager) Discover(dir string) ([]PluginInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading plugin directory %s: %w", dir, err)
	}

	var plugins []PluginInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		binPath := filepath.Join(dir, name, name)

		info, err := os.Stat(binPath)
		if err != nil {
			continue // no binary found — skip
		}
		if info.IsDir() {
			continue
		}
		// Check if executable.
		if info.Mode()&0o111 == 0 {
			continue
		}

		pluginType, err := m.probePluginType(binPath)
		if err != nil {
			continue // can't determine type — skip
		}

		plugins = append(plugins, PluginInfo{
			Name: name,
			Type: pluginType,
			Path: binPath,
		})
	}

	return plugins, nil
}

// probePluginType attempts to connect to a binary with each handshake config
// to determine its type.
func (m *Manager) probePluginType(binPath string) (PluginType, error) {
	configs := []struct {
		Type      PluginType
		Handshake goplugin.HandshakeConfig
		Plugins   map[string]goplugin.Plugin
		Key       string
	}{
		{PluginTypeProvider, ProviderHandshake, ProviderPluginMap, "provider"},
		{PluginTypeStrategy, StrategyHandshake, StrategyPluginMap, "strategy"},
		{PluginTypeHook, HookHandshake, HookPluginMap, "hook"},
	}

	for _, cfg := range configs {
		client := goplugin.NewClient(&goplugin.ClientConfig{
			HandshakeConfig:  cfg.Handshake,
			Plugins:          cfg.Plugins,
			Cmd:              exec.Command(binPath),
			AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
			Logger:           m.logger,
		})

		rpcClient, err := client.Client()
		if err != nil {
			client.Kill()
			continue
		}

		_, err = rpcClient.Dispense(cfg.Key)
		client.Kill()
		if err != nil {
			continue
		}
		return cfg.Type, nil
	}

	return "", fmt.Errorf("could not determine plugin type for %s", binPath)
}

// Load starts a plugin process and registers it.
func (m *Manager) Load(info PluginInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[info.Name]; exists {
		return fmt.Errorf("plugin %q already loaded", info.Name)
	}

	handshake, pluginMap, key, err := configForType(info.Type)
	if err != nil {
		return err
	}

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          pluginMap,
		Cmd:              exec.Command(info.Path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           m.logger,
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("connecting to plugin %q: %w", info.Name, err)
	}

	raw, err := rpcClient.Dispense(key)
	if err != nil {
		client.Kill()
		return fmt.Errorf("dispensing plugin %q: %w", info.Name, err)
	}

	m.plugins[info.Name] = &loadedPlugin{
		Info:   info,
		Client: client,
		Raw:    raw,
	}

	return nil
}

// configForType returns the handshake, plugin map, and dispense key for a type.
func configForType(pt PluginType) (goplugin.HandshakeConfig, map[string]goplugin.Plugin, string, error) {
	switch pt {
	case PluginTypeProvider:
		return ProviderHandshake, ProviderPluginMap, "provider", nil
	case PluginTypeStrategy:
		return StrategyHandshake, StrategyPluginMap, "strategy", nil
	case PluginTypeHook:
		return HookHandshake, HookPluginMap, "hook", nil
	default:
		return goplugin.HandshakeConfig{}, nil, "", fmt.Errorf("unknown plugin type: %s", pt)
	}
}

// GetProvider returns a loaded provider plugin by name.
func (m *Manager) GetProvider(name string) (ProviderPlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lp, ok := m.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	if lp.Info.Type != PluginTypeProvider {
		return nil, fmt.Errorf("plugin %q is %s, not provider", name, lp.Info.Type)
	}
	p, ok := lp.Raw.(ProviderPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q does not implement ProviderPlugin", name)
	}
	return p, nil
}

// GetStrategy returns a loaded strategy plugin by name.
func (m *Manager) GetStrategy(name string) (StrategyPlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lp, ok := m.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	if lp.Info.Type != PluginTypeStrategy {
		return nil, fmt.Errorf("plugin %q is %s, not strategy", name, lp.Info.Type)
	}
	s, ok := lp.Raw.(StrategyPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q does not implement StrategyPlugin", name)
	}
	return s, nil
}

// GetHook returns a loaded hook plugin by name.
func (m *Manager) GetHook(name string) (HookPlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lp, ok := m.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	if lp.Info.Type != PluginTypeHook {
		return nil, fmt.Errorf("plugin %q is %s, not hook", name, lp.Info.Type)
	}
	h, ok := lp.Raw.(HookPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q does not implement HookPlugin", name)
	}
	return h, nil
}

// List returns info about all loaded plugins.
func (m *Manager) List() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(m.plugins))
	for _, lp := range m.plugins {
		infos = append(infos, lp.Info)
	}
	return infos
}

// Remove stops and removes a plugin by name.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lp, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	lp.Client.Kill()
	delete(m.plugins, name)
	return nil
}

// Shutdown stops all loaded plugins. Safe to call multiple times.
func (m *Manager) Shutdown(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, lp := range m.plugins {
		lp.Client.Kill()
		delete(m.plugins, name)
	}
}

// Loaded returns the count of currently loaded plugins.
func (m *Manager) Loaded() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}
