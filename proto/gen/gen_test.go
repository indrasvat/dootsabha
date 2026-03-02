package gen_test

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	gen "github.com/indrasvat/dootsabha/proto/gen"
)

// ── Message Construction ───────────────────────────────────────────────────────

func TestInvokeRequestConstruction(t *testing.T) {
	req := &gen.InvokeRequest{
		Prompt:       "explain concurrency in Go",
		Model:        "claude-sonnet-4-6",
		MaxTurns:     3,
		Temperature:  0.7,
		OutputFormat: "json",
		ExtraArgs:    []string{"--dangerously-skip-permissions", "--verbose"},
		Env:          map[string]string{"HOME": "/tmp", "LANG": "en_US.UTF-8"},
		WorkDir:      "/home/user/project",
		TimeoutMs:    30000,
	}

	if req.GetPrompt() != "explain concurrency in Go" {
		t.Errorf("Prompt = %q, want %q", req.GetPrompt(), "explain concurrency in Go")
	}
	if req.GetModel() != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", req.GetModel(), "claude-sonnet-4-6")
	}
	if req.GetMaxTurns() != 3 {
		t.Errorf("MaxTurns = %d, want 3", req.GetMaxTurns())
	}
	if req.GetTemperature() != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", req.GetTemperature())
	}
	if req.GetOutputFormat() != "json" {
		t.Errorf("OutputFormat = %q, want %q", req.GetOutputFormat(), "json")
	}
	if len(req.GetExtraArgs()) != 2 {
		t.Errorf("ExtraArgs len = %d, want 2", len(req.GetExtraArgs()))
	}
	if len(req.GetEnv()) != 2 {
		t.Errorf("Env len = %d, want 2", len(req.GetEnv()))
	}
	if req.GetWorkDir() != "/home/user/project" {
		t.Errorf("WorkDir = %q, want %q", req.GetWorkDir(), "/home/user/project")
	}
	if req.GetTimeoutMs() != 30000 {
		t.Errorf("TimeoutMs = %d, want 30000", req.GetTimeoutMs())
	}
}

func TestInvokeResponseConstruction(t *testing.T) {
	resp := &gen.InvokeResponse{
		Content:    "Concurrency in Go uses goroutines...",
		RawJson:    []byte(`{"result":"test"}`),
		Provider:   "claude",
		Model:      "claude-sonnet-4-6",
		SessionId:  "sess_abc123",
		CostUsd:    0.0042,
		TokensIn:   150,
		TokensOut:  320,
		DurationMs: 2500,
		ExitCode:   0,
		Stderr:     "",
	}

	if resp.GetContent() != "Concurrency in Go uses goroutines..." {
		t.Errorf("Content mismatch")
	}
	if string(resp.GetRawJson()) != `{"result":"test"}` {
		t.Errorf("RawJson = %q", resp.GetRawJson())
	}
	if resp.GetProvider() != "claude" {
		t.Errorf("Provider = %q", resp.GetProvider())
	}
	if resp.GetModel() != "claude-sonnet-4-6" {
		t.Errorf("Model = %q", resp.GetModel())
	}
	if resp.GetSessionId() != "sess_abc123" {
		t.Errorf("SessionId = %q", resp.GetSessionId())
	}
	if resp.GetCostUsd() != 0.0042 {
		t.Errorf("CostUsd = %f", resp.GetCostUsd())
	}
	if resp.GetTokensIn() != 150 {
		t.Errorf("TokensIn = %d", resp.GetTokensIn())
	}
	if resp.GetTokensOut() != 320 {
		t.Errorf("TokensOut = %d", resp.GetTokensOut())
	}
	if resp.GetDurationMs() != 2500 {
		t.Errorf("DurationMs = %d", resp.GetDurationMs())
	}
	if resp.GetExitCode() != 0 {
		t.Errorf("ExitCode = %d", resp.GetExitCode())
	}
	if resp.GetStderr() != "" {
		t.Errorf("Stderr = %q", resp.GetStderr())
	}
}

