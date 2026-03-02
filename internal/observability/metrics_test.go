package observability

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMetricsRecordSingle(t *testing.T) {
	m := NewMetrics()
	m.RecordInvocation("claude", 100*time.Millisecond, 0.01, 50, 100, nil)

	stats := m.ProviderStats("claude")
	if stats.Invocations != 1 {
		t.Errorf("invocations = %d, want 1", stats.Invocations)
	}
	if stats.CostUSD != 0.01 {
		t.Errorf("cost = %f, want 0.01", stats.CostUSD)
	}
	if stats.TokensIn != 50 {
		t.Errorf("tokens in = %d, want 50", stats.TokensIn)
	}
	if stats.TokensOut != 100 {
		t.Errorf("tokens out = %d, want 100", stats.TokensOut)
	}
	if stats.Errors != 0 {
		t.Errorf("errors = %d, want 0", stats.Errors)
	}
}

func TestMetricsRecordMultiple(t *testing.T) {
	m := NewMetrics()
	m.RecordInvocation("claude", 100*time.Millisecond, 0.01, 50, 100, nil)
	m.RecordInvocation("claude", 200*time.Millisecond, 0.02, 60, 120, nil)
	m.RecordInvocation("codex", 150*time.Millisecond, 0.015, 40, 80, nil)

	claude := m.ProviderStats("claude")
	if claude.Invocations != 2 {
		t.Errorf("claude invocations = %d, want 2", claude.Invocations)
	}
	if claude.CostUSD != 0.03 {
		t.Errorf("claude cost = %f, want 0.03", claude.CostUSD)
	}
	if claude.TokensIn != 110 {
		t.Errorf("claude tokens in = %d, want 110", claude.TokensIn)
	}

	codex := m.ProviderStats("codex")
	if codex.Invocations != 1 {
		t.Errorf("codex invocations = %d, want 1", codex.Invocations)
	}
}

func TestMetricsRecordWithError(t *testing.T) {
	m := NewMetrics()
	m.RecordInvocation("claude", 100*time.Millisecond, 0, 0, 0, fmt.Errorf("timeout"))
	m.RecordInvocation("claude", 200*time.Millisecond, 0.01, 50, 100, nil)

	stats := m.ProviderStats("claude")
	if stats.Errors != 1 {
		t.Errorf("errors = %d, want 1", stats.Errors)
	}
	if stats.Invocations != 2 {
		t.Errorf("invocations = %d, want 2", stats.Invocations)
	}
}

func TestMetricsSummary(t *testing.T) {
	m := NewMetrics()
	m.RecordInvocation("claude", 100*time.Millisecond, 0.01, 50, 100, nil)
	m.RecordInvocation("codex", 200*time.Millisecond, 0.02, 60, 120, nil)
	m.RecordInvocation("gemini", 150*time.Millisecond, 0.015, 40, 80, nil)

	summary := m.Summary()

	if len(summary.Providers) != 3 {
		t.Fatalf("providers count = %d, want 3", len(summary.Providers))
	}
	if summary.TotalCostUSD != 0.045 {
		t.Errorf("total cost = %f, want 0.045", summary.TotalCostUSD)
	}
	if summary.TotalTokensIn != 150 {
		t.Errorf("total tokens in = %d, want 150", summary.TotalTokensIn)
	}
	if summary.TotalTokensOut != 300 {
		t.Errorf("total tokens out = %d, want 300", summary.TotalTokensOut)
	}
	if summary.TotalDuration <= 0 {
		t.Error("total duration should be positive")
	}
}

func TestMetricsProviderStatsNotFound(t *testing.T) {
	m := NewMetrics()
	stats := m.ProviderStats("nonexistent")

	if stats.Invocations != 0 {
		t.Errorf("invocations = %d, want 0", stats.Invocations)
	}
}

func TestMetricsConcurrentRecording(t *testing.T) {
	m := NewMetrics()
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			provider := fmt.Sprintf("provider-%d", n%3)
			m.RecordInvocation(provider, time.Millisecond, 0.001, 10, 20, nil)
		}(i)
	}

	wg.Wait()

	summary := m.Summary()
	totalInvocations := 0
	for _, pm := range summary.Providers {
		totalInvocations += pm.Invocations
	}

	if totalInvocations != 100 {
		t.Errorf("total invocations = %d, want 100", totalInvocations)
	}
}

func TestMetricsSummaryDuration(t *testing.T) {
	m := NewMetrics()
	time.Sleep(5 * time.Millisecond)
	summary := m.Summary()

	if summary.TotalDuration < 5*time.Millisecond {
		t.Errorf("total duration = %v, want >= 5ms", summary.TotalDuration)
	}
}

func TestMetricsEmptySummary(t *testing.T) {
	m := NewMetrics()
	summary := m.Summary()

	if len(summary.Providers) != 0 {
		t.Errorf("providers count = %d, want 0", len(summary.Providers))
	}
	if summary.TotalCostUSD != 0 {
		t.Errorf("total cost = %f, want 0", summary.TotalCostUSD)
	}
}

func TestMetricsDurationAggregation(t *testing.T) {
	m := NewMetrics()
	m.RecordInvocation("claude", 100*time.Millisecond, 0, 0, 0, nil)
	m.RecordInvocation("claude", 200*time.Millisecond, 0, 0, 0, nil)

	stats := m.ProviderStats("claude")
	if stats.Duration != 300*time.Millisecond {
		t.Errorf("duration = %v, want 300ms", stats.Duration)
	}
}
