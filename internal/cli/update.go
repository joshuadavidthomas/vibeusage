package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
	"github.com/joshuadavidthomas/vibeusage/internal/updater"
)

var (
	updateCheckOnly bool
	updateYes       bool
	updateVersion   string
)

var updaterFactory = func() updater.Service {
	return updater.NewClient()
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for updates and install newer releases",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate(cmd.Context())
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "Check for updates without installing")
	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false, "Install update without interactive confirmation")
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "Install a specific version (for example: v1.2.3)")
}

func runUpdate(ctx context.Context) error {
	service := updaterFactory()
	check, err := service.Check(ctx, updater.CheckRequest{
		CurrentVersion: version,
		TargetVersion:  updateVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if updateCheckOnly {
		return outputUpdateCheck(check)
	}

	if !check.UpdateAvailable {
		return outputUpdateCheck(check)
	}

	if jsonOutput && !updateYes {
		return fmt.Errorf("--json update install requires --yes to avoid interactive prompts")
	}

	if !updateYes {
		confirmed, err := confirmUpdate(check)
		if err != nil {
			return err
		}
		if !confirmed {
			if jsonOutput {
				return display.OutputJSON(outWriter, display.UpdateStatusJSON{
					CurrentVersion:  check.CurrentVersion,
					LatestVersion:   check.LatestVersion,
					TargetVersion:   check.TargetVersion,
					UpdateAvailable: check.UpdateAvailable,
					IsDowngrade:     check.IsDowngrade,
					Applied:         false,
				})
			}
			outln("Update canceled")
			return nil
		}
	}

	apply, err := service.Apply(ctx, updater.ApplyRequest{
		Check:          check,
		AllowDowngrade: updateVersion != "",
	})
	if err != nil {
		return fmt.Errorf("failed to apply update: %w", err)
	}

	if jsonOutput {
		return display.OutputJSON(outWriter, display.UpdateStatusJSON{
			CurrentVersion:  check.CurrentVersion,
			LatestVersion:   check.LatestVersion,
			TargetVersion:   check.TargetVersion,
			UpdateAvailable: check.UpdateAvailable,
			IsDowngrade:     check.IsDowngrade,
			Asset:           check.AssetName,
			Applied:         apply.Updated,
			Pending:         apply.Pending,
		})
	}

	if apply.Pending {
		out("✓ Staged update to %s; restart your shell and rerun vibeusage\n", check.TargetVersion)
		return nil
	}

	out("✓ Updated vibeusage %s → %s\n", check.CurrentVersion, check.TargetVersion)
	return nil
}

func outputUpdateCheck(check updater.CheckResult) error {
	if jsonOutput {
		return display.OutputJSON(outWriter, display.UpdateStatusJSON{
			CurrentVersion:  check.CurrentVersion,
			LatestVersion:   check.LatestVersion,
			TargetVersion:   check.TargetVersion,
			UpdateAvailable: check.UpdateAvailable,
			IsDowngrade:     check.IsDowngrade,
			Asset:           check.AssetName,
		})
	}

	if check.UpdateAvailable {
		if check.IsDowngrade {
			out("Version change available (downgrade): %s → %s\n", check.CurrentVersion, check.TargetVersion)
		} else {
			out("Update available: %s → %s\n", check.CurrentVersion, check.TargetVersion)
		}
		out("Run `vibeusage update --yes` to install.\n")
		return nil
	}

	out("vibeusage is up to date (%s)\n", check.CurrentVersion)
	return nil
}

func confirmUpdate(check updater.CheckResult) (bool, error) {
	if !isTerminal() {
		return false, fmt.Errorf("interactive confirmation required; rerun with --yes")
	}

	title := fmt.Sprintf("Install update %s → %s?", check.CurrentVersion, check.TargetVersion)
	if check.IsDowngrade {
		title = fmt.Sprintf("Install downgrade %s → %s?", check.CurrentVersion, check.TargetVersion)
	}

	confirmed, err := prompt.Default.Confirm(prompt.ConfirmConfig{
		Title:       title,
		Description: "The binary will be replaced in place.",
		Affirmative: "Install",
		Negative:    "Cancel",
	})
	if err != nil {
		return false, err
	}
	return confirmed, nil
}
