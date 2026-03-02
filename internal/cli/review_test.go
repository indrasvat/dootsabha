package cli

import (
	"slices"
	"testing"
)

func TestReviewCommandRegistered(t *testing.T) {
	if !findCmd("review") {
		t.Fatal("review command not registered in rootCmd")
	}
}

func TestReviewCommandAliases(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "review" {
			continue
		}
		if !slices.Contains(cmd.Aliases, "sameeksha") {
			t.Error("review command missing alias 'sameeksha'")
		}
		return
	}
	t.Fatal("review command not found")
}

func TestReviewCommandFlags(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "review" {
			continue
		}
		required := []string{"author", "kartaa", "reviewer", "pareekshak", "model"}
		for _, flag := range required {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("review command missing flag --%s", flag)
			}
		}
		// kartaa and pareekshak should be hidden
		if !cmd.Flags().Lookup("kartaa").Hidden {
			t.Error("--kartaa should be hidden")
		}
		if !cmd.Flags().Lookup("pareekshak").Hidden {
			t.Error("--pareekshak should be hidden")
		}
		return
	}
	t.Fatal("review command not found")
}
