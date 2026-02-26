package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()
		cfgPath := config.ConfigFile()

		if jsonOutput {
			return display.OutputJSON(outWriter, display.ConfigShowJSON{
				Fetch: display.ConfigFetchJSON{
					Timeout:               cfg.Fetch.Timeout,
					StaleThresholdMinutes: cfg.Fetch.StaleThresholdMinutes,
					MaxConcurrent:         cfg.Fetch.MaxConcurrent,
				},
				EnabledProviders: cfg.EnabledProviders,
				Display: display.ConfigDisplayJSON{
					ShowRemaining: cfg.Display.ShowRemaining,
					PaceColors:    cfg.Display.PaceColors,
					ResetFormat:   cfg.Display.ResetFormat,
				},
				Credentials: display.ConfigCredentialsJSON{
					UseKeyring: cfg.Credentials.UseKeyring,
				},
				Roles: cfg.Roles,
				Path:  cfgPath,
			})
		}

		if quiet {
			outln(cfgPath)
			return nil
		}

		out("Config: %s\n\n", cfgPath)
		_ = toml.NewEncoder(outWriter).Encode(cfg)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show directory paths",
	RunE: func(cmd *cobra.Command, args []string) error {
		showCache, _ := cmd.Flags().GetBool("cache")
		showCreds, _ := cmd.Flags().GetBool("credentials")

		if jsonOutput {
			if showCache {
				return display.OutputJSON(outWriter, map[string]string{"cache_dir": config.CacheDir()})
			} else if showCreds {
				return display.OutputJSON(outWriter, map[string]string{"credentials_dir": config.CredentialsDir()})
			}
			return display.OutputJSON(outWriter, map[string]string{
				"config_dir":      config.ConfigDir(),
				"config_file":     config.ConfigFile(),
				"cache_dir":       config.CacheDir(),
				"credentials_dir": config.CredentialsDir(),
			})
		}

		if quiet {
			if showCache {
				outln(config.CacheDir())
			} else if showCreds {
				outln(config.CredentialsDir())
			} else {
				outln(config.ConfigDir())
			}
			return nil
		}

		if showCache {
			outln(config.CacheDir())
		} else if showCreds {
			outln(config.CredentialsDir())
		} else {
			out("Config dir:    %s\n", config.ConfigDir())
			out("Config file:   %s\n", config.ConfigFile())
			out("Cache dir:     %s\n", config.CacheDir())
			out("Credentials:   %s\n", config.CredentialsDir())
		}
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm && !jsonOutput {
			ok, err := prompt.Default.Confirm(prompt.ConfirmConfig{
				Title: "Reset configuration to defaults?",
			})
			if err != nil {
				return err
			}
			if !ok {
				outln("Reset cancelled")
				return nil
			}
		}

		cfgPath := config.ConfigFile()
		if err := os.Remove(cfgPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("resetting config: %w", err)
		}

		if jsonOutput {
			return display.OutputJSON(outWriter, display.ActionResultJSON{
				Success: true,
				Reset:   true,
				Message: "Configuration reset to defaults",
			})
		}

		outln("✓ Configuration reset to defaults")
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open configuration in editor",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := config.ConfigFile()

		_ = os.MkdirAll(config.ConfigDir(), 0o755)
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			cfg := config.DefaultConfig()
			_ = config.Save(cfg, cfgPath)
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		c := exec.Command(editor, cfgPath)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	configPathCmd.Flags().BoolP("cache", "c", false, "Show cache directory")
	configPathCmd.Flags().Bool("credentials", false, "Show credentials directory")
	configResetCmd.Flags().BoolP("confirm", "y", false, "Skip confirmation")

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configEditCmd)
}
