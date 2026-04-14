package plugin_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/plugin"
	"github.com/indrasvat/dootsabha/internal/providers"
	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// ── Roundtrip Conversions ──────────────────────────────────────────────────────

func TestProviderResultRoundtrip(t *testing.T) {
	original := &providers.ProviderResult{
		Content:   "Go is a statically typed language.",
		Model:     "claude-opus-4-6",
		Duration:  150 * time.Millisecond,
		CostUSD:   0.001,
		TokensIn:  10,
		TokensOut: 5,
		SessionID: "session-123",
	}

	proto := plugin.ProviderResultToProto(original)
	roundtripped := plugin.ProtoToProviderResult(proto)

	if roundtripped.Content != original.Content {
		t.Errorf("Content = %q, want %q", roundtripped.Content, original.Content)
	}
	if roundtripped.Model != original.Model {
		t.Errorf("Model = %q, want %q", roundtripped.Model, original.Model)
	}
	if roundtripped.Duration != original.Duration {
		t.Errorf("Duration = %v, want %v", roundtripped.Duration, original.Duration)
	}
	if roundtripped.CostUSD != original.CostUSD {
		t.Errorf("CostUSD = %f, want %f", roundtripped.CostUSD, original.CostUSD)
	}
	if roundtripped.TokensIn != original.TokensIn {
		t.Errorf("TokensIn = %d, want %d", roundtripped.TokensIn, original.TokensIn)
	}
	if roundtripped.TokensOut != original.TokensOut {
		t.Errorf("TokensOut = %d, want %d", roundtripped.TokensOut, original.TokensOut)
	}
	if roundtripped.SessionID != original.SessionID {
		t.Errorf("SessionID = %q, want %q", roundtripped.SessionID, original.SessionID)
	}
}

func TestHealthStatusRoundtrip(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		original := &providers.HealthStatus{
			Healthy:    true,
			CLIVersion: "2.1.63",
			Model:      "claude-opus-4-6",
			AuthValid:  true,
			Error:      "",
		}

		proto := plugin.HealthStatusToProto(original)
		roundtripped := plugin.ProtoToHealthStatus(proto)

		if roundtripped.Healthy != original.Healthy {
			t.Errorf("Healthy = %v", roundtripped.Healthy)
		}
		if roundtripped.CLIVersion != original.CLIVersion {
			t.Errorf("CLIVersion = %q", roundtripped.CLIVersion)
		}
		if roundtripped.Model != original.Model {
			t.Errorf("Model = %q", roundtripped.Model)
		}
		if roundtripped.AuthValid != original.AuthValid {
			t.Errorf("AuthValid = %v", roundtripped.AuthValid)
		}
		if roundtripped.Error != original.Error {
			t.Errorf("Error = %q", roundtripped.Error)
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		original := &providers.HealthStatus{
			Healthy: false,
			Error:   "binary not found",
		}

		proto := plugin.HealthStatusToProto(original)
		roundtripped := plugin.ProtoToHealthStatus(proto)

		if roundtripped.Healthy {
			t.Error("expected Healthy=false")
		}
		if roundtripped.Error != "binary not found" {
			t.Errorf("Error = %q", roundtripped.Error)
		}
	})
}

func TestInvokeOptionsRoundtrip(t *testing.T) {
	original := providers.InvokeOptions{
		Model:    "claude-haiku-4-5",
		MaxTurns: 3,
		Timeout:  5 * time.Minute,
	}

	proto := plugin.InvokeOptionsToProto(original)
	roundtripped := plugin.ProtoToInvokeOptions(proto)

	if roundtripped.Model != original.Model {
		t.Errorf("Model = %q, want %q", roundtripped.Model, original.Model)
	}
	if roundtripped.MaxTurns != original.MaxTurns {
		t.Errorf("MaxTurns = %d, want %d", roundtripped.MaxTurns, original.MaxTurns)
	}
	if roundtripped.Timeout != original.Timeout {
		t.Errorf("Timeout = %v, want %v", roundtripped.Timeout, original.Timeout)
	}
}

