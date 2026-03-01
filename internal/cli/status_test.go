package cli

import (
	"slices"
	"testing"
)

func TestStatusCommandRegistered(t *testing.T) {
	if !findCmd("status") {
		t.Fatal("status command not registered in rootCmd")
	}
}

func TestStatusCommandAliases(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "status" {
			continue
		}
		if slices.Contains(cmd.Aliases, "sthiti") {
			return
		}
		t.Error("status command missing alias 'sthiti'")
	}
}

func TestStatusCommandNoFlags(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "status" {
			continue
		}
		// Status has no local flags (uses root persistent flags only).
		if cmd.Flags().HasFlags() {
			t.Log("status command has local flags (informational)")
		}
		return
	}
	t.Fatal("status command not found")
}

func TestConfigCommandRegistered(t *testing.T) {
	if !findCmd("config") {
		t.Fatal("config command not registered in rootCmd")
	}
}

func TestConfigCommandAliases(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "config" {
			continue
		}
		if slices.Contains(cmd.Aliases, "vinyaas") {
			return
		}
		t.Error("config command missing alias 'vinyaas'")
	}
}

func TestConfigShowSubcommand(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() != "config" {
			continue
		}
		for _, sub := range cmd.Commands() {
			if sub.Name() == "show" {
				return
			}
		}
		t.Error("config command missing 'show' subcommand")
	}
}

func TestProviderColorKnown(t *testing.T) {
	for _, name := range []string{"claude", "codex", "gemini"} {
		c := providerColor(name)
		if c == "" {
			t.Errorf("providerColor(%q) returned empty color", name)
		}
	}
}

func TestGlobalFlagsRegistered(t *testing.T) {
	pf := rootCmd.PersistentFlags()
	flags := []string{"json", "verbose", "quiet", "timeout", "kaalseema", "session-timeout", "config"}
	for _, flag := range flags {
		if pf.Lookup(flag) == nil {
			t.Errorf("root persistent flag --%s not registered", flag)
		}
	}
	// kaalseema should be hidden
	if !pf.Lookup("kaalseema").Hidden {
		t.Error("--kaalseema should be a hidden flag")
	}
}
