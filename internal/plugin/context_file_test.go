package plugin

import (
	"encoding/json"
	"os"
	"testing"
)

func TestWriteContextFileCreatesValidJSON(t *testing.T) {
	ctx := ContextFile{
		Version:   "0.1.0",
		SessionID: "ds_test1",
		Workspace: "/tmp/test",
		Providers: map[string]ContextProvider{
			"claude": {Healthy: true, Model: "sonnet-4-6"},
		},
		Capabilities: ContextCapabilities{
			Council: true,
			Review:  true,
			Refine:  true,
			Plugins: true,
		},
		TTY:       true,
		TermWidth: 120,
	}

	path, err := WriteContextFile(ctx)
	if err != nil {
		t.Fatalf("WriteContextFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var decoded ContextFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal: %v\ncontent: %s", err, string(data))
	}

	if decoded.Version != "0.1.0" {
		t.Errorf("version = %q", decoded.Version)
	}
	if decoded.SessionID != "ds_test1" {
		t.Errorf("session_id = %q", decoded.SessionID)
	}
	if decoded.Workspace != "/tmp/test" {
		t.Errorf("workspace = %q", decoded.Workspace)
	}
}

func TestWriteContextFileContainsAllFields(t *testing.T) {
	ctx := ContextFile{
		Version:   "0.1.0",
		SessionID: "ds_abcde",
		Workspace: "/home/user/project",
		Providers: map[string]ContextProvider{
			"claude": {Healthy: true, Model: "sonnet-4-6"},
			"codex":  {Healthy: false, Model: "o4-mini"},
		},
		Capabilities: ContextCapabilities{
			Council: true,
			Review:  true,
			Refine:  false,
			Plugins: true,
		},
		TTY:       false,
		TermWidth: 80,
	}

	path, err := WriteContextFile(ctx)
	if err != nil {
		t.Fatalf("WriteContextFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	requiredKeys := []string{"version", "session_id", "workspace", "providers", "capabilities", "tty", "terminal_width"}
	for _, key := range requiredKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing required key: %q", key)
		}
	}
}

func TestWriteContextFileCleanup(t *testing.T) {
	ctx := ContextFile{
		Version:   "0.1.0",
		SessionID: "ds_clean",
	}

	path, err := WriteContextFile(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// File should exist.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}

	// Clean up.
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// File should no longer exist.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be removed after cleanup")
	}
}

func TestWriteContextFileProviders(t *testing.T) {
	ctx := ContextFile{
		Providers: map[string]ContextProvider{
			"claude": {Healthy: true, Model: "sonnet-4-6"},
			"codex":  {Healthy: false, Model: "o4-mini"},
			"gemini": {Healthy: true, Model: "gemini-2.5-flash"},
		},
	}

	path, err := WriteContextFile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ContextFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Providers) != 3 {
		t.Fatalf("providers count = %d, want 3", len(decoded.Providers))
	}
	if !decoded.Providers["claude"].Healthy {
		t.Error("claude should be healthy")
	}
	if decoded.Providers["codex"].Healthy {
		t.Error("codex should be unhealthy")
	}
	if decoded.Providers["codex"].Model != "o4-mini" {
		t.Errorf("codex model = %q", decoded.Providers["codex"].Model)
	}
}

func TestWriteContextFileCapabilities(t *testing.T) {
	ctx := ContextFile{
		Capabilities: ContextCapabilities{
			Council: true,
			Review:  true,
			Refine:  false,
			Plugins: true,
		},
	}

	path, err := WriteContextFile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ContextFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if !decoded.Capabilities.Council {
		t.Error("council should be true")
	}
	if decoded.Capabilities.Refine {
		t.Error("refine should be false")
	}
}

func TestDefaultContextFile(t *testing.T) {
	ctx := DefaultContextFile("ds_test2", true, 120)

	if ctx.SessionID != "ds_test2" {
		t.Errorf("session_id = %q", ctx.SessionID)
	}
	if !ctx.TTY {
		t.Error("tty should be true")
	}
	if ctx.TermWidth != 120 {
		t.Errorf("terminal_width = %d", ctx.TermWidth)
	}
	if len(ctx.Providers) != 3 {
		t.Errorf("providers count = %d, want 3", len(ctx.Providers))
	}
	if !ctx.Capabilities.Council {
		t.Error("council capability should be true")
	}
	if ctx.Workspace == "" {
		t.Error("workspace should not be empty")
	}
}

func TestWriteContextFileEmptyProviders(t *testing.T) {
	ctx := ContextFile{
		Version:   "0.1.0",
		Providers: map[string]ContextProvider{},
	}

	path, err := WriteContextFile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ContextFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Providers) != 0 {
		t.Errorf("providers count = %d, want 0", len(decoded.Providers))
	}
}