func TestDispatchResultRoundtrip(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		original := &core.DispatchResult{
			Provider:  "claude",
			Model:     "claude-opus-4-6",
			Content:   "API design proposal",
			Duration:  3 * time.Second,
			CostUSD:   0.005,
			TokensIn:  100,
			TokensOut: 200,
			Error:     nil,
		}

		proto := plugin.DispatchResultToProto(original)
		roundtripped := plugin.ProtoToDispatchResult(proto)

		if roundtripped.Provider != original.Provider {
			t.Errorf("Provider = %q", roundtripped.Provider)
		}
		if roundtripped.Content != original.Content {
			t.Errorf("Content = %q", roundtripped.Content)
		}
		if roundtripped.Duration != original.Duration {
			t.Errorf("Duration = %v", roundtripped.Duration)
		}
		if roundtripped.CostUSD != original.CostUSD {
			t.Errorf("CostUSD = %f", roundtripped.CostUSD)
		}
		if roundtripped.Error != nil {
			t.Errorf("Error = %v, want nil", roundtripped.Error)
		}
	})

	t.Run("failure", func(t *testing.T) {
		original := &core.DispatchResult{
			Provider: "codex",
			Error:    fmt.Errorf("timeout after 30s"),
		}

		proto := plugin.DispatchResultToProto(original)
		roundtripped := plugin.ProtoToDispatchResult(proto)

		if roundtripped.Error == nil {
			t.Fatal("expected error, got nil")
		}
		if roundtripped.Error.Error() != "timeout after 30s" {
			t.Errorf("Error = %q", roundtripped.Error.Error())
		}
	})
}

func TestReviewResultRoundtrip(t *testing.T) {
	original := &core.ReviewResult{
		Reviewer:  "claude",
		Reviewed:  []string{"codex", "gemini"},
		Content:   "Both outputs are well-structured.",
		Duration:  2 * time.Second,
		CostUSD:   0.003,
		TokensIn:  300,
		TokensOut: 100,
		Error:     nil,
	}

	proto := plugin.ReviewResultToProto(original)
	roundtripped := plugin.ProtoToReviewResult(proto)

	if roundtripped.Reviewer != original.Reviewer {
		t.Errorf("Reviewer = %q", roundtripped.Reviewer)
	}
	if len(roundtripped.Reviewed) != len(original.Reviewed) {
		t.Fatalf("Reviewed len = %d, want %d", len(roundtripped.Reviewed), len(original.Reviewed))
	}
	for i, name := range roundtripped.Reviewed {
		if name != original.Reviewed[i] {
			t.Errorf("Reviewed[%d] = %q, want %q", i, name, original.Reviewed[i])
		}
	}
	if roundtripped.Content != original.Content {
		t.Errorf("Content = %q", roundtripped.Content)
	}
}

func TestSynthesisResultRoundtrip(t *testing.T) {
	t.Run("no_fallback", func(t *testing.T) {
		original := &core.SynthesisResult{
			Chair:         "claude",
			ChairFallback: "",
			Content:       "Final synthesis",
			Duration:      3 * time.Second,
			CostUSD:       0.004,
			TokensIn:      500,
			TokensOut:     250,
		}

		proto := plugin.SynthesisResultToProto(original)
		roundtripped := plugin.ProtoToSynthesisResult(proto)

		if roundtripped.Chair != original.Chair {
			t.Errorf("Chair = %q", roundtripped.Chair)
		}
		if roundtripped.ChairFallback != "" {
			t.Errorf("ChairFallback = %q, want empty", roundtripped.ChairFallback)
		}
		if roundtripped.Content != original.Content {
			t.Errorf("Content = %q", roundtripped.Content)
		}
	})

	t.Run("with_fallback", func(t *testing.T) {
		original := &core.SynthesisResult{
			Chair:         "claude",
			ChairFallback: "codex",
			Content:       "Fallback synthesis",
			Duration:      4 * time.Second,
			CostUSD:       0.006,
			TokensIn:      600,
			TokensOut:     300,
		}

		proto := plugin.SynthesisResultToProto(original)
		roundtripped := plugin.ProtoToSynthesisResult(proto)

		if roundtripped.ChairFallback != "codex" {
			t.Errorf("ChairFallback = %q, want codex", roundtripped.ChairFallback)
		}
	})
}

// ── Duration Edge Cases ────────────────────────────────────────────────────────

func TestDurationConversion(t *testing.T) {
	durations := []time.Duration{
		0,
		1 * time.Millisecond,
		999 * time.Millisecond,
		1 * time.Second,
		5 * time.Minute,
		30 * time.Minute,
	}

	for _, d := range durations {
		opts := providers.InvokeOptions{Timeout: d}
		proto := plugin.InvokeOptionsToProto(opts)
		roundtripped := plugin.ProtoToInvokeOptions(proto)

		if roundtripped.Timeout != d {
			t.Errorf("duration %v: roundtripped to %v", d, roundtripped.Timeout)
		}
	}

	// Sub-millisecond durations truncate to 0ms (millisecond precision)
	subMs := providers.InvokeOptions{Timeout: 500 * time.Microsecond}
	proto := plugin.InvokeOptionsToProto(subMs)
	roundtripped := plugin.ProtoToInvokeOptions(proto)
	if roundtripped.Timeout != 0 {
		t.Errorf("sub-ms duration: got %v, want 0 (truncated)", roundtripped.Timeout)
	}
}