func TestHealthCheckResponseConstruction(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		resp := &gen.HealthCheckResponse{
			Healthy:    true,
			CliVersion: "2.1.63",
			Model:      "claude-sonnet-4-6",
			AuthValid:  true,
			Error:      "",
		}
		if !resp.GetHealthy() {
			t.Error("expected Healthy=true")
		}
		if resp.GetCliVersion() != "2.1.63" {
			t.Errorf("CliVersion = %q", resp.GetCliVersion())
		}
		if !resp.GetAuthValid() {
			t.Error("expected AuthValid=true")
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		resp := &gen.HealthCheckResponse{
			Healthy: false,
			Error:   "binary not found: claude",
		}
		if resp.GetHealthy() {
			t.Error("expected Healthy=false")
		}
		if resp.GetError() != "binary not found: claude" {
			t.Errorf("Error = %q", resp.GetError())
		}
	})
}

func TestCapabilitiesResponseConstruction(t *testing.T) {
	resp := &gen.CapabilitiesResponse{
		SupportsJson:      true,
		SupportsStreaming: false,
		SupportedModels:   []string{"claude-sonnet-4-6", "claude-haiku-4-5"},
		DefaultModel:      "claude-sonnet-4-6",
		MaxContextTokens:  200000,
	}

	if !resp.GetSupportsJson() {
		t.Error("expected SupportsJson=true")
	}
	if resp.GetSupportsStreaming() {
		t.Error("expected SupportsStreaming=false")
	}
	if len(resp.GetSupportedModels()) != 2 {
		t.Errorf("SupportedModels len = %d, want 2", len(resp.GetSupportedModels()))
	}
	if resp.GetSupportedModels()[0] != "claude-sonnet-4-6" {
		t.Errorf("SupportedModels[0] = %q", resp.GetSupportedModels()[0])
	}
	if resp.GetDefaultModel() != "claude-sonnet-4-6" {
		t.Errorf("DefaultModel = %q", resp.GetDefaultModel())
	}
	if resp.GetMaxContextTokens() != 200000 {
		t.Errorf("MaxContextTokens = %d", resp.GetMaxContextTokens())
	}
}

func TestSessionSummaryConstruction(t *testing.T) {
	ss := &gen.SessionSummary{
		SessionId:    "ds_a1b2c3",
		Strategy:     "council",
		Providers:    []string{"claude", "codex", "gemini"},
		TotalCostUsd: 0.015,
		TotalTokens:  1200,
		DurationMs:   8500,
		Errors:       []string{"codex: timeout after 30s"},
	}

	if ss.GetSessionId() != "ds_a1b2c3" {
		t.Errorf("SessionId = %q", ss.GetSessionId())
	}
	if ss.GetStrategy() != "council" {
		t.Errorf("Strategy = %q", ss.GetStrategy())
	}
	if len(ss.GetProviders()) != 3 {
		t.Errorf("Providers len = %d, want 3", len(ss.GetProviders()))
	}
	if ss.GetTotalCostUsd() != 0.015 {
		t.Errorf("TotalCostUsd = %f", ss.GetTotalCostUsd())
	}
	if ss.GetTotalTokens() != 1200 {
		t.Errorf("TotalTokens = %d", ss.GetTotalTokens())
	}
	if len(ss.GetErrors()) != 1 {
		t.Errorf("Errors len = %d, want 1", len(ss.GetErrors()))
	}
}

// ── Serialization Roundtrips ───────────────────────────────────────────────────

func TestInvokeRequestSerializationRoundtrip(t *testing.T) {
	original := &gen.InvokeRequest{
		Prompt:       "review this code",
		Model:        "codex-mini",
		MaxTurns:     5,
		Temperature:  0.3,
		OutputFormat: "stream-json",
		ExtraArgs:    []string{"--sandbox", "full"},
		Env:          map[string]string{"API_KEY": "sk-test", "DEBUG": "1"},
		WorkDir:      "/workspace",
		TimeoutMs:    60000,
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &gen.InvokeRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Error("roundtrip mismatch: original != decoded")
	}
}

