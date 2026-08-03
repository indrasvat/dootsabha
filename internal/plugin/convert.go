// Package plugin provides gRPC plugin infrastructure for दूतसभा.
// This file contains conversion helpers between internal Go types and
// protobuf-generated types.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
	"github.com/indrasvat/dootsabha/internal/providers"
	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// ── Provider types ─────────────────────────────────────────────────────────────

// ProviderResultToProto converts a providers.ProviderResult to a proto InvokeResponse.
func ProviderResultToProto(r *providers.ProviderResult) *gen.InvokeResponse {
	return &gen.InvokeResponse{
		Content:    r.Content,
		Provider:   "", // filled by caller (provider name not stored in ProviderResult)
		Model:      r.Model,
		SessionId:  r.SessionID,
		CostUsd:    r.CostUSD,
		TokensIn:   int32(r.TokensIn),
		TokensOut:  int32(r.TokensOut),
		DurationMs: r.Duration.Milliseconds(),
	}
}

// ProtoToProviderResult converts a proto InvokeResponse back to a providers.ProviderResult.
func ProtoToProviderResult(p *gen.InvokeResponse) *providers.ProviderResult {
	return &providers.ProviderResult{
		Content:   p.GetContent(),
		Model:     p.GetModel(),
		SessionID: p.GetSessionId(),
		CostUSD:   p.GetCostUsd(),
		TokensIn:  int(p.GetTokensIn()),
		TokensOut: int(p.GetTokensOut()),
		Duration:  time.Duration(p.GetDurationMs()) * time.Millisecond,
	}
}

// HealthStatusToProto converts a providers.HealthStatus to a proto HealthCheckResponse.
func HealthStatusToProto(h *providers.HealthStatus) *gen.HealthCheckResponse {
	return &gen.HealthCheckResponse{
		Healthy:    h.Healthy,
		CliVersion: h.CLIVersion,
		Model:      h.Model,
		AuthValid:  h.AuthValid,
		Error:      h.Error,
	}
}

// ProtoToHealthStatus converts a proto HealthCheckResponse back to a providers.HealthStatus.
func ProtoToHealthStatus(p *gen.HealthCheckResponse) *providers.HealthStatus {
	return &providers.HealthStatus{
		Healthy:    p.GetHealthy(),
		CLIVersion: p.GetCliVersion(),
		Model:      p.GetModel(),
		AuthValid:  p.GetAuthValid(),
		Error:      p.GetError(),
	}
}

// InvokeOptionsToProto converts a providers.InvokeOptions to proto InvokeRequest fields.
// Only the option fields are set; prompt must be set by the caller.
func InvokeOptionsToProto(opts providers.InvokeOptions) *gen.InvokeRequest {
	return &gen.InvokeRequest{
		Model:     opts.Model,
		MaxTurns:  int32(opts.MaxTurns),
		TimeoutMs: opts.Timeout.Milliseconds(),
	}
}

// ProtoToInvokeOptions converts a proto InvokeRequest's option fields to providers.InvokeOptions.
func ProtoToInvokeOptions(p *gen.InvokeRequest) providers.InvokeOptions {
	return providers.InvokeOptions{
		Model:    p.GetModel(),
		MaxTurns: int(p.GetMaxTurns()),
		Timeout:  time.Duration(p.GetTimeoutMs()) * time.Millisecond,
	}
}

// ── Engine/strategy types ──────────────────────────────────────────────────────

// errorToProto splits an error into the two things the wire can carry: its text
// and whether it was a deadline.
//
// The sentinel cannot survive as text. Rebuilding an error from a string on the
// far side produces something errors.Is(err, context.DeadlineExceeded) rejects,
// so a timed-out agent that crossed this boundary stopped being a timeout and
// the caller's exit code silently fell from 4 to 5 (or 3). Matching the message
// text to guess would be worse — it would make the exit contract depend on
// wording. The fact travels as its own field instead.
func errorToProto(err error) (msg string, timedOut bool) {
	if err == nil {
		return "", false
	}
	return err.Error(), errors.Is(err, context.DeadlineExceeded)
}

