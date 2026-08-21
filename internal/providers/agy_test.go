package providers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/providers"
)

// agyEnvelope builds an `agy --output-format json` document. Field names and the
// token relationship (total == in+out, thinking ⊆ out) mirror agy 1.1.17.
func agyEnvelope(t *testing.T, status, response, errMsg string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"conversation_id":  "3963cf7a-446b-479b-846f-1693117031f9",
		"status":           status,
		"response":         response,
		"error":            errMsg,
		"duration_seconds": 1.5,
		"num_turns":        1,
		"usage": map[string]int{
			"input_tokens": 1801, "output_tokens": 27, "thinking_tokens": 25,
			"cache_read_tokens": 0, "total_tokens": 1828,
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func agyOK(t *testing.T, response string) []byte {
	t.Helper()
	return agyEnvelope(t, "SUCCESS", response, "")
}

func TestAgyProviderName(t *testing.T) {
	p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{})
	if got := p.Name(); got != "agy" {
		t.Errorf("Name() = %q, want %q", got, "agy")
	}
}

func TestAgyProviderInvokeSuccess(t *testing.T) {
	runner := &mockRunner{stdout: agyOK(t, "PONG\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "PONG" {
		t.Errorf("Content = %q, want %q", result.Content, "PONG")
	}
	if result.SessionID != "3963cf7a-446b-479b-846f-1693117031f9" {
		t.Errorf("SessionID = %q, want the conversation_id", result.SessionID)
	}
	if result.TokensIn != 1801 || result.TokensOut != 27 {
		t.Errorf("tokens = (%d,%d), want (1801,27)", result.TokensIn, result.TokensOut)
	}
	// thinking_tokens is a SUBSET of output_tokens; adding it would double-count.
	if result.TokensOut == 27+25 {
		t.Error("TokensOut added thinking_tokens — they are already inside output_tokens")
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0 — agy reports no cost", result.CostUSD)
	}
	if result.Model != providers.AgyDefaultModel {
		t.Errorf("Model = %q, want %q", result.Model, providers.AgyDefaultModel)
	}
}

func TestAgyProviderInvokeMultilineContent(t *testing.T) {
	runner := &mockRunner{stdout: agyOK(t, "  line one\nline two\n\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "explain", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "line one\nline two" {
		t.Errorf("Content = %q, want interior newlines preserved", result.Content)
	}
}

func TestAgyProviderInvokeArgs(t *testing.T) {
	runner := &mockRunner{stdout: agyOK(t, "ok")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	if _, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := runner.capturedArgs

	if !slices.Contains(args, "--dangerously-skip-permissions") {
		t.Errorf("--dangerously-skip-permissions not found in args: %v", args)
	}
	if got := flagValue(args, "--model"); got != providers.AgyDefaultModel {
		t.Errorf("--model = %q, want %q: %v", got, providers.AgyDefaultModel, args)
	}
	if got := flagValue(args, "--output-format"); got != "json" {
		t.Errorf("--output-format = %q, want json: %v", got, args)
	}
	if n := countFlag(args, "--output-format"); n != 1 {
		t.Errorf("--output-format appears %d times, want 1: %v", n, args)
	}
	// agy rejects --effort for the lower tiers; the model suffix carries effort.
	if slices.Contains(args, "--effort") {
		t.Errorf("दूतसभा must not emit --effort: %v", args)
	}
	// The prompt precedes the user's flags: agy's stdlib flag parser stops at the
	// first non-flag token, so a stray token in `flags` must not be able to
	// swallow `-p <prompt>`.
	pi := slices.Index(args, "-p")
	if pi < 0 || pi+1 >= len(args) || args[pi+1] != "Say PONG" {
		t.Fatalf("expected `-p \"Say PONG\"` in args, got %v", args)
	}
	if fi := slices.Index(args, "--dangerously-skip-permissions"); fi < pi {
		t.Errorf("user flags must follow the prompt, got %v", args)
	}
}

func TestAgyProviderModelOverride(t *testing.T) {
	const override = "Gemini 3.1 Pro (High)"
	runner := &mockRunner{stdout: agyOK(t, "ok")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{Model: override})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != override {
		t.Errorf("Model = %q, want %q", result.Model, override)
	}
	if got := flagValue(runner.capturedArgs, "--model"); got != override {
		t.Errorf("--model = %q, want %q: %v", got, override, runner.capturedArgs)
	}
}

// --model and --output-format are pinned: a config entry must not duplicate or
// override them. `--output-format text` is the dangerous one — it would silently
// break every parse.
func TestAgyProviderStripsPinnedFlagsFromConfig(t *testing.T) {
	// agy parses argv with Go's stdlib flag package: `-model` IS `--model`, and a
	// repeat is LAST-WINS. Single-dash spellings therefore had to be matched too —
	// `-output-format text` broke every parse, and `-model X` silently ran a
	// different model than the one दूतसभा reported.
	for _, flags := range [][]string{
		{"--model", "legacy-model"},
		{"--model=other"},
		{"-model", "legacy-model"},
		{"-model=other"},
		{"--output-format", "text"},
		{"--output-format=stream-json"},
		{"-output-format", "text"},
		{"-output-format=text"},
		{"--model", "legacy", "-output-format", "text", "-model=x"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			cfg := defaultConfig(t)
			pc := cfg.Providers["agy"]
			pc.Flags = append(slices.Clone(flags), pc.Flags...)
			cfg.Providers["agy"] = pc

			runner := &mockRunner{stdout: agyOK(t, "ok")}
			p := providers.NewAgyProvider(cfg, runner)
			if _, err := p.Invoke(context.Background(), "Say PONG", providers.InvokeOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			args := runner.capturedArgs

			if n := countFlag(args, "--model"); n != 1 {
				t.Errorf("--model appears %d times, want 1: %v", n, args)
			}
			if got := flagValue(args, "--model"); got != providers.AgyDefaultModel {
				t.Errorf("--model = %q, want the resolved default: %v", got, args)
			}
			if got := flagValue(args, "--output-format"); got != "json" {
				t.Errorf("--output-format = %q, want json: %v", got, args)
			}
			if n := countFlag(args, "--output-format"); n != 1 {
				t.Errorf("--output-format appears %d times, want 1: %v", n, args)
			}
			// countFlag is an exact string compare, so a surviving `-output-format`
			// counts as ZERO and the assertions above pass with the hole open.
			// Normalise before checking, the way agy's own parser does.
			for i, a := range args {
				if i == 0 || i == 2 {
					continue // दूतसभा's own --model / --output-format
				}
				name, _, _ := strings.Cut(a, "=")
				switch strings.TrimLeft(name, "-") {
				case "model", "output-format":
					t.Errorf("pinned flag %q survived at index %d: %v", a, i, args)
				}
			}
		})
	}
}

// Clearing providers.agy.model is a deliberate opt-out: दूतसभा emits no --model,
// so the user's own flag must survive rather than be stripped for nothing.
func TestAgyProviderEmptyModelKeepsUserFlag(t *testing.T) {
	cfg := defaultConfig(t)
	pc := cfg.Providers["agy"]
	pc.Model = ""
	pc.Flags = []string{"--model", "Gemini 3.1 Pro (Low)"}
	cfg.Providers["agy"] = pc

	runner := &mockRunner{stdout: agyOK(t, "ok")}
	p := providers.NewAgyProvider(cfg, runner)
	if _, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := flagValue(runner.capturedArgs, "--model"); got != "Gemini 3.1 Pro (Low)" {
		t.Errorf("--model = %q, want the user's flag: %v", got, runner.capturedArgs)
	}
}

// In JSON mode agy writes the reason to STDOUT and leaves stderr EMPTY. Reading
// stderr alone loses it entirely — the regression this guards.
func TestAgyProviderInvokeErrorEnvelopeOnStdout(t *testing.T) {
	const msg = `invalid model selection (--model "nope" --effort ""): model nope is not recognized`
	runner := &mockRunner{stdout: agyEnvelope(t, "ERROR", "", msg), exitCode: 1}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not recognized") {
		t.Errorf("error %q must carry the envelope's message", err.Error())
	}
}

// A tool call failing inside a turn that still answered is exit 0 + status
// "ERROR" + a populated response. Observed live. Rejecting it discards a usable
// answer, so status must not be the discriminator.
func TestAgyProviderDegradedTurnStillSucceeds(t *testing.T) {
	runner := &mockRunner{
		stdout: agyEnvelope(t, "ERROR", "6\n", "Find command timed out.: context deadline exceeded"),
	}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	result, err := p.Invoke(context.Background(), "count funcs", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("degraded turn must not fail the invocation: %v", err)
	}
	if result.Content != "6" {
		t.Errorf("Content = %q, want the answer kept", result.Content)
	}
}

func TestAgyProviderInvokeFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runner  *mockRunner
		wantSub string
	}{
		{"non-zero exit falls back to stderr", &mockRunner{
			stdout: []byte("not json"), stderr: []byte("Error: authentication required"), exitCode: 1,
		}, "authentication required"},
		{"non-zero exit with nothing to report", &mockRunner{exitCode: 2}, "exit code 2"},
		{"exit 0 but empty stdout", &mockRunner{}, "empty output"},
		{"exit 0 but unparseable", &mockRunner{stdout: []byte("Fetching available models...")}, "json parse"},
		{"empty response with an error field", &mockRunner{
			stdout: agyEnvelope(t, "ERROR", "", "quota exhausted"),
		}, "quota exhausted"},
		{"empty response with no error field", &mockRunner{
			stdout: agyEnvelope(t, "SUCCESS", "  \n", ""),
		}, "empty response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := providers.NewAgyProvider(defaultConfig(t), tc.runner)
			_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q should contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestAgyProviderInvokeRunnerError(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hello", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "binary not found") {
		t.Errorf("error %q should contain %q", err.Error(), "binary not found")
	}
}

func TestAgyProviderInvokeTimeout(t *testing.T) {
	p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invoke

	if _, err := p.Invoke(ctx, "hello", providers.InvokeOptions{}); err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestAgyProviderHealthCheck(t *testing.T) {
	runner := &mockRunner{stdout: []byte("1.1.17\n")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Healthy {
		t.Errorf("expected Healthy=true, got error: %s", status.Error)
	}
	if status.CLIVersion != "1.1.17" {
		t.Errorf("CLIVersion = %q, want %q", status.CLIVersion, "1.1.17")
	}
	if status.Model != providers.AgyDefaultModel {
		t.Errorf("Model = %q, want %q", status.Model, providers.AgyDefaultModel)
	}
	if !status.AuthValid {
		t.Error("expected AuthValid=true")
	}
}

func TestAgyProviderHealthCheckBinaryMissing(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("binary not found: no such file or directory")}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false when binary is missing")
	}
	if status.Error == "" {
		t.Error("expected non-empty Error field")
	}
}

func TestAgyProviderHealthCheckNonZeroExit(t *testing.T) {
	runner := &mockRunner{stderr: []byte("unknown flag: --version"), exitCode: 2}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	status, err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should not return error: %v", err)
	}
	if status.Healthy {
		t.Error("expected Healthy=false on non-zero exit")
	}
}

// A type change in a field दूतसभा does NOT read must never cost the user the
// answer — nor, on a failure, the reason. Found by adversarial review: the
// envelope was decoded strictly, so `duration_seconds` or `num_turns` drifting
// would have reinstated the exact "bare exit code 1" bug 707 exists to fix.
func TestAgyProviderToleratesUnreadFieldDrift(t *testing.T) {
	const answer = "GOOD ANSWER"
	for _, body := range []string{
		`{"status":"SUCCESS","response":"` + answer + `","num_turns":1.5,"usage":{"input_tokens":18053.0,"output_tokens":26}}`,
		`{"status":"SUCCESS","response":"` + answer + `","duration_seconds":"3.5","usage":{}}`,
		`{"status":"SUCCESS","response":"` + answer + `","thinking_tokens":null,"usage":{"input_tokens":1,"output_tokens":2}}`,
	} {
		t.Run(body[:40], func(t *testing.T) {
			p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{stdout: []byte(body)})
			result, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
			if err != nil {
				t.Fatalf("drift in an unread field discarded the answer: %v", err)
			}
			if result.Content != answer {
				t.Errorf("Content = %q, want %q", result.Content, answer)
			}
		})
	}

	// A float token count degrades to its integer value, not to a parse failure.
	p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{
		stdout: []byte(`{"status":"SUCCESS","response":"x","usage":{"input_tokens":18053.0,"output_tokens":26}}`),
	})
	result, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TokensIn != 18053 || result.TokensOut != 26 {
		t.Errorf("tokens = (%d,%d), want (18053,26)", result.TokensIn, result.TokensOut)
	}

	// An absurd count degrades to 0 rather than overflowing the int32 the gRPC
	// plugin narrows to.
	p = providers.NewAgyProvider(defaultConfig(t), &mockRunner{
		stdout: []byte(`{"status":"SUCCESS","response":"x","usage":{"input_tokens":99999999999999,"output_tokens":-5}}`),
	})
	result, err = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TokensIn != 0 || result.TokensOut != 0 {
		t.Errorf("tokens = (%d,%d), want (0,0) for out-of-range counts", result.TokensIn, result.TokensOut)
	}
}

