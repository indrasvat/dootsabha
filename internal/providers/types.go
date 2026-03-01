// Package providers defines the Provider interface and shared types for
// all दूतसभा agent backends (Claude, Codex, Gemini).
package providers

import (
	"context"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

// Runner abstracts subprocess execution so providers can be unit-tested
// without spawning real processes. *core.SubprocessRunner satisfies this
// interface directly.
type Runner interface {
	Run(ctx context.Context, binary string, args []string, opts ...core.RunOption) (*core.SubprocessResult, error)
}

// Provider is the interface all agent providers implement.
type Provider interface {
	Name() string
	Invoke(ctx context.Context, prompt string, opts InvokeOptions) (*ProviderResult, error)
	HealthCheck(ctx context.Context) (*HealthStatus, error)
}

// InvokeOptions configures a single provider invocation.
type InvokeOptions struct {
	Model    string        // override default model (empty = use config default)
	MaxTurns int           // max agent turns (0 = no limit)
	Timeout  time.Duration // per-invocation timeout (0 = use config default)
}

// ProviderResult holds the structured output of a successful provider invocation.
type ProviderResult struct {
	Content   string
	Model     string
	Duration  time.Duration
	CostUSD   float64
	TokensIn  int
	TokensOut int
	SessionID string
}

// HealthStatus reports the current health of a provider CLI.
type HealthStatus struct {
	Healthy    bool
	CLIVersion string
	Model      string
	AuthValid  bool
	Error      string
}