func TestInvokeResponseSerializationRoundtrip(t *testing.T) {
	original := &gen.InvokeResponse{
		Content:    "The code looks correct.",
		RawJson:    []byte(`{"is_error":false,"result":"OK","session_id":"s1"}`),
		Provider:   "claude",
		Model:      "claude-sonnet-4-6",
		SessionId:  "s1",
		CostUsd:    0.003,
		TokensIn:   200,
		TokensOut:  150,
		DurationMs: 3200,
		ExitCode:   0,
		Stderr:     "warning: deprecated flag",
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Error("roundtrip mismatch")
	}
	// Explicitly verify raw_json bytes survived
	if string(decoded.GetRawJson()) != `{"is_error":false,"result":"OK","session_id":"s1"}` {
		t.Errorf("RawJson corrupted: %q", decoded.GetRawJson())
	}
}

func TestExecuteRequestSerializationRoundtrip(t *testing.T) {
	original := &gen.ExecuteRequest{
		Prompt: "design a REST API",
		Agents: []*gen.AgentConfig{
			{Name: "claude", Model: "claude-sonnet-4-6", TimeoutMs: 30000},
			{Name: "codex", Model: "codex-mini", TimeoutMs: 45000, ExtraArgs: []string{"--sandbox", "full"}},
			{Name: "gemini", Model: "gemini-3-pro", TimeoutMs: 30000},
		},
		Config: &gen.StrategyConfig{
			Parallel:     true,
			Rounds:       2,
			Chair:        "claude",
			StrategyName: "council",
			Anonymous:    true,
		},
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &gen.ExecuteRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Error("roundtrip mismatch")
	}
	if len(decoded.GetAgents()) != 3 {
		t.Errorf("agents len = %d, want 3", len(decoded.GetAgents()))
	}
	if decoded.GetConfig().GetRounds() != 2 {
		t.Errorf("rounds = %d, want 2", decoded.GetConfig().GetRounds())
	}
}

func TestExecuteResponseSerializationRoundtrip(t *testing.T) {
	original := &gen.ExecuteResponse{
		DispatchResults: []*gen.DispatchResult{
			{Provider: "claude", Model: "claude-sonnet-4-6", Content: "API design v1", DurationMs: 3000, CostUsd: 0.005, TokensIn: 100, TokensOut: 200},
			{Provider: "codex", Model: "codex-mini", Content: "API design v2", DurationMs: 4000, CostUsd: 0.003, TokensIn: 100, TokensOut: 180},
			{Provider: "gemini", Content: "", Error: "timeout after 30s"},
		},
		ReviewResults: []*gen.ReviewResult{
			{Reviewer: "claude", Reviewed: []string{"codex", "gemini"}, Content: "review of codex and gemini", DurationMs: 2000, TokensIn: 300, TokensOut: 100},
			{Reviewer: "codex", Reviewed: []string{"claude", "gemini"}, Content: "review of claude and gemini", DurationMs: 2500, TokensIn: 350, TokensOut: 120},
			{Reviewer: "gemini", Error: "skipped: dispatch failed"},
		},
		Synthesis: &gen.SynthesisResult{
			Chair: "claude", Content: "synthesized output", DurationMs: 3000, CostUsd: 0.004, TokensIn: 500, TokensOut: 250,
		},
		Metadata: &gen.SessionMeta{
			TotalCostUsd:    0.012,
			TotalTokensIn:   1350,
			TotalTokensOut:  850,
			TotalDurationMs: 14500,
			ProvidersStatus: map[string]string{"claude": "healthy", "codex": "healthy", "gemini": "error:timeout"},
		},
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &gen.ExecuteResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Error("roundtrip mismatch")
	}
}

func TestHookRequestSerializationRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		req  *gen.HookRequest
	}{
		{
			name: "pre_invoke",
			req: &gen.HookRequest{
				EventType: gen.EventType_PRE_INVOKE,
				Metadata:  map[string]string{"trace_id": "t1"},
				Payload:   &gen.HookRequest_InvokeRequest{InvokeRequest: &gen.InvokeRequest{Prompt: "hello"}},
			},
		},
		{
			name: "post_invoke",
			req: &gen.HookRequest{
				EventType: gen.EventType_POST_INVOKE,
				Payload:   &gen.HookRequest_InvokeResponse{InvokeResponse: &gen.InvokeResponse{Content: "world", Provider: "claude"}},
			},
		},
		{
			name: "pre_synthesis",
			req: &gen.HookRequest{
				EventType: gen.EventType_PRE_SYNTHESIS,
				Payload: &gen.HookRequest_InvokeResponses{InvokeResponses: &gen.InvokeResponseList{
					Responses: []*gen.InvokeResponse{
						{Content: "r1", Provider: "claude"},
						{Content: "r2", Provider: "codex"},
					},
				}},
			},
		},
		{
			name: "post_session",
			req: &gen.HookRequest{
				EventType: gen.EventType_POST_SESSION,
				Payload: &gen.HookRequest_SessionSummary{SessionSummary: &gen.SessionSummary{
					SessionId: "ds_123", Strategy: "council", Providers: []string{"claude", "codex"},
					TotalCostUsd: 0.01, TotalTokens: 500, DurationMs: 5000,
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := proto.Marshal(tt.req)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			decoded := &gen.HookRequest{}
			if err := proto.Unmarshal(data, decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !proto.Equal(tt.req, decoded) {
				t.Error("roundtrip mismatch")
			}
		})
	}
}

func TestCancelRequestResponseRoundtrip(t *testing.T) {
	req := &gen.CancelRequest{SessionId: "sess_abc123"}
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	decodedReq := &gen.CancelRequest{}
	if err := proto.Unmarshal(data, decodedReq); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if !proto.Equal(req, decodedReq) {
		t.Error("request roundtrip mismatch")
	}

	resp := &gen.CancelResponse{Cancelled: true}
	data, err = proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	decodedResp := &gen.CancelResponse{}
	if err := proto.Unmarshal(data, decodedResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !decodedResp.GetCancelled() {
		t.Error("expected Cancelled=true after roundtrip")
	}
}

// ── Proto3 Semantics ───────────────────────────────────────────────────────────

func TestDefaultValues(t *testing.T) {
	req := &gen.InvokeRequest{Prompt: "test"}

	if req.GetModel() != "" {
		t.Errorf("default Model = %q, want empty", req.GetModel())
	}
	if req.GetMaxTurns() != 0 {
		t.Errorf("default MaxTurns = %d, want 0", req.GetMaxTurns())
	}
	if req.GetTemperature() != 0.0 {
		t.Errorf("default Temperature = %f, want 0.0", req.GetTemperature())
	}
	if req.GetTimeoutMs() != 0 {
		t.Errorf("default TimeoutMs = %d, want 0", req.GetTimeoutMs())
	}
	if req.GetOutputFormat() != "" {
		t.Errorf("default OutputFormat = %q, want empty", req.GetOutputFormat())
	}
	if len(req.GetExtraArgs()) != 0 {
		t.Errorf("default ExtraArgs len = %d, want 0", len(req.GetExtraArgs()))
	}
	if len(req.GetEnv()) != 0 {
		t.Errorf("default Env len = %d, want 0", len(req.GetEnv()))
	}
	if req.GetWorkDir() != "" {
		t.Errorf("default WorkDir = %q, want empty", req.GetWorkDir())
	}
}

func TestZeroValueBehavior(t *testing.T) {
	// Proto3 omits zero values on wire — verify they survive roundtrip.
	original := &gen.InvokeResponse{
		Content:    "test",
		Provider:   "claude",
		CostUsd:    0.0,
		TokensIn:   0,
		TokensOut:  0,
		DurationMs: 0,
		ExitCode:   0,
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.GetCostUsd() != 0.0 {
		t.Errorf("CostUsd = %f, want 0.0", decoded.GetCostUsd())
	}
	if decoded.GetTokensIn() != 0 {
		t.Errorf("TokensIn = %d, want 0", decoded.GetTokensIn())
	}
	if decoded.GetTokensOut() != 0 {
		t.Errorf("TokensOut = %d, want 0", decoded.GetTokensOut())
	}
	if decoded.GetDurationMs() != 0 {
		t.Errorf("DurationMs = %d, want 0", decoded.GetDurationMs())
	}
	if decoded.GetExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", decoded.GetExitCode())
	}
}

func TestRepeatedFieldOperations(t *testing.T) {
	counts := []int{0, 1, 3, 5}
	for _, n := range counts {
		agents := make([]*gen.AgentConfig, n)
		for i := range agents {
			agents[i] = &gen.AgentConfig{Name: strings.Repeat("a", i+1)}
		}
		req := &gen.ExecuteRequest{Prompt: "test", Agents: agents}

		data, err := proto.Marshal(req)
		if err != nil {
			t.Fatalf("marshal (n=%d): %v", n, err)
		}

		decoded := &gen.ExecuteRequest{}
		if err := proto.Unmarshal(data, decoded); err != nil {
			t.Fatalf("unmarshal (n=%d): %v", n, err)
		}

		if len(decoded.GetAgents()) != n {
			t.Errorf("n=%d: got %d agents", n, len(decoded.GetAgents()))
		}
	}

	// Test append after construction
	req := &gen.ExecuteRequest{
		Prompt: "test",
		Agents: []*gen.AgentConfig{{Name: "a"}},
	}
	req.Agents = append(req.Agents, &gen.AgentConfig{Name: "b"})

	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal append: %v", err)
	}
	decoded := &gen.ExecuteRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal append: %v", err)
	}
	if len(decoded.GetAgents()) != 2 {
		t.Errorf("after append: got %d agents, want 2", len(decoded.GetAgents()))
	}
}

func TestMapFieldOperations(t *testing.T) {
	req := &gen.InvokeRequest{
		Prompt: "test",
		Env: map[string]string{
			"KEY":     "VALUE",
			"EMPTY":   "",
			"UNICODE": "दूतसभा",
		},
	}

	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &gen.InvokeRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	env := decoded.GetEnv()
	if len(env) != 3 {
		t.Fatalf("env len = %d, want 3", len(env))
	}
	if env["KEY"] != "VALUE" {
		t.Errorf("env[KEY] = %q", env["KEY"])
	}
	if env["EMPTY"] != "" {
		t.Errorf("env[EMPTY] = %q, want empty", env["EMPTY"])
	}
	if env["UNICODE"] != "दूतसभा" {
		t.Errorf("env[UNICODE] = %q, want दूतसभा", env["UNICODE"])
	}
}

// ── Enum & Oneof ───────────────────────────────────────────────────────────────

func TestEventTypeEnum(t *testing.T) {
	tests := []struct {
		event gen.EventType
		val   int32
		name  string
	}{
		{gen.EventType_PRE_INVOKE, 0, "PRE_INVOKE"},
		{gen.EventType_POST_INVOKE, 1, "POST_INVOKE"},
		{gen.EventType_PRE_SYNTHESIS, 2, "PRE_SYNTHESIS"},
		{gen.EventType_POST_SESSION, 3, "POST_SESSION"},
	}

	for _, tt := range tests {
		if int32(tt.event) != tt.val {
			t.Errorf("%s = %d, want %d", tt.name, int32(tt.event), tt.val)
		}
		if tt.event.String() != tt.name {
			t.Errorf("%d.String() = %q, want %q", tt.val, tt.event.String(), tt.name)
		}
	}
}

func TestHookRequestOneofPayloads(t *testing.T) {
	// PRE_INVOKE: InvokeRequest
	req1 := &gen.HookRequest{
		EventType: gen.EventType_PRE_INVOKE,
		Payload:   &gen.HookRequest_InvokeRequest{InvokeRequest: &gen.InvokeRequest{Prompt: "hello"}},
	}
	if _, ok := req1.Payload.(*gen.HookRequest_InvokeRequest); !ok {
		t.Error("PRE_INVOKE: expected InvokeRequest oneof")
	}
	if req1.GetInvokeResponse() != nil {
		t.Error("PRE_INVOKE: InvokeResponse should be nil")
	}
	if req1.GetInvokeResponses() != nil {
		t.Error("PRE_INVOKE: InvokeResponses should be nil")
	}
	if req1.GetSessionSummary() != nil {
		t.Error("PRE_INVOKE: SessionSummary should be nil")
	}

	// POST_INVOKE: InvokeResponse
	req2 := &gen.HookRequest{
		EventType: gen.EventType_POST_INVOKE,
		Payload:   &gen.HookRequest_InvokeResponse{InvokeResponse: &gen.InvokeResponse{Content: "world"}},
	}
	if _, ok := req2.Payload.(*gen.HookRequest_InvokeResponse); !ok {
		t.Error("POST_INVOKE: expected InvokeResponse oneof")
	}
	if req2.GetInvokeRequest() != nil {
		t.Error("POST_INVOKE: InvokeRequest should be nil")
	}

	// PRE_SYNTHESIS: InvokeResponseList
	req3 := &gen.HookRequest{
		EventType: gen.EventType_PRE_SYNTHESIS,
		Payload: &gen.HookRequest_InvokeResponses{InvokeResponses: &gen.InvokeResponseList{
			Responses: []*gen.InvokeResponse{{Content: "r1"}, {Content: "r2"}},
		}},
	}
	if list := req3.GetInvokeResponses(); list == nil || len(list.GetResponses()) != 2 {
		t.Error("PRE_SYNTHESIS: expected 2 responses in list")
	}

	// POST_SESSION: SessionSummary
	req4 := &gen.HookRequest{
		EventType: gen.EventType_POST_SESSION,
		Payload:   &gen.HookRequest_SessionSummary{SessionSummary: &gen.SessionSummary{SessionId: "ds_1"}},
	}
	if ss := req4.GetSessionSummary(); ss == nil || ss.GetSessionId() != "ds_1" {
		t.Error("POST_SESSION: expected SessionSummary with id ds_1")
	}
}

func TestHookResponseOneofModifiedPayload(t *testing.T) {
	// Modified InvokeRequest (PreInvoke hook rewrites prompt)
	resp1 := &gen.HookResponse{
		Proceed: true,
		ModifiedPayload: &gen.HookResponse_ModifiedInvokeRequest{
			ModifiedInvokeRequest: &gen.InvokeRequest{Prompt: "[HOOK] original prompt"},
		},
	}
	if req := resp1.GetModifiedInvokeRequest(); req == nil || req.GetPrompt() != "[HOOK] original prompt" {
		t.Error("expected modified InvokeRequest with rewritten prompt")
	}
	if resp1.GetModifiedInvokeResponse() != nil {
		t.Error("ModifiedInvokeResponse should be nil when request is set")
	}

	// Modified InvokeResponse (PostInvoke hook redacts content)
	resp2 := &gen.HookResponse{
		Proceed: true,
		ModifiedPayload: &gen.HookResponse_ModifiedInvokeResponse{
			ModifiedInvokeResponse: &gen.InvokeResponse{Content: "[REDACTED]"},
		},
	}
	if r := resp2.GetModifiedInvokeResponse(); r == nil || r.GetContent() != "[REDACTED]" {
		t.Error("expected modified InvokeResponse with redacted content")
	}
}

// ── Edge Cases: Strings & Sizes ────────────────────────────────────────────────

func TestEmptyStrings(t *testing.T) {
	req := &gen.InvokeRequest{Prompt: ""}
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.InvokeRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.GetPrompt() != "" {
		t.Errorf("empty prompt became %q", decoded.GetPrompt())
	}

	resp := &gen.InvokeResponse{Content: "", Provider: "claude"}
	data, err = proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decodedResp := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decodedResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decodedResp.GetContent() != "" {
		t.Errorf("empty content became %q", decodedResp.GetContent())
	}
}

func TestMaxContentSize32KB(t *testing.T) {
	content := strings.Repeat("A", 32768) // 32KB — truncation limit
	resp := &gen.InvokeResponse{Content: content, Provider: "claude"}

	data, err := proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal 32KB: %v", err)
	}

	decoded := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal 32KB: %v", err)
	}

	if len(decoded.GetContent()) != 32768 {
		t.Errorf("content len = %d, want 32768", len(decoded.GetContent()))
	}
}