// The failure message is decoded independently of the full envelope, so a drift
// elsewhere cannot strand the user on a bare exit code — stderr is EMPTY in JSON
// mode, so there is no second chance.
func TestAgyProviderErrorSurvivesEnvelopeDrift(t *testing.T) {
	const msg = "Quota exceeded for Gemini 3.7 Flash. Retry after 3600s."
	runner := &mockRunner{
		stdout:   []byte(`{"status":"ERROR","response":"","error":"` + msg + `","num_turns":1.5,"usage":{}}`),
		exitCode: 1,
	}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), msg) {
		t.Errorf("error %q lost the envelope message", err.Error())
	}
}

// agy's most common failure lists every valid model (~750 bytes). Cutting that
// removes the only actionable content, so the cap must clear it.
func TestAgyProviderErrorCapClearsTheModelList(t *testing.T) {
	long := "model nope is not recognized\nAvailable models:\n" + strings.Repeat("  Gemini 3.7 Flash (High)\n", 30)
	runner := &mockRunner{
		stdout:   []byte(`{"status":"ERROR","response":"","error":` + strconv.Quote(long) + `,"usage":{}}`),
		exitCode: 1,
	}
	p := providers.NewAgyProvider(defaultConfig(t), runner)

	_, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "[truncated]") {
		t.Errorf("a ~750-byte model list must survive the cap, got: %q", err.Error())
	}

	// Still capped, so a hostile binary cannot flood the terminal.
	runner = &mockRunner{
		stdout:   []byte(`{"status":"ERROR","response":"","error":` + strconv.Quote(strings.Repeat("x", 100_000)) + `,"usage":{}}`),
		exitCode: 1,
	}
	p = providers.NewAgyProvider(defaultConfig(t), runner)
	_, err = p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "[truncated]") {
		t.Errorf("a 100KB error must still be capped, got %v", err)
	}
}

