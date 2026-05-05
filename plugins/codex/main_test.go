package main

import (
	"context"
	"slices"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

func TestCodexPluginCapabilitiesUseDefaultModel(t *testing.T) {
	resp, err := newCodexPluginServer().Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if resp.DefaultModel != core.DefaultCodexModel {
		t.Fatalf("DefaultModel = %q, want %q", resp.DefaultModel, core.DefaultCodexModel)
	}
	if slices.Contains(resp.SupportedModels, core.DefaultCodexModel) {
		return
	}
	t.Fatalf("SupportedModels = %v, want %q", resp.SupportedModels, core.DefaultCodexModel)
}