func TestLargeContent128KB(t *testing.T) {
	content := strings.Repeat("B", 131072) // 128KB — pre-truncation size
	resp := &gen.InvokeResponse{Content: content, Provider: "claude"}

	data, err := proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal 128KB: %v", err)
	}

	decoded := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal 128KB: %v", err)
	}

	if len(decoded.GetContent()) != 131072 {
		t.Errorf("content len = %d, want 131072", len(decoded.GetContent()))
	}
}

func TestUnicodeContentDevanagari(t *testing.T) {
	prompt := "दूतसभा कैसे काम करता है?"
	response := "दूतसभा एक AI एजेंट ऑर्केस्ट्रेटर है।"

	req := &gen.InvokeRequest{Prompt: prompt}
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decodedReq := &gen.InvokeRequest{}
	if err := proto.Unmarshal(data, decodedReq); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decodedReq.GetPrompt() != prompt {
		t.Errorf("prompt = %q, want %q", decodedReq.GetPrompt(), prompt)
	}

	resp := &gen.InvokeResponse{Content: response, Provider: "claude"}
	data, err = proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decodedResp := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decodedResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decodedResp.GetContent() != response {
		t.Errorf("content = %q, want %q", decodedResp.GetContent(), response)
	}
}

func TestUnicodeContentEmoji(t *testing.T) {
	content := "Analysis complete \U0001f44d Score: 9/10 \u2705"
	resp := &gen.InvokeResponse{Content: content, Provider: "claude"}

	data, err := proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.GetContent() != content {
		t.Errorf("emoji content mismatch")
	}
}

