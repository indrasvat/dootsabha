package plugin

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/indrasvat/dootsabha/internal/version"
)

// ContextFile represents the Tier 2 context passed to extensions via a JSON file.
type ContextFile struct {
	Version      string                     `json:"version"`
	SessionID    string                     `json:"session_id"`
	Workspace    string                     `json:"workspace"`
	Providers    map[string]ContextProvider `json:"providers"`
	Capabilities ContextCapabilities        `json:"capabilities"`
	TTY          bool                       `json:"tty"`
	TermWidth    int                        `json:"terminal_width"`
}

// ContextProvider describes a provider's status in the context file.
type ContextProvider struct {
	Healthy bool   `json:"healthy"`
	Model   string `json:"model"`
}

// ContextCapabilities describes what the host supports.
type ContextCapabilities struct {
	Council bool `json:"council"`
	Review  bool `json:"review"`
	Refine  bool `json:"refine"`
	Plugins bool `json:"plugins"`
}

// WriteContextFile creates a temporary JSON context file and returns its path.
// The caller is responsible for removing the file when done (defer os.Remove(path)).
func WriteContextFile(ctx ContextFile) (string, error) {
	if ctx.Version == "" {
		ctx.Version = version.Version
	}

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal context file: %w", err)
	}

	f, err := os.CreateTemp("", "dootsabha-context-*.json")
	if err != nil {
		return "", fmt.Errorf("create context file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write context file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("close context file: %w", err)
	}

	return f.Name(), nil
}

// DefaultContextFile returns a ContextFile with sensible defaults for the current session.
func DefaultContextFile(sessionID string, isTTY bool, termWidth int) ContextFile {
	wd, _ := os.Getwd()

	return ContextFile{
		Version:   version.Version,
		SessionID: sessionID,
		Workspace: wd,
		Providers: map[string]ContextProvider{
			"claude": {Healthy: true, Model: "opus-4-6"},
			"codex":  {Healthy: true, Model: "o4-mini"},
			"gemini": {Healthy: true, Model: "gemini-2.5-flash"},
		},
		Capabilities: ContextCapabilities{
			Council: true,
			Review:  true,
			Refine:  true,
			Plugins: true,
		},
		TTY:       isTTY,
		TermWidth: termWidth,
	}
}
