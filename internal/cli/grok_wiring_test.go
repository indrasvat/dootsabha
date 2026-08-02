package cli

import (
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/output"
)

// grok must resolve through the single dispatch point every command routes via
// (consult, council, review, refine, status all call getProvider).
func TestGetProviderResolvesGrok(t *testing.T) {
	p, err := getProvider("grok", nil, nil)
	if err != nil {
		t.Fatalf("getProvider(\"grok\") returned error: %v", err)
	}
	if p == nil {
		t.Fatal("getProvider(\"grok\") returned nil provider")
	}
	if p.Name() != "grok" {
		t.Errorf("provider Name() = %q, want grok", p.Name())
	}
}

func TestProviderColorGrokIsDistinct(t *testing.T) {
	grok := providerColor("grok")
	if grok == "" {
		t.Fatal("providerColor(\"grok\") returned empty color")
	}
	if grok != output.GrokColor {
		t.Errorf("providerColor(\"grok\") = %v, want output.GrokColor (%v)", grok, output.GrokColor)
	}
	// Must not collide with the existing provider dots.
	for _, other := range []string{"claude", "codex", "agy"} {
		if providerColor(other) == grok {
			t.Errorf("grok colour collides with %s", other)
		}
	}
}

// The unknown-provider error is user-facing and must list grok as a valid value.
func TestUnknownProviderErrorMentionsGrok(t *testing.T) {
	_, err := getProvider("definitely-not-a-provider", nil, nil)
	if err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
	if !strings.Contains(err.Error(), "grok") {
		t.Errorf("error %q should list grok among valid values", err.Error())
	}
}