func TestUnicodeContentCJK(t *testing.T) {
	content := "分析完了。コードは正しいです。코드가 올바릅니다."
	resp := &gen.InvokeResponse{Content: content, Provider: "gemini"}

	data, err := proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.GetContent() != content {
		t.Errorf("CJK content mismatch")
	}
}

// ── Edge Cases: Numbers & Collections ──────────────────────────────────────────

func TestNegativeCostUSD(t *testing.T) {
	resp := &gen.InvokeResponse{Content: "test", Provider: "claude", CostUsd: -0.001}

	data, err := proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.InvokeResponse{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.GetCostUsd() != -0.001 {
		t.Errorf("CostUsd = %f, want -0.001", decoded.GetCostUsd())
	}
}

func TestMaxExtraArgs(t *testing.T) {
	args := make([]string, 100)
	for i := range args {
		args[i] = strings.Repeat("x", i+1)
	}
	req := &gen.InvokeRequest{Prompt: "test", ExtraArgs: args}

	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.InvokeRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.GetExtraArgs()) != 100 {
		t.Fatalf("ExtraArgs len = %d, want 100", len(decoded.GetExtraArgs()))
	}
	// Verify order preserved
	for i, arg := range decoded.GetExtraArgs() {
		expected := strings.Repeat("x", i+1)
		if arg != expected {
			t.Errorf("ExtraArgs[%d] = %q, want %q", i, arg, expected)
			break
		}
	}
}

