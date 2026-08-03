package cli

import (
	"os"
	"slices"
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

func TestCouncilCommandRegistered(t *testing.T) {
	if !findCmd("council") {
		t.Fatal("council command not registered in rootCmd")
	}
}

func TestCouncilCommandAliases(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "council" {
			continue
		}
		if !slices.Contains(cmd.Aliases, "sabha") {
			t.Error("council command missing alias 'sabha'")
		}
		if !slices.Contains(cmd.Aliases, "सभा") {
			t.Error("council command missing alias 'सभा'")
		}
		return
	}
	t.Fatal("council command not found")
}

func TestCouncilCommandFlags(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "council" {
			continue
		}
		required := []string{"agents", "dootas", "chair", "adhyaksha", "rounds", "chakra", "parallel", "samantar"}
		for _, flag := range required {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("council command missing flag --%s", flag)
			}
		}
		// Bilingual aliases should be hidden.
		hidden := []string{"dootas", "adhyaksha", "chakra", "samantar"}
		for _, flag := range hidden {
			f := cmd.Flags().Lookup(flag)
			if f == nil {
				continue // already reported above
			}
			if !f.Hidden {
				t.Errorf("--%s should be hidden", flag)
			}
		}
		return
	}
	t.Fatal("council command not found")
}

// TestCouncilStepsCountsWallClockNotInvocations.
//
// The session ceiling bounds elapsed time, so the step count that feeds the
// "this pipeline may be cut short" warning must count the longest CHAIN of
// calls, not the total number of calls. Dispatch and peer review run their
// agents concurrently by default — counting invocations made every council run
// on the shipped defaults (3 agents, 5m timeout, 30m ceiling) warn that it
// might be cut short when 15m of the 30m is all it can possibly use.
func TestCouncilStepsCountsWallClockNotInvocations(t *testing.T) {
	tests := []struct {
		name     string
		parallel bool
		rounds   int
		agents   []string
		want     int
	}{
		{"parallel: three stages regardless of agent count", true, 1, []string{"claude", "codex", "agy"}, 3},
		{"parallel: ten agents are still three stages", true, 1, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, 3},
		{"parallel: rounds multiply the stages", true, 3, []string{"claude", "codex"}, 9},
		{"sequential: every agent is its own link", false, 1, []string{"claude", "codex", "agy"}, 7},
		{"sequential: rounds multiply", false, 2, []string{"claude", "codex"}, 10},
		{"sequential: single agent", false, 1, []string{"claude"}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &core.Config{Council: core.CouncilConfig{Parallel: tt.parallel, Rounds: tt.rounds}}
			if got := councilSteps(cfg, tt.agents); got != tt.want {
				t.Errorf("councilSteps = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestShippedDefaultsDoNotWarn — the budget warning must be a signal, not
// background noise. Every default pipeline has to fit its default ceiling.
func TestShippedDefaultsDoNotWarn(t *testing.T) {
	cfg, err := core.LoadConfig(os.DevNull)
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	perInvoke, session := resolveTimeouts(0, 0, false, cfg)

	cases := map[string]int{
		"review (author + reviewer)":       2,
		"refine (v1 + 2 reviewers)":        5,
		"council (3 agents, parallel)":     councilSteps(cfg, []string{"claude", "codex", "agy"}),
		"council (2 agents inside Claude)": councilSteps(cfg, []string{"codex", "agy"}),
	}
	for name, steps := range cases {
		if w := budgetWarning(perInvoke, session, steps); w != "" {
			t.Errorf("%s warns on the shipped defaults (%s x %d vs %s): %s",
				name, perInvoke, steps, session, w)
		}
	}
}
