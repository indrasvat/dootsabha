package cli

import (
	"slices"
	"testing"
)

func findCmd(name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return true
		}
	}
	return false
}

func TestConsultCommandRegistered(t *testing.T) {
	if !findCmd("consult") {
		t.Fatal("consult command not registered in rootCmd")
	}
}

func TestConsultCommandAliases(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "consult" {
			continue
		}
		if slices.Contains(cmd.Aliases, "paraamarsh") {
			return
		}
		t.Error("consult command missing alias 'paraamarsh'")
	}
}

func TestConsultCommandFlags(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "consult" {
			continue
		}
		required := []string{"agent", "doota", "model", "pratyaya", "max-turns"}
		for _, flag := range required {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("consult command missing flag --%s", flag)
			}
		}
		// doota and pratyaya should be hidden
		if !cmd.Flags().Lookup("doota").Hidden {
			t.Error("--doota should be hidden")
		}
		if !cmd.Flags().Lookup("pratyaya").Hidden {
			t.Error("--pratyaya should be hidden")
		}
		return
	}
	t.Fatal("consult command not found")
}

func TestExitErrorImplementsError(t *testing.T) {
	e := &ExitError{Code: 3, Message: "provider failed"}
	if e.Error() != "provider failed" {
		t.Errorf("ExitError.Error() = %q, want %q", e.Error(), "provider failed")
	}
	if e.Code != 3 {
		t.Errorf("ExitError.Code = %d, want 3", e.Code)
	}
}

func TestGetProviderKnownNames(t *testing.T) {
	for _, name := range []string{"claude", "codex", "gemini"} {
		_, err := getProvider(name, nil, nil)
		if err != nil {
			t.Errorf("getProvider(%q) returned unexpected error: %v", name, err)
		}
	}
}

func TestGetProviderUnknown(t *testing.T) {
	_, err := getProvider("unknown-xyz", nil, nil)
	if err == nil {
		t.Error("getProvider with unknown name should return error")
	}
}
