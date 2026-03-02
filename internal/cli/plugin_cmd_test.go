package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/indrasvat/dootsabha/internal/output"
)

func TestInferPluginType(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"claude-provider", "provider"},
		{"codex-provider", "provider"},
		{"council-strategy", "strategy"},
		{"cost-guard-hook", "hook"},
		{"unknown-binary", ""},
		{"provider", ""}, // bare suffix is not a match
		{"strategy", ""}, // bare suffix is not a match
		{"", ""},
	}

	for _, tc := range tests {
		got := inferPluginType(tc.name)
		if got != tc.want {
			t.Errorf("inferPluginType(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDiscoverGRPCPluginsNoDir(t *testing.T) {
	// When the plugins directory doesn't exist, should return nil (no crash).
	entries := discoverGRPCPlugins()
	if len(entries) > 0 {
		t.Errorf("expected no entries when no plugin dir, got %d", len(entries))
	}
}

func TestPluginEntryJSON(t *testing.T) {
	entry := pluginEntry{
		Name:   "claude-provider",
		Type:   "provider",
		Path:   "/usr/local/bin/claude-provider",
		Status: "installed",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]string
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded["name"] != "claude-provider" {
		t.Errorf("name = %q", decoded["name"])
	}
	if decoded["type"] != "provider" {
		t.Errorf("type = %q", decoded["type"])
	}
	if decoded["status"] != "installed" {
		t.Errorf("status = %q", decoded["status"])
	}
}

func TestPluginEntrySliceJSON(t *testing.T) {
	entries := []pluginEntry{
		{Name: "claude-provider", Type: "provider", Path: "/bin/claude-provider", Status: "installed"},
		{Name: "hello", Type: "extension", Path: "/bin/dootsabha-hello", Status: "available"},
	}

	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded []map[string]string
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(decoded))
	}
	if decoded[0]["name"] != "claude-provider" {
		t.Errorf("entry[0].name = %q", decoded[0]["name"])
	}
	if decoded[1]["type"] != "extension" {
		t.Errorf("entry[1].type = %q", decoded[1]["type"])
	}
}

func TestPluginListCmdHelp(t *testing.T) {
	cmd := newPluginCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("plugin --help: %v", err)
	}

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("list")) {
		t.Error("help should mention 'list' subcommand")
	}
	if !bytes.Contains([]byte(out), []byte("inspect")) {
		t.Error("help should mention 'inspect' subcommand")
	}
}

func TestPluginListCmdAliases(t *testing.T) {
	cmd := newPluginCmd()
	aliases := cmd.Aliases
	found := make(map[string]bool)
	for _, a := range aliases {
		found[a] = true
	}
	if !found["vistaarak"] {
		t.Error("missing alias: vistaarak")
	}
	if !found["विस्तारक"] {
		t.Error("missing alias: विस्तारक")
	}
}

func TestPluginListSubcmdAliases(t *testing.T) {
	cmd := newPluginListCmd()
	found := make(map[string]bool)
	for _, a := range cmd.Aliases {
		found[a] = true
	}
	if !found["soochi"] {
		t.Error("missing alias: soochi")
	}
	if !found["सूची"] {
		t.Error("missing alias: सूची")
	}
}

func TestPluginInspectSubcmdAliases(t *testing.T) {
	cmd := newPluginInspectCmd()
	found := make(map[string]bool)
	for _, a := range cmd.Aliases {
		found[a] = true
	}
	if !found["parikshan"] {
		t.Error("missing alias: parikshan")
	}
	if !found["परीक्षण"] {
		t.Error("missing alias: परीक्षण")
	}
}

func TestPluginInspectRequiresArg(t *testing.T) {
	cmd := newPluginInspectCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Error("inspect without args should fail")
	}
}

