package cli

import (
	"testing"
)

func TestUsageCmd_HasProviderSubcommands(t *testing.T) {
	providers := []string{"claude", "codex", "copilot", "cursor", "gemini", "openrouter", "warp", "kimicode", "amp"}
	for _, name := range providers {
		found := false
		for _, cmd := range usageCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("usageCmd missing provider subcommand %q", name)
		}
	}

	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "kimi" {
			t.Error("usageCmd should not expose legacy provider subcommand \"kimi\"")
		}
	}
}
