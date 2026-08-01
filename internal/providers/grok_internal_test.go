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