func TestPluginTypeColor(t *testing.T) {
	// Just verify each type returns a non-empty color (no panics).
	types := []string{"provider", "strategy", "hook", "extension", "unknown"}
	for _, pt := range types {
		c := pluginTypeColor(pt)
		if string(c) == "" {
			t.Errorf("pluginTypeColor(%q) returned empty color", pt)
		}
	}
}

func TestDiscoverGRPCPluginsWithBinaries(t *testing.T) {
	// Create a temp directory structure that mimics plugins/bin/
	// with fake executable binaries.
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugins", "bin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create fake plugin binaries.
	for _, name := range []string{"claude-provider", "codex-provider", "council-strategy"} {
		p := filepath.Join(pluginDir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Also create a non-plugin binary that should be ignored.
	if err := os.WriteFile(filepath.Join(pluginDir, "random-tool"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Scan the directory directly (bypass os.Executable resolution).
	dirEntries, err := os.ReadDir(pluginDir)
	if err != nil {
		t.Fatal(err)
	}

	var entries []pluginEntry
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}
		pt := inferPluginType(de.Name())
		if pt == "" {
			continue
		}
		entries = append(entries, pluginEntry{
			Name:   de.Name(),
			Type:   pt,
			Path:   filepath.Join(pluginDir, de.Name()),
			Status: "installed",
		})
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(entries))
	}

	// Verify types.
	typeMap := make(map[string]string)
	for _, e := range entries {
		typeMap[e.Name] = e.Type
	}
	if typeMap["claude-provider"] != "provider" {
		t.Errorf("claude-provider type = %q", typeMap["claude-provider"])
	}
	if typeMap["codex-provider"] != "provider" {
		t.Errorf("codex-provider type = %q", typeMap["codex-provider"])
	}
	if typeMap["council-strategy"] != "strategy" {
		t.Errorf("council-strategy type = %q", typeMap["council-strategy"])
	}
}

func TestDiscoverGRPCPluginsSkipsNonExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugins", "bin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a non-executable plugin binary.
	p := filepath.Join(pluginDir, "test-provider")
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirEntries, err := os.ReadDir(pluginDir)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}
		if inferPluginType(de.Name()) != "" {
			count++
		}
	}

	if count != 0 {
		t.Errorf("expected 0 executable plugins, got %d", count)
	}
}

func TestPluginListJSONOutput(t *testing.T) {
	// Test that the JSON envelope format is correct by using WriteJSON directly.
	entries := []pluginEntry{
		{Name: "claude-provider", Type: "provider", Path: "/bin/claude-provider", Status: "installed"},
		{Name: "hello", Type: "extension", Path: "/bin/dootsabha-hello", Status: "available"},
	}

	var buf bytes.Buffer
	if err := output.WriteJSON(&buf, entries); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var envelope struct {
		Meta struct {
			SchemaVersion int `json:"schema_version"`
		} `json:"meta"`
		Data []pluginEntry `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if envelope.Meta.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", envelope.Meta.SchemaVersion)
	}
	if len(envelope.Data) != 2 {
		t.Fatalf("data count = %d, want 2", len(envelope.Data))
	}
	if envelope.Data[0].Name != "claude-provider" {
		t.Errorf("data[0].name = %q", envelope.Data[0].Name)
	}
	if envelope.Data[1].Status != "available" {
		t.Errorf("data[1].status = %q", envelope.Data[1].Status)
	}
}

func TestPluginInspectJSONOutput(t *testing.T) {
	entry := pluginEntry{
		Name:   "council-strategy",
		Type:   "strategy",
		Path:   "/plugins/bin/council-strategy",
		Status: "installed",
	}

	var buf bytes.Buffer
	if err := output.WriteJSON(&buf, &entry); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var envelope struct {
		Meta struct {
			SchemaVersion int `json:"schema_version"`
		} `json:"meta"`
		Data pluginEntry `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if envelope.Data.Name != "council-strategy" {
		t.Errorf("name = %q", envelope.Data.Name)
	}
	if envelope.Data.Type != "strategy" {
		t.Errorf("type = %q", envelope.Data.Type)
	}
}
