package main

import (
	"context"
	"slices"
	"testing"
)

// The plugin advertises the grok defaults to every gRPC consumer. It no longer
// holds its own copy of the model id — DefaultModel and SupportedModels[0] both
// read providers.GrokDefaultModel — so this pins the VALUE that constant must
// carry, and the shape of what the plugin promises consumers.
func TestGrokPluginCapabilities(t *testing.T) {
	caps, err := newGrokPluginServer().Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}

	if got := caps.GetDefaultModel(); got != "grok-4.6" {
		t.Errorf("DefaultModel = %q, want grok-4.6", got)
	}

	// grok-4.5 was NOT retired — `grok models` still lists it. Dropping it here
	// would tell consumers a live model is unsupported.
	want := []string{"grok-4.6", "grok-4.5"}
	if got := caps.GetSupportedModels(); !slices.Equal(got, want) {
		t.Errorf("SupportedModels = %v, want %v (newest first, 4.5 still selectable)", got, want)
	}

	// Invariant, not a literal: whatever the default is, it must be offered.
	if !slices.Contains(caps.GetSupportedModels(), caps.GetDefaultModel()) {
		t.Errorf("DefaultModel %q is not in SupportedModels %v",
			caps.GetDefaultModel(), caps.GetSupportedModels())
	}

	// docs.x.ai lists 500k for both 4.5 and 4.6 — the bump must not "helpfully"
	// change this.
	if got := caps.GetMaxContextTokens(); got != 500000 {
		t.Errorf("MaxContextTokens = %d, want 500000", got)
	}
}
