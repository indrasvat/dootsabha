package cli

import (
	"slices"
	"testing"
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
