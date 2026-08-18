package providers

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// grokIsolatedEnv is the R1 harness-inheritance fix: grok discovers ~/.claude/**
// off $HOME, so an empty HOME severs skills/agents/plugins/MCP/hooks/CLAUDE.md
// while GROK_HOME keeps auth working.
func TestGrokIsolatedEnvRedirectsHome(t *testing.T) {
	base := []string{"HOME=/Users/someone", "PATH=/usr/bin", "LANG=en_US.UTF-8"}

	env, err := grokIsolatedEnv(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home := envLookup(env, "HOME")
	if home == "" || home == "/Users/someone" {
		t.Errorf("HOME = %q, want an isolated temp dir", home)
	}
	if entries, err := os.ReadDir(home); err != nil {
		t.Errorf("isolated HOME unreadable: %v", err)
	} else if len(entries) != 0 {
		t.Errorf("isolated HOME must be empty, has %d entries", len(entries))
	}
}

// The real grok home must be resolved from the ORIGINAL HOME, before it is
// overridden — getting this order wrong points GROK_HOME at the empty dir and
// silently breaks auth.
func TestGrokIsolatedEnvDerivesGrokHomeFromOriginalHome(t *testing.T) {
	base := []string{"HOME=/Users/someone", "PATH=/usr/bin"}

	env, err := grokIsolatedEnv(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := envLookup(env, "GROK_HOME"); got != "/Users/someone/.grok" {
		t.Errorf("GROK_HOME = %q, want /Users/someone/.grok", got)
	}
}

// An explicit GROK_HOME must be honoured rather than recomputed.
func TestGrokIsolatedEnvHonoursExplicitGrokHome(t *testing.T) {
	base := []string{"HOME=/Users/someone", "GROK_HOME=/custom/grok"}

	env, err := grokIsolatedEnv(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := envLookup(env, "GROK_HOME"); got != "/custom/grok" {
		t.Errorf("GROK_HOME = %q, want /custom/grok", got)
	}
}

func TestGrokIsolatedEnvSingleHomeEntryAndPreservesOthers(t *testing.T) {
	base := []string{"HOME=/Users/someone", "PATH=/usr/bin", "LANG=en_US.UTF-8", "GROK_HOME=/old"}

	env, err := grokIsolatedEnv(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	homes, grokHomes := 0, 0
	for _, kv := range env {
		if strings.HasPrefix(kv, "HOME=") {
			homes++
		}
		if strings.HasPrefix(kv, "GROK_HOME=") {
			grokHomes++
		}
	}
	if homes != 1 {
		t.Errorf("HOME appears %d times, want exactly 1", homes)
	}
	if grokHomes != 1 {
		t.Errorf("GROK_HOME appears %d times, want exactly 1", grokHomes)
	}
	// Unrelated vars survive untouched.
	for _, want := range []string{"PATH=/usr/bin", "LANG=en_US.UTF-8"} {
		if !slices.Contains(env, want) {
			t.Errorf("%q was dropped from env", want)
		}
	}
}

// The isolated HOME is reused process-wide rather than created per invocation.
func TestGrokIsolatedEnvIsStableAcrossCalls(t *testing.T) {
	a, err := grokIsolatedEnv([]string{"HOME=/x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := grokIsolatedEnv([]string{"HOME=/x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envLookup(a, "HOME") != envLookup(b, "HOME") {
		t.Error("isolated HOME should be stable across calls")
	}
}

func TestGrokEmptyHomeIs0700(t *testing.T) {
	dir, err := grokEmptyHome()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("isolated HOME perm = %o, want 700", perm)
	}
}

func TestStripPinnedFlags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"removes valued flag and its value", []string{"--sandbox", "off", "--keep"}, []string{"--keep"}},
		{"removes equals form", []string{"--sandbox=off", "--keep"}, []string{"--keep"}},
		{"removes bool flag", []string{"--verbatim", "--keep"}, []string{"--keep"}},
		{"removes short model flag", []string{"-m", "x", "--keep"}, []string{"--keep"}},
		{"keeps unknown flags", []string{"--keep", "--also-keep"}, []string{"--keep", "--also-keep"}},
		{"trailing valued flag without value", []string{"--keep", "--sandbox"}, []string{"--keep"}},
		{"empty", nil, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripPinnedFlags(tc.in)
			if !slices.Equal(got, tc.want) {
				t.Errorf("stripPinnedFlags(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtractGrokEffort(t *testing.T) {
	tests := []struct {
		name       string
		in         []string
		wantEffort string
		wantRest   []string
	}{
		{"defaults to high", []string{"--keep"}, "high", []string{"--keep"}},
		{"reads --reasoning-effort", []string{"--reasoning-effort", "low", "--keep"}, "low", []string{"--keep"}},
		{"reads --effort alias", []string{"--effort", "medium"}, "medium", []string{}},
		{"reads equals form", []string{"--reasoning-effort=low"}, "low", []string{}},
		{"reads xhigh (new in grok-4.6)", []string{"--reasoning-effort", "xhigh"}, "xhigh", []string{}},
		{"reads xhigh equals form", []string{"--effort=xhigh"}, "xhigh", []string{}},
		// An empty value must fall back, never forward `--reasoning-effort ""`,
		// which the CLI rejects outright.
		{"empty equals value falls back", []string{"--reasoning-effort="}, "high", []string{}},
		{"empty equals value on alias falls back", []string{"--effort=", "--keep"}, "high", []string{"--keep"}},
		// The equals form used to accept a flag-shaped value, so a pinned flag
		// could ride in as the "effort". Both forms now refuse it.
		{"equals form rejects a flag-shaped value", []string{"--reasoning-effort=-x"}, "high", []string{}},
		{"equals form rejects a smuggled pinned flag", []string{"--reasoning-effort=--sandbox=danger-full-access"}, "high", []string{}},
		// Padding must be trimmed, not forwarded: grok rejects " xhigh ".
		{"padded value is trimmed", []string{"--reasoning-effort= xhigh "}, "xhigh", []string{}},
		{"padded space-form value is trimmed", []string{"--reasoning-effort", "  low  "}, "low", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			effort, rest := extractGrokEffort(tc.in)
			if effort != tc.wantEffort {
				t.Errorf("effort = %q, want %q", effort, tc.wantEffort)
			}
			if !slices.Equal(rest, tc.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tc.wantRest)
			}
		})
	}
}

// A value-taking flag must not swallow the NEXT FLAG as its value. Config like
// ["--sandbox", "--keep-me"] previously dropped --keep-me entirely, and
// ["--effort", "--keep-me"] set effort to "--keep-me".
// Found by grok reviewing this provider through dootsabha.
// grok is clap-based, so `-mgrok-9` parses as `-m grok-9` — the attached short
// form is real, and matchPinned's "=" split alone did not see it. The stray flag
// then reached grok and broke the whole invocation ("the argument
// '--model <MODEL>' cannot be used multiple times"). Found by an adversarial
// agent driving the real binary.
func TestStripPinnedFlagsHandlesAttachedShortOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"attached -m", []string{"-mgrok-9", "--keep"}, []string{"--keep"}},
		{"attached -p", []string{"-pPWN", "--keep"}, []string{"--keep"}},
		{"bare -m still consumes its value", []string{"-m", "grok-9", "--keep"}, []string{"--keep"}},
		{"lone dash is a value, not a flag", []string{"-", "--keep"}, []string{"-", "--keep"}},
		{"unrelated short flag survives", []string{"-c", "--keep"}, []string{"-c", "--keep"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripPinnedFlags(tc.in); !slices.Equal(got, tc.want) {
				t.Errorf("stripPinnedFlags(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestStripPinnedFlagsDoesNotSwallowFollowingFlag(t *testing.T) {
	got := stripPinnedFlags([]string{"--sandbox", "--keep-me"})
	if !slices.Equal(got, []string{"--keep-me"}) {
		t.Errorf("stripPinnedFlags = %v, want [--keep-me] — a flag must not be eaten as a value", got)
	}
}

func TestExtractGrokEffortDoesNotSwallowFollowingFlag(t *testing.T) {
	effort, rest := extractGrokEffort([]string{"--effort", "--keep-me"})
	if strings.HasPrefix(effort, "-") {
		t.Errorf("effort = %q — a flag must never become the effort value", effort)
	}
	if effort != grokDefaultEffort {
		t.Errorf("effort = %q, want the default %q when no real value follows", effort, grokDefaultEffort)
	}
	if !slices.Contains(rest, "--keep-me") {
		t.Errorf("rest = %v, want --keep-me preserved", rest)
	}
}

// --- parseGrokNDJSON, directly ---------------------------------------------
//
// Exercised indirectly through Invoke elsewhere; these pin the parser itself so
// a regression names the parser rather than surfacing as a confusing Invoke error.

func TestParseGrokNDJSONLastResultWins(t *testing.T) {
	stream := []byte(
		`{"type":"system","subtype":"init"}` + "\n" +
			`{"type":"result","subtype":"success","result":"first"}` + "\n" +
			`{"type":"assistant","message":{"content":[]}}` + "\n" +
			`{"type":"result","subtype":"success","result":"second"}` + "\n")

	got := parseGrokNDJSON(stream)
	if got == nil {
		t.Fatal("expected a result event")
	}
	if got.Result != "second" {
		t.Errorf("Result = %q, want the LAST result line", got.Result)
	}
}

func TestParseGrokNDJSONSkipsMalformed(t *testing.T) {
	stream := []byte("not json\n{\"broken\":\n\n{\"type\":\"result\",\"result\":\"ok\"}\n")
	got := parseGrokNDJSON(stream)
	if got == nil || got.Result != "ok" {
		t.Errorf("malformed lines must be skipped, got %+v", got)
	}
}

func TestParseGrokNDJSONNoResultEvent(t *testing.T) {
	stream := []byte(`{"type":"system"}` + "\n" + `{"type":"assistant"}` + "\n")
	if got := parseGrokNDJSON(stream); got != nil {
		t.Errorf("no result event must yield nil, got %+v", got)
	}
}

func TestParseGrokNDJSONEmpty(t *testing.T) {
	if got := parseGrokNDJSON(nil); got != nil {
		t.Errorf("empty input must yield nil, got %+v", got)
	}
}

// An error result is still type=="result"; the discriminator is is_error.
func TestParseGrokNDJSONErrorResult(t *testing.T) {
	stream := []byte(`{"type":"result","subtype":"error_during_execution","is_error":true,` +
		`"errors":["quota exceeded"],"session_id":"x"}` + "\n")

	got := parseGrokNDJSON(stream)
	if got == nil {
		t.Fatal("an error result must still parse — it carries the reason")
	}
	if !got.IsError {
		t.Error("IsError = false, want true")
	}
	if len(got.Errors) != 1 || got.Errors[0] != "quota exceeded" {
		t.Errorf("Errors = %v, want the upstream message", got.Errors)
	}
	if got.Result != "" {
		t.Errorf("Result = %q, want empty — error events carry no result field", got.Result)
	}
}

// Real review payloads exceed bufio.Scanner's 64KB token limit; the parser uses
// bytes.Split precisely to avoid that.
func TestParseGrokNDJSONHugeLine(t *testing.T) {
	big := strings.Repeat("x", 200_000)
	stream := []byte(`{"type":"result","result":"` + big + `"}` + "\n")

	got := parseGrokNDJSON(stream)
	if got == nil || len(got.Result) != len(big) {
		t.Errorf("200KB line must parse intact, got len=%d", len(got.Result))
	}
}