// stdout is exactly ONE document. Decode stops at the first value, so without an
// explicit check a second envelope is dropped and the wrong turn becomes the answer.
func TestAgyProviderRejectsTrailingDocument(t *testing.T) {
	for _, tc := range []struct {
		name, stdout string
		wantErr      bool
	}{
		{"two envelopes", `{"status":"SUCCESS","response":"FIRST"}{"status":"ERROR","response":"SECOND","error":"real failure"}`, true},
		{"trailing garbage", `{"status":"SUCCESS","response":"ok"} some banner`, true},
		// Decoder.More() returns FALSE for these — it asks "another element in the
		// current array/object?", not "is there trailing data?". A `}` or `]` both
		// slips through AND masks a second document behind it. Found by दूतसभा's
		// own review of this branch.
		{"stray closing brace", `{"status":"SUCCESS","response":"ok"}}`, true},
		{"stray closing bracket", `{"status":"SUCCESS","response":"ok"}]`, true},
		{"second document behind a brace", `{"status":"SUCCESS","response":"FIRST"}}{"status":"ERROR","response":"SECOND"}`, true},
		{"second document behind a bracket", `{"status":"SUCCESS","response":"FIRST"}]{"status":"ERROR","response":"SECOND"}`, true},
		{"trailing newline is fine", `{"status":"SUCCESS","response":"ok"}` + "\n", false},
		{"trailing whitespace is fine", `{"status":"SUCCESS","response":"ok"}  ` + "\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{stdout: []byte(tc.stdout)})
			_, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{})
			if tc.wantErr && err == nil {
				t.Error("trailing data was silently ignored — the second turn would be lost")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Stripping a pinned flag must not eat a following FLAG — only its value.
// grok's stripper guards this; the agy copy had regressed it, silently dropping
// the auto-approve flag so agy would block on tool permission prompts.
func TestAgyProviderPinnedFlagDoesNotEatNextFlag(t *testing.T) {
	cfg := defaultConfig(t)
	pc := cfg.Providers["agy"]
	pc.Flags = []string{"--output-format", "--dangerously-skip-permissions", "--effort", "high"}
	cfg.Providers["agy"] = pc

	runner := &mockRunner{stdout: agyOK(t, "ok")}
	p := providers.NewAgyProvider(cfg, runner)
	if _, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Contains(runner.capturedArgs, "--dangerously-skip-permissions") {
		t.Errorf("a valueless pinned flag ate the next flag: %v", runner.capturedArgs)
	}
	if got := flagValue(runner.capturedArgs, "--effort"); got != "high" {
		t.Errorf("--effort = %q, want high: %v", got, runner.capturedArgs)
	}
}

// The prompt is दूतसभा's. agy parses argv with the stdlib flag package, so a
// config copy of -p/--print/--prompt is LAST-WINS and would replace the prompt
// outright — the agent would answer a question the caller never asked.
// Found by दूतसभा's own review of this branch.
func TestAgyProviderPromptCannotBeHijackedFromConfig(t *testing.T) {
	for _, spelling := range []string{"-p", "--p", "--print", "--prompt", "-prompt"} {
		t.Run(spelling, func(t *testing.T) {
			cfg := defaultConfig(t)
			pc := cfg.Providers["agy"]
			pc.Flags = append([]string{spelling, "HIJACKED"}, pc.Flags...)
			cfg.Providers["agy"] = pc

			runner := &mockRunner{stdout: agyOK(t, "ok")}
			p := providers.NewAgyProvider(cfg, runner)
			if _, err := p.Invoke(context.Background(), "REAL PROMPT", providers.InvokeOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if slices.Contains(runner.capturedArgs, "HIJACKED") {
				t.Errorf("config smuggled a prompt past दूतसभा's: %v", runner.capturedArgs)
			}
			if n := countFlag(runner.capturedArgs, "-p"); n != 1 {
				t.Errorf("-p appears %d times, want 1: %v", n, runner.capturedArgs)
			}
		})
	}
}

// दूतसभा always sends --output-format json, which agy print mode only accepts
// from 1.1.8. On an older build every call fails before the prompt runs, so
// `status` must not report it healthy. Raised by Codex on PR #27.
func TestAgyProviderHealthCheckRejectsPreJSONVersions(t *testing.T) {
	for _, tc := range []struct {
		version string
		healthy bool
	}{
		{"1.0.8", false}, // the version 703 shipped against — no --output-format
		{"1.1.7", false}, // last release before the flag
		{"1.1.8", true},  // the release that added it
		{"1.1.17", true}, // verified working
		{"2.0.0", true},  // future major
		{"1.1.8-beta", true},
		{"devel", true}, // unparseable must NOT be declared broken
	} {
		t.Run(tc.version, func(t *testing.T) {
			p := providers.NewAgyProvider(defaultConfig(t), &mockRunner{stdout: []byte(tc.version + "\n")})
			status, err := p.HealthCheck(context.Background())
			if err != nil {
				t.Fatalf("HealthCheck should not return error: %v", err)
			}
			if status.Healthy != tc.healthy {
				t.Errorf("Healthy = %v, want %v (error: %s)", status.Healthy, tc.healthy, status.Error)
			}
			if status.CLIVersion != tc.version {
				t.Errorf("CLIVersion = %q, want %q", status.CLIVersion, tc.version)
			}
			if !tc.healthy && !strings.Contains(status.Error, "agy update") {
				t.Errorf("error %q must tell the user how to fix it", status.Error)
			}
		})
	}
}

// agy's own --print-timeout defaults to 5m, exactly दूतसभा's default --timeout.
// When it fires, agy reports a plain ERROR envelope with exit 1 — so a timeout
// arrives looking like any other provider failure and the caller gets exit 3
// ("try another agent") instead of exit 4 ("raise the timeout"). दूतसभा forwards
// its own step window plus a margin so its timer always fires first. Task 708.
func TestAgyProviderForwardsStepBudgetAsPrintTimeout(t *testing.T) {
	for _, tc := range []struct {
		name   string
		window time.Duration
	}{
		{"short window", 30 * time.Second},
		{"past agy's own 5m default", 15 * time.Minute},
		{"session-sized window", 30 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.window)
			defer cancel()

			runner := &mockRunner{stdout: agyOK(t, "ok")}
			p := providers.NewAgyProvider(defaultConfig(t), runner)
			if _, err := p.Invoke(ctx, "hi", providers.InvokeOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			raw := flagValue(runner.capturedArgs, "--print-timeout")
			if raw == "" {
				t.Fatalf("--print-timeout not forwarded: %v", runner.capturedArgs)
			}
			got, err := time.ParseDuration(raw)
			if err != nil {
				t.Fatalf("--print-timeout %q is not a duration agy can parse: %v", raw, err)
			}
			// Must EXCEED the step window, or the two timers race and agy may win.
			if got <= tc.window {
				t.Errorf("--print-timeout %s does not exceed the %s step window", got, tc.window)
			}
			// ...but not by so much that a hung call outlives the pipeline.
			if got > tc.window+2*time.Minute {
				t.Errorf("--print-timeout %s overshoots the %s window", got, tc.window)
			}
		})
	}
}

// `--print-timeout 0` means ZERO, not "disabled" — verified on 1.1.17, it fails
// instantly with "timeout waiting for response". So an unbounded step must emit
// no flag at all rather than a zero, and the user's own flag stays in control.
func TestAgyProviderUnboundedStepEmitsNoPrintTimeout(t *testing.T) {
	t.Run("no deadline", func(t *testing.T) {
		runner := &mockRunner{stdout: agyOK(t, "ok")}
		p := providers.NewAgyProvider(defaultConfig(t), runner)
		if _, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if slices.Contains(runner.capturedArgs, "--print-timeout") {
			t.Errorf("unbounded step must not pin a timeout: %v", runner.capturedArgs)
		}
	})

	// Unbounded is the one case where a user's own --print-timeout survives.
	t.Run("user flag survives when unbounded", func(t *testing.T) {
		cfg := defaultConfig(t)
		pc := cfg.Providers["agy"]
		pc.Flags = append([]string{"--print-timeout", "45m"}, pc.Flags...)
		cfg.Providers["agy"] = pc

		runner := &mockRunner{stdout: agyOK(t, "ok")}
		p := providers.NewAgyProvider(cfg, runner)
		if _, err := p.Invoke(context.Background(), "hi", providers.InvokeOptions{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := flagValue(runner.capturedArgs, "--print-timeout"); got != "45m" {
			t.Errorf("--print-timeout = %q, want the user's 45m: %v", got, runner.capturedArgs)
		}
	})

	// A bounded step pins it, in any spelling agy accepts.
	for _, spelling := range []string{"--print-timeout", "-print-timeout", "--print-timeout=1ms"} {
		t.Run("bounded step pins "+spelling, func(t *testing.T) {
			cfg := defaultConfig(t)
			pc := cfg.Providers["agy"]
			pc.Flags = append([]string{spelling, "1ms"}, pc.Flags...)
			cfg.Providers["agy"] = pc

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			runner := &mockRunner{stdout: agyOK(t, "ok")}
			p := providers.NewAgyProvider(cfg, runner)
			if _, err := p.Invoke(ctx, "hi", providers.InvokeOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n := countFlag(runner.capturedArgs, "--print-timeout"); n != 1 {
				t.Errorf("--print-timeout appears %d times, want 1: %v", n, runner.capturedArgs)
			}
			if got := flagValue(runner.capturedArgs, "--print-timeout"); got == "1ms" {
				t.Errorf("config overrode दूतसभा's window: %v", runner.capturedArgs)
			}
		})
	}
}