func TestMaxEnvMapEntries(t *testing.T) {
	env := make(map[string]string, 50)
	for i := range 50 {
		env[strings.Repeat("k", i+1)] = strings.Repeat("v", i+1)
	}
	req := &gen.InvokeRequest{Prompt: "test", Env: env}

	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.InvokeRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.GetEnv()) != 50 {
		t.Errorf("Env len = %d, want 50", len(decoded.GetEnv()))
	}
}

func TestMaxAgentsInExecuteRequest(t *testing.T) {
	agents := make([]*gen.AgentConfig, 5) // MaxAgents boundary
	for i := range agents {
		agents[i] = &gen.AgentConfig{Name: strings.Repeat("a", i+1)}
	}
	req := &gen.ExecuteRequest{Prompt: "test", Agents: agents}

	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.ExecuteRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.GetAgents()) != 5 {
		t.Errorf("agents len = %d, want 5", len(decoded.GetAgents()))
	}

	// Also test 0 agents (empty repeated field)
	empty := &gen.ExecuteRequest{Prompt: "test"}
	data, err = proto.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	decodedEmpty := &gen.ExecuteRequest{}
	if err := proto.Unmarshal(data, decodedEmpty); err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	if len(decodedEmpty.GetAgents()) != 0 {
		t.Errorf("empty agents len = %d, want 0", len(decodedEmpty.GetAgents()))
	}
}

