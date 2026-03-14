package plugin_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/indrasvat/dootsabha/internal/plugin"
	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// setupTestPluginDir creates a temporary directory structure that mimics
// the plugin discovery layout: pluginDir/name/name (binary).
// It symlinks to the pre-built mock-plugin binaries.
func setupTestPluginDir(t *testing.T, plugins ...string) string {
	t.Helper()
	binDir := mockPluginBinDir(t)
	tmpDir := t.TempDir()

	for _, name := range plugins {
		pluginDir := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", pluginDir, err)
		}
		src := filepath.Join(binDir, name)
		dst := filepath.Join(pluginDir, name)
		// Symlink so we don't copy large binaries.
		if err := os.Symlink(src, dst); err != nil {
			t.Fatalf("symlink %s → %s: %v", src, dst, err)
		}
	}
	return tmpDir
}

// ── Discovery Tests ─────────────────────────────────────────────────────────

func TestManagerDiscoverEmpty(t *testing.T) {
	mgr := plugin.NewManager()
	dir := t.TempDir()
	plugins, err := mgr.Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestManagerDiscoverNonexistentDir(t *testing.T) {
	mgr := plugin.NewManager()
	_, err := mgr.Discover("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestManagerDiscoverProvider(t *testing.T) {
	dir := setupTestPluginDir(t, "mock-provider")
	mgr := plugin.NewManager()
	plugins, err := mgr.Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Type != plugin.PluginTypeProvider {
		t.Errorf("type = %s, want provider", plugins[0].Type)
	}
	if plugins[0].Name != "mock-provider" {
		t.Errorf("name = %s, want mock-provider", plugins[0].Name)
	}
}

func TestManagerDiscoverStrategy(t *testing.T) {
	dir := setupTestPluginDir(t, "mock-strategy")
	mgr := plugin.NewManager()
	plugins, err := mgr.Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Type != plugin.PluginTypeStrategy {
		t.Errorf("type = %s, want strategy", plugins[0].Type)
	}
}

func TestManagerDiscoverHook(t *testing.T) {
	dir := setupTestPluginDir(t, "mock-hook")
	mgr := plugin.NewManager()
	plugins, err := mgr.Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Type != plugin.PluginTypeHook {
		t.Errorf("type = %s, want hook", plugins[0].Type)
	}
}

func TestManagerDiscoverMultiple(t *testing.T) {
	dir := setupTestPluginDir(t, "mock-provider", "mock-strategy", "mock-hook")
	mgr := plugin.NewManager()
	plugins, err := mgr.Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(plugins))
	}

	types := make(map[plugin.PluginType]bool)
	for _, p := range plugins {
		types[p.Type] = true
	}
	if !types[plugin.PluginTypeProvider] {
		t.Error("missing provider plugin")
	}
	if !types[plugin.PluginTypeStrategy] {
		t.Error("missing strategy plugin")
	}
	if !types[plugin.PluginTypeHook] {
		t.Error("missing hook plugin")
	}
}

// ── Load + Registry Tests ───────────────────────────────────────────────────

