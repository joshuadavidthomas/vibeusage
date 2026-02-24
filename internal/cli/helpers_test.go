package cli

import "github.com/joshuadavidthomas/vibeusage/internal/config"

// reloadConfig forces a config reload. Used by tests that modify
// VIBEUSAGE_CONFIG_DIR via t.Setenv before exercising commands.
func reloadConfig() {
	_, _ = config.Reload()
}
