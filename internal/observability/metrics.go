package observability

import (
	"sync"
	"time"
)

// ProviderMetrics tracks per-provider invocation statistics.
type ProviderMetrics struct {
	Invocations int           `json:"invocations"`
	Duration    time.Duration `json:"duration_ms"`
	CostUSD     float64       `json:"cost_usd"`
	TokensIn    int           `json:"tokens_in"`
	TokensOut   int           `json:"tokens_out"`
	Errors      int           `json:"errors"`
}

// MetricsSummary contains aggregated metrics for the session.
type MetricsSummary struct {
	Providers      map[string]ProviderMetrics `json:"providers"`
	TotalDuration  time.Duration              `json:"total_duration_ms"`
	TotalCostUSD   float64                    `json:"total_cost_usd"`
	TotalTokensIn  int                        `json:"total_tokens_in"`
	TotalTokensOut int                        `json:"total_tokens_out"`
}

// Metrics is a thread-safe invocation metrics collector.
type Metrics struct {
	mu        sync.Mutex
	providers map[string]*ProviderMetrics
	start     time.Time
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		providers: make(map[string]*ProviderMetrics),
		start:     time.Now(),
	}
}

// RecordInvocation records a single provider invocation result.
func (m *Metrics) RecordInvocation(provider string, duration time.Duration, costUSD float64, tokensIn, tokensOut int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pm, ok := m.providers[provider]
	if !ok {
		pm = &ProviderMetrics{}
		m.providers[provider] = pm
	}

	pm.Invocations++
	pm.Duration += duration
	pm.CostUSD += costUSD
	pm.TokensIn += tokensIn
	pm.TokensOut += tokensOut
	if err != nil {
		pm.Errors++
	}
}

// Summary returns aggregated metrics for the session.
func (m *Metrics) Summary() MetricsSummary {
	m.mu.Lock()
	defer m.mu.Unlock()

	summary := MetricsSummary{
		Providers:     make(map[string]ProviderMetrics, len(m.providers)),
		TotalDuration: time.Since(m.start),
	}

	for name, pm := range m.providers {
		summary.Providers[name] = *pm
		summary.TotalCostUSD += pm.CostUSD
		summary.TotalTokensIn += pm.TokensIn
		summary.TotalTokensOut += pm.TokensOut
	}

	return summary
}

// ProviderStats returns metrics for a specific provider. Returns zero value if not found.
func (m *Metrics) ProviderStats(provider string) ProviderMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pm, ok := m.providers[provider]; ok {
		return *pm
	}
	return ProviderMetrics{}
}