func TestMaxProvidersInSessionSummary(t *testing.T) {
	ss := &gen.SessionSummary{
		SessionId: "ds_1",
		Strategy:  "council",
		Providers: []string{"claude", "codex", "gemini", "ollama", "deepseek"},
	}

	data, err := proto.Marshal(ss)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.SessionSummary{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.GetProviders()) != 5 {
		t.Errorf("providers len = %d, want 5", len(decoded.GetProviders()))
	}
}

func TestMaxErrorsInSessionSummary(t *testing.T) {
	errors := make([]string, 10)
	for i := range errors {
		errors[i] = strings.Repeat("error ", i+1)
	}
	ss := &gen.SessionSummary{
		SessionId: "ds_1",
		Errors:    errors,
	}

	data, err := proto.Marshal(ss)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.SessionSummary{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.GetErrors()) != 10 {
		t.Errorf("errors len = %d, want 10", len(decoded.GetErrors()))
	}
}

// ── Strategy Message Specifics ─────────────────────────────────────────────────

func TestDispatchResultWithError(t *testing.T) {
	dr := &gen.DispatchResult{
		Provider: "codex",
		Error:    "timeout after 30s",
	}

	data, err := proto.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.DispatchResult{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.GetError() != "timeout after 30s" {
		t.Errorf("Error = %q", decoded.GetError())
	}
	if decoded.GetContent() != "" {
		t.Errorf("Content should be empty on error, got %q", decoded.GetContent())
	}
}

func TestReviewResultReviewedOrder(t *testing.T) {
	rr := &gen.ReviewResult{
		Reviewer: "claude",
		Reviewed: []string{"codex", "gemini", "ollama"},
		Content:  "review",
	}

	data, err := proto.Marshal(rr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.ReviewResult{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	reviewed := decoded.GetReviewed()
	expected := []string{"codex", "gemini", "ollama"}
	if len(reviewed) != len(expected) {
		t.Fatalf("reviewed len = %d, want %d", len(reviewed), len(expected))
	}
	for i, name := range reviewed {
		if name != expected[i] {
			t.Errorf("reviewed[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestSynthesisResultFallbackEmpty(t *testing.T) {
	sr := &gen.SynthesisResult{
		Chair:         "claude",
		ChairFallback: "", // no fallback needed
		Content:       "synthesized",
	}

	data, err := proto.Marshal(sr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.SynthesisResult{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.GetChairFallback() != "" {
		t.Errorf("ChairFallback = %q, want empty", decoded.GetChairFallback())
	}
}

func TestSynthesisResultFallbackPopulated(t *testing.T) {
	sr := &gen.SynthesisResult{
		Chair:         "claude",
		ChairFallback: "codex", // claude failed, codex took over
		Content:       "fallback synthesis",
	}

	data, err := proto.Marshal(sr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.SynthesisResult{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.GetChairFallback() != "codex" {
		t.Errorf("ChairFallback = %q, want codex", decoded.GetChairFallback())
	}
}

func TestSessionMetaProvidersStatusMap(t *testing.T) {
	meta := &gen.SessionMeta{
		TotalCostUsd:    0.015,
		TotalTokensIn:   1000,
		TotalTokensOut:  500,
		TotalDurationMs: 10000,
		ProvidersStatus: map[string]string{
			"claude": "healthy",
			"codex":  "error:timeout",
			"gemini": "healthy",
		},
	}

	data, err := proto.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &gen.SessionMeta{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	status := decoded.GetProvidersStatus()
	if len(status) != 3 {
		t.Fatalf("status len = %d, want 3", len(status))
	}
	if status["claude"] != "healthy" {
		t.Errorf("claude status = %q", status["claude"])
	}
	if status["codex"] != "error:timeout" {
		t.Errorf("codex status = %q", status["codex"])
	}
}
