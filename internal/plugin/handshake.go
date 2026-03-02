package plugin

import (
	goplugin "github.com/hashicorp/go-plugin"
)

// Handshake configs — distinct MagicCookieValue per plugin type prevents
// accidentally loading a provider binary as a strategy plugin.

// ProviderHandshake is the handshake config for provider plugins.
var ProviderHandshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "DOOTSABHA_PROVIDER_PLUGIN",
	MagicCookieValue: "dootsabha-provider-v1",
}

// StrategyHandshake is the handshake config for strategy plugins.
var StrategyHandshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "DOOTSABHA_STRATEGY_PLUGIN",
	MagicCookieValue: "dootsabha-strategy-v1",
}

// HookHandshake is the handshake config for hook plugins.
var HookHandshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "DOOTSABHA_HOOK_PLUGIN",
	MagicCookieValue: "dootsabha-hook-v1",
}

// ProviderPluginMap is the plugin map for provider plugins.
var ProviderPluginMap = map[string]goplugin.Plugin{
	"provider": &ProviderGRPCPlugin{},
}

// StrategyPluginMap is the plugin map for strategy plugins.
var StrategyPluginMap = map[string]goplugin.Plugin{
	"strategy": &StrategyGRPCPlugin{},
}

// HookPluginMap is the plugin map for hook plugins.
var HookPluginMap = map[string]goplugin.Plugin{
	"hook": &HookGRPCPlugin{},
}
