package main

import (
	"context"
	"slices"
	"testing"

	"github.com/indrasvat/dootsabha/internal/providers"
)

// Capabilities is what a host reads before dispatching, and nothing else asserts
// it. It now reads providers.AgyDefaultModel, so this pins the SHAPE — that the
// default is offered, that superseded-but-live models survive a bump, and that
// SupportsJson tracks the provider actually parsing JSON.
func TestAgyPluginCapabilities(t *testing.T) {
	caps, err := newAgyPluginServer().Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}

	if caps.DefaultModel != providers.AgyDefaultModel {
		t.Errorf("DefaultModel = %q, want %q", caps.DefaultModel, providers.AgyDefaultModel)
	}
	if !slices.Contains(caps.SupportedModels, caps.DefaultModel) {
		t.Errorf("SupportedModels %v omits the default %q", caps.SupportedModels, caps.DefaultModel)
	}
	// A bump is not a retirement: `agy models` still lists these.
	for _, m := range []string{
		"Gemini 3.6 Flash (High)", "Gemini 3.5 Flash (High)", "Gemini 3.1 Pro (High)",
	} {
		if !slices.Contains(caps.SupportedModels, m) {
			t.Errorf("SupportedModels dropped %q, which is still live", m)
		}
	}
	if !caps.SupportsJson {
		t.Error("SupportsJson = false, but the provider parses --output-format json")
	}
	if caps.MaxContextTokens != 1_000_000 {
		t.Errorf("MaxContextTokens = %d, want 1000000", caps.MaxContextTokens)
	}
}