// ── Field Coverage via Reflection ──────────────────────────────────────────────

func TestProtoFieldCoverage_ProviderResult(t *testing.T) {
	// Every exported field in providers.ProviderResult must be mapped.
	typ := reflect.TypeFor[providers.ProviderResult]()
	mapped := map[string]bool{
		"Content": true, "Model": true, "Duration": true,
		"CostUSD": true, "TokensIn": true, "TokensOut": true,
		"SessionID": true,
	}

	for field := range typ.Fields() {
		if !field.IsExported() {
			continue
		}
		if !mapped[field.Name] {
			t.Errorf("ProviderResult.%s has no proto mapping", field.Name)
		}
	}
}

func TestProtoFieldCoverage_DispatchResult(t *testing.T) {
	typ := reflect.TypeFor[core.DispatchResult]()
	mapped := map[string]bool{
		"Provider": true, "Model": true, "Content": true,
		"Duration": true, "CostUSD": true, "TokensIn": true,
		"TokensOut": true, "Error": true,
	}

	for field := range typ.Fields() {
		if !field.IsExported() {
			continue
		}
		if !mapped[field.Name] {
			t.Errorf("DispatchResult.%s has no proto mapping", field.Name)
		}
	}
}

func TestProtoFieldCoverage_ReviewResult(t *testing.T) {
	typ := reflect.TypeFor[core.ReviewResult]()
	mapped := map[string]bool{
		"Reviewer": true, "Reviewed": true, "Content": true,
		"Duration": true, "CostUSD": true, "TokensIn": true,
		"TokensOut": true, "Error": true,
	}

	for field := range typ.Fields() {
		if !field.IsExported() {
			continue
		}
		if !mapped[field.Name] {
			t.Errorf("ReviewResult.%s has no proto mapping", field.Name)
		}
	}
}

func TestProtoFieldCoverage_SynthesisResult(t *testing.T) {
	typ := reflect.TypeFor[core.SynthesisResult]()
	mapped := map[string]bool{
		"Chair": true, "ChairFallback": true, "Content": true,
		"Duration": true, "CostUSD": true, "TokensIn": true,
		"TokensOut": true,
	}

	for field := range typ.Fields() {
		if !field.IsExported() {
			continue
		}
		if !mapped[field.Name] {
			t.Errorf("SynthesisResult.%s has no proto mapping", field.Name)
		}
	}
}

// ── Architecture Doc Extension Points ──────────────────────────────────────────

func TestArchitectureDocExtensionFields(t *testing.T) {
	// The proto InvokeRequest must include fields from the architecture doc
	// that are not yet in the current Go types. These are extension points
	// for third-party provider plugins (ollama, deepseek, etc.).
	req := &gen.InvokeRequest{
		Prompt:       "test",
		Temperature:  0.5,
		OutputFormat: "json",
		ExtraArgs:    []string{"--flag"},
		Env:          map[string]string{"KEY": "VAL"},
		WorkDir:      "/tmp",
	}

	if req.GetTemperature() != 0.5 {
		t.Errorf("Temperature field missing or broken")
	}
	if req.GetOutputFormat() != "json" {
		t.Errorf("OutputFormat field missing or broken")
	}
	if len(req.GetExtraArgs()) != 1 {
		t.Errorf("ExtraArgs field missing or broken")
	}
	if len(req.GetEnv()) != 1 {
		t.Errorf("Env field missing or broken")
	}
	if req.GetWorkDir() != "/tmp" {
		t.Errorf("WorkDir field missing or broken")
	}
}

func TestArchitectureDocCapabilitiesFields(t *testing.T) {
	// The proto CapabilitiesResponse must include all fields from the
	// architecture doc's ProviderCaps type.
	caps := &gen.CapabilitiesResponse{
		SupportsJson:      true,
		SupportsStreaming: true,
		SupportedModels:   []string{"model-a", "model-b"},
		DefaultModel:      "model-a",
		MaxContextTokens:  128000,
	}

	if !caps.GetSupportsJson() {
		t.Error("SupportsJson field missing or broken")
	}
	if !caps.GetSupportsStreaming() {
		t.Error("SupportsStreaming field missing or broken")
	}
	if len(caps.GetSupportedModels()) != 2 {
		t.Error("SupportedModels field missing or broken")
	}
	if caps.GetDefaultModel() != "model-a" {
		t.Error("DefaultModel field missing or broken")
	}
	if caps.GetMaxContextTokens() != 128000 {
		t.Error("MaxContextTokens field missing or broken")
	}
}