// protoToError rebuilds an error, re-attaching the deadline sentinel when the
// far side said so, so errors.Is keeps working across the boundary.
func protoToError(msg string, timedOut bool) error {
	switch {
	case msg == "":
		return nil
	case timedOut:
		return fmt.Errorf("%s: %w", msg, context.DeadlineExceeded)
	default:
		return errors.New(msg)
	}
}

// DispatchResultToProto converts a core.DispatchResult to a proto DispatchResult.
func DispatchResultToProto(d *core.DispatchResult) *gen.DispatchResult {
	errStr, timedOut := errorToProto(d.Error)
	return &gen.DispatchResult{
		Provider:   d.Provider,
		Model:      d.Model,
		Content:    d.Content,
		DurationMs: d.Duration.Milliseconds(),
		CostUsd:    d.CostUSD,
		TokensIn:   int32(d.TokensIn),
		TokensOut:  int32(d.TokensOut),
		Error:      errStr,
		TimedOut:   timedOut,
	}
}

// ProtoToDispatchResult converts a proto DispatchResult back to a core.DispatchResult.
func ProtoToDispatchResult(p *gen.DispatchResult) *core.DispatchResult {
	err := protoToError(p.GetError(), p.GetTimedOut())
	return &core.DispatchResult{
		Provider:  p.GetProvider(),
		Model:     p.GetModel(),
		Content:   p.GetContent(),
		Duration:  time.Duration(p.GetDurationMs()) * time.Millisecond,
		CostUSD:   p.GetCostUsd(),
		TokensIn:  int(p.GetTokensIn()),
		TokensOut: int(p.GetTokensOut()),
		Error:     err,
	}
}

// ReviewResultToProto converts a core.ReviewResult to a proto ReviewResult.
func ReviewResultToProto(r *core.ReviewResult) *gen.ReviewResult {
	errStr, timedOut := errorToProto(r.Error)
	return &gen.ReviewResult{
		Reviewer:   r.Reviewer,
		Reviewed:   r.Reviewed,
		Content:    r.Content,
		DurationMs: r.Duration.Milliseconds(),
		CostUsd:    r.CostUSD,
		TokensIn:   int32(r.TokensIn),
		TokensOut:  int32(r.TokensOut),
		Error:      errStr,
		TimedOut:   timedOut,
	}
}

// ProtoToReviewResult converts a proto ReviewResult back to a core.ReviewResult.
func ProtoToReviewResult(p *gen.ReviewResult) *core.ReviewResult {
	err := protoToError(p.GetError(), p.GetTimedOut())
	return &core.ReviewResult{
		Reviewer:  p.GetReviewer(),
		Reviewed:  p.GetReviewed(),
		Content:   p.GetContent(),
		Duration:  time.Duration(p.GetDurationMs()) * time.Millisecond,
		CostUSD:   p.GetCostUsd(),
		TokensIn:  int(p.GetTokensIn()),
		TokensOut: int(p.GetTokensOut()),
		Error:     err,
	}
}

// SynthesisResultToProto converts a core.SynthesisResult to a proto SynthesisResult.
func SynthesisResultToProto(s *core.SynthesisResult) *gen.SynthesisResult {
	chairErr, timedOut := errorToProto(s.ChairError)
	return &gen.SynthesisResult{
		Chair:         s.Chair,
		ChairFallback: s.ChairFallback,
		Content:       s.Content,
		DurationMs:    s.Duration.Milliseconds(),
		CostUsd:       s.CostUSD,
		TokensIn:      int32(s.TokensIn),
		TokensOut:     int32(s.TokensOut),
		ChairError:    chairErr,
		TimedOut:      timedOut,
	}
}

// ProtoToSynthesisResult converts a proto SynthesisResult back to a core.SynthesisResult.
func ProtoToSynthesisResult(p *gen.SynthesisResult) *core.SynthesisResult {
	chairErr := protoToError(p.GetChairError(), p.GetTimedOut())
	return &core.SynthesisResult{
		Chair:         p.GetChair(),
		ChairFallback: p.GetChairFallback(),
		Content:       p.GetContent(),
		Duration:      time.Duration(p.GetDurationMs()) * time.Millisecond,
		CostUSD:       p.GetCostUsd(),
		TokensIn:      int(p.GetTokensIn()),
		TokensOut:     int(p.GetTokensOut()),
		ChairError:    chairErr,
	}
}
