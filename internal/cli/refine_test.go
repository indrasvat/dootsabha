package cli

import (
	"slices"
	"strings"
	"testing"
)

func TestRefineCommandRegistered(t *testing.T) {
	if !findCmd("refine") {
		t.Fatal("refine command not registered in rootCmd")
	}
}

func TestRefineCommandAliases(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "refine" {
			continue
		}
		if !slices.Contains(cmd.Aliases, "sanshodhan") {
			t.Error("refine command missing alias 'sanshodhan'")
		}
		if !slices.Contains(cmd.Aliases, "संशोधन") {
			t.Error("refine command missing alias 'संशोधन'")
		}
		return
	}
	t.Fatal("refine command not found")
}

func TestRefineCommandFlags(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "refine" {
			continue
		}
		required := []string{"author", "kartaa", "reviewers", "pareekshak", "anonymous", "gupt", "model"}
		for _, flag := range required {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("refine command missing flag --%s", flag)
			}
		}
		// Bilingual aliases should be hidden.
		for _, hidden := range []string{"kartaa", "pareekshak", "gupt"} {
			if !cmd.Flags().Lookup(hidden).Hidden {
				t.Errorf("--%s should be hidden", hidden)
			}
		}
		return
	}
	t.Fatal("refine command not found")
}

func TestRefineCommandDefaults(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "refine" {
			continue
		}
		// Check default values.
		authorFlag := cmd.Flags().Lookup("author")
		if authorFlag.DefValue != "claude" {
			t.Errorf("--author default should be 'claude', got %q", authorFlag.DefValue)
		}
		reviewersFlag := cmd.Flags().Lookup("reviewers")
		if reviewersFlag.DefValue != "codex,gemini" {
			t.Errorf("--reviewers default should be 'codex,gemini', got %q", reviewersFlag.DefValue)
		}
		anonFlag := cmd.Flags().Lookup("anonymous")
		if anonFlag.DefValue != "true" {
			t.Errorf("--anonymous default should be 'true', got %q", anonFlag.DefValue)
		}
		return
	}
	t.Fatal("refine command not found")
}

func TestParseReviewerList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"codex,gemini", []string{"codex", "gemini"}},
		{"codex", []string{"codex"}},
		{"codex, gemini, claude", []string{"codex", "gemini", "claude"}},
		{" codex , gemini ", []string{"codex", "gemini"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := parseReviewerList(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseReviewerList(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseReviewerList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestBuildReviewPromptAnonymous(t *testing.T) {
	prompt := buildReviewPrompt("test content", "claude", true)
	if !strings.Contains(prompt, "Review the following content") {
		t.Error("anonymous review prompt should start with generic prefix")
	}
	if strings.Contains(prompt, "claude") {
		t.Error("anonymous review prompt should not contain author name")
	}
	if !strings.Contains(prompt, "test content") {
		t.Error("review prompt should contain the content")
	}
}

func TestBuildReviewPromptNamed(t *testing.T) {
	prompt := buildReviewPrompt("test content", "claude", false)
	if !strings.Contains(prompt, "claude") {
		t.Error("named review prompt should contain author name")
	}
	if !strings.Contains(prompt, "test content") {
		t.Error("review prompt should contain the content")
	}
}

func TestBuildIncorporatePromptAnonymous(t *testing.T) {
	prompt := buildIncorporatePrompt("current", "feedback", "codex", true)
	if !strings.Contains(prompt, "A reviewer provided this feedback") {
		t.Error("anonymous incorporate prompt should use generic reviewer reference")
	}
	if strings.Contains(prompt, "codex") {
		t.Error("anonymous incorporate prompt should not contain reviewer name")
	}
	if !strings.Contains(prompt, "current") {
		t.Error("incorporate prompt should contain current content")
	}
	if !strings.Contains(prompt, "feedback") {
		t.Error("incorporate prompt should contain review feedback")
	}
}

func TestBuildIncorporatePromptNamed(t *testing.T) {
	prompt := buildIncorporatePrompt("current", "feedback", "codex", false)
	if !strings.Contains(prompt, "codex") {
		t.Error("named incorporate prompt should contain reviewer name")
	}
}