func TestManagerLoadAndGetProvider(t *testing.T) {
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	binDir := mockPluginBinDir(t)
	err := mgr.Load(plugin.PluginInfo{
		Name: "test-provider",
		Type: plugin.PluginTypeProvider,
		Path: filepath.Join(binDir, "mock-provider"),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	provider, err := mgr.GetProvider("test-provider")
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}

	resp, err := provider.Invoke(context.Background(), &gen.InvokeRequest{
		Prompt: "manager test",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp.Content != "Mock response to: manager test" {
		t.Errorf("content = %q", resp.Content)
	}
}

func TestManagerLoadAndGetStrategy(t *testing.T) {
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	binDir := mockPluginBinDir(t)
	err := mgr.Load(plugin.PluginInfo{
		Name: "test-strategy",
		Type: plugin.PluginTypeStrategy,
		Path: filepath.Join(binDir, "mock-strategy"),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	strategy, err := mgr.GetStrategy("test-strategy")
	if err != nil {
		t.Fatalf("get strategy: %v", err)
	}

	resp, err := strategy.Execute(context.Background(), &gen.ExecuteRequest{
		Prompt: "strategy test",
		Agents: []*gen.AgentConfig{{Name: "claude"}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(resp.DispatchResults) != 1 {
		t.Errorf("dispatch count = %d, want 1", len(resp.DispatchResults))
	}
}

func TestManagerLoadAndGetHook(t *testing.T) {
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	binDir := mockPluginBinDir(t)
	err := mgr.Load(plugin.PluginInfo{
		Name: "test-hook",
		Type: plugin.PluginTypeHook,
		Path: filepath.Join(binDir, "mock-hook"),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	hook, err := mgr.GetHook("test-hook")
	if err != nil {
		t.Fatalf("get hook: %v", err)
	}

	resp, err := hook.PreInvoke(context.Background(), &gen.HookRequest{
		EventType: gen.EventType_PRE_INVOKE,
		Payload: &gen.HookRequest_InvokeRequest{
			InvokeRequest: &gen.InvokeRequest{Prompt: "hook test"},
		},
	})
	if err != nil {
		t.Fatalf("pre_invoke: %v", err)
	}
	modified := resp.GetModifiedInvokeRequest()
	if modified == nil {
		t.Fatal("expected modified request")
		return
	}
	if modified.Prompt != "[hook] hook test" {
		t.Errorf("prompt = %q", modified.Prompt)
	}
}

func TestManagerLoadDuplicate(t *testing.T) {
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	binDir := mockPluginBinDir(t)
	info := plugin.PluginInfo{
		Name: "dup",
		Type: plugin.PluginTypeProvider,
		Path: filepath.Join(binDir, "mock-provider"),
	}
	if err := mgr.Load(info); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if err := mgr.Load(info); err == nil {
		t.Fatal("expected error on duplicate load")
	}
}

func TestManagerGetWrongType(t *testing.T) {
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	binDir := mockPluginBinDir(t)
	err := mgr.Load(plugin.PluginInfo{
		Name: "my-provider",
		Type: plugin.PluginTypeProvider,
		Path: filepath.Join(binDir, "mock-provider"),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	_, err = mgr.GetStrategy("my-provider")
	if err == nil {
		t.Fatal("expected error getting provider as strategy")
	}

	_, err = mgr.GetHook("my-provider")
	if err == nil {
		t.Fatal("expected error getting provider as hook")
	}
}

func TestManagerGetNotFound(t *testing.T) {
	mgr := plugin.NewManager()
	_, err := mgr.GetProvider("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestManagerList(t *testing.T) {
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	binDir := mockPluginBinDir(t)
	for _, name := range []string{"mock-provider", "mock-strategy", "mock-hook"} {
		pt := plugin.PluginTypeProvider
		switch name {
		case "mock-strategy":
			pt = plugin.PluginTypeStrategy
		case "mock-hook":
			pt = plugin.PluginTypeHook
		}
		err := mgr.Load(plugin.PluginInfo{
			Name: name,
			Type: pt,
			Path: filepath.Join(binDir, name),
		})
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}

	list := mgr.List()
	if len(list) != 3 {
		t.Errorf("list count = %d, want 3", len(list))
	}
}

func TestManagerLoaded(t *testing.T) {
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	if mgr.Loaded() != 0 {
		t.Errorf("loaded = %d, want 0", mgr.Loaded())
	}

	binDir := mockPluginBinDir(t)
	err := mgr.Load(plugin.PluginInfo{
		Name: "p1",
		Type: plugin.PluginTypeProvider,
		Path: filepath.Join(binDir, "mock-provider"),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mgr.Loaded() != 1 {
		t.Errorf("loaded = %d, want 1", mgr.Loaded())
	}
}

// ── Remove + Shutdown Tests ─────────────────────────────────────────────────

func TestManagerRemove(t *testing.T) {
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	binDir := mockPluginBinDir(t)
	err := mgr.Load(plugin.PluginInfo{
		Name: "removable",
		Type: plugin.PluginTypeProvider,
		Path: filepath.Join(binDir, "mock-provider"),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mgr.Loaded() != 1 {
		t.Fatal("expected 1 loaded")
	}

	if err := mgr.Remove("removable"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if mgr.Loaded() != 0 {
		t.Errorf("loaded = %d, want 0 after remove", mgr.Loaded())
	}

	// Get after remove should fail.
	_, err = mgr.GetProvider("removable")
	if err == nil {
		t.Fatal("expected error after remove")
	}
}

func TestManagerRemoveNotFound(t *testing.T) {
	mgr := plugin.NewManager()
	err := mgr.Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error removing nonexistent plugin")
	}
}

func TestManagerShutdown(t *testing.T) {
	mgr := plugin.NewManager()
	binDir := mockPluginBinDir(t)

	for i, name := range []string{"mock-provider", "mock-strategy"} {
		pt := plugin.PluginTypeProvider
		if name == "mock-strategy" {
			pt = plugin.PluginTypeStrategy
		}
		err := mgr.Load(plugin.PluginInfo{
			Name: name,
			Type: pt,
			Path: filepath.Join(binDir, name),
		})
		if err != nil {
			t.Fatalf("load %d: %v", i, err)
		}
	}

	if mgr.Loaded() != 2 {
		t.Fatalf("loaded = %d, want 2", mgr.Loaded())
	}

	mgr.Shutdown(context.Background())

	if mgr.Loaded() != 0 {
		t.Errorf("loaded = %d after shutdown, want 0", mgr.Loaded())
	}
}

func TestManagerShutdownIdempotent(t *testing.T) {
	mgr := plugin.NewManager()
	binDir := mockPluginBinDir(t)
	err := mgr.Load(plugin.PluginInfo{
		Name: "p1",
		Type: plugin.PluginTypeProvider,
		Path: filepath.Join(binDir, "mock-provider"),
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	mgr.Shutdown(context.Background())
	mgr.Shutdown(context.Background()) // should not panic
}

// ── Discover + Load End-to-End Test ─────────────────────────────────────────

func TestManagerDiscoverAndLoadAll(t *testing.T) {
	dir := setupTestPluginDir(t, "mock-provider", "mock-strategy", "mock-hook")
	mgr := plugin.NewManager()
	defer mgr.Shutdown(context.Background())

	plugins, err := mgr.Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(plugins) != 3 {
		t.Fatalf("discovered %d, want 3", len(plugins))
	}

	for _, p := range plugins {
		if err := mgr.Load(p); err != nil {
			t.Fatalf("load %s: %v", p.Name, err)
		}
	}

	if mgr.Loaded() != 3 {
		t.Errorf("loaded = %d, want 3", mgr.Loaded())
	}

	// Verify each type works.
	provider, err := mgr.GetProvider("mock-provider")
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}
	resp, err := provider.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if !resp.Healthy {
		t.Error("provider not healthy")
	}

	strategy, err := mgr.GetStrategy("mock-strategy")
	if err != nil {
		t.Fatalf("get strategy: %v", err)
	}
	execResp, err := strategy.Execute(context.Background(), &gen.ExecuteRequest{
		Prompt: "e2e test",
		Agents: []*gen.AgentConfig{{Name: "claude"}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(execResp.DispatchResults) != 1 {
		t.Errorf("dispatch count = %d", len(execResp.DispatchResults))
	}

	hook, err := mgr.GetHook("mock-hook")
	if err != nil {
		t.Fatalf("get hook: %v", err)
	}
	hookResp, err := hook.PostSession(context.Background(), &gen.HookRequest{
		EventType: gen.EventType_POST_SESSION,
		Payload: &gen.HookRequest_SessionSummary{
			SessionSummary: &gen.SessionSummary{SessionId: "e2e"},
		},
	})
	if err != nil {
		t.Fatalf("post_session: %v", err)
	}
	if !hookResp.Proceed {
		t.Error("expected proceed = true")
	}
}
