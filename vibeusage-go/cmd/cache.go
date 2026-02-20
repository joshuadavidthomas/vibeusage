package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage cached usage data",
}

var cacheShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show cache status per provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()
		staleThreshold := cfg.Fetch.StaleThresholdMinutes

		ids := provider.ListIDs()
		sort.Strings(ids)

		type cacheInfo struct {
			Snapshot string `json:"snapshot"`
			OrgID    bool   `json:"org_id_cached"`
			Age      *int   `json:"age_minutes"`
		}

		cacheData := make(map[string]cacheInfo)

		for _, pid := range ids {
			info := cacheInfo{Snapshot: "none"}

			snap := config.LoadCachedSnapshot(pid)
			if snap != nil {
				age := int(time.Since(snap.FetchedAt).Minutes())
				info.Age = &age
				if age < staleThreshold {
					info.Snapshot = "fresh"
				} else {
					info.Snapshot = "stale"
				}
			}

			orgPath := config.OrgIDPath(pid)
			if _, err := os.Stat(orgPath); err == nil {
				info.OrgID = true
			}

			cacheData[pid] = info
		}

		if jsonOutput {
			display.OutputJSON(cacheData)
			return nil
		}

		if quiet {
			for _, pid := range ids {
				fmt.Printf("%s: %s\n", pid, cacheData[pid].Snapshot)
			}
			return nil
		}

		fmt.Println("Cache Status")
		fmt.Println(strings.Repeat("─", 50))
		fmt.Printf("%-12s %-12s %-8s %s\n", "Provider", "Snapshot", "Org ID", "Age")
		fmt.Println(strings.Repeat("─", 50))

		for _, pid := range ids {
			info := cacheData[pid]
			snapStatus := "—"
			switch info.Snapshot {
			case "fresh":
				snapStatus = "✓ Fresh"
			case "stale":
				snapStatus = "⚠ Stale"
			}

			orgStatus := "—"
			if info.OrgID {
				orgStatus = "✓"
			}

			ageStr := "—"
			if info.Age != nil {
				a := *info.Age
				if a >= 1440 {
					ageStr = fmt.Sprintf("%dd", a/1440)
				} else if a >= 60 {
					ageStr = fmt.Sprintf("%dh", a/60)
				} else if a >= 1 {
					ageStr = fmt.Sprintf("%dm", a)
				} else {
					ageStr = "<1m"
				}
			}

			fmt.Printf("%-12s %-12s %-8s %s\n", pid, snapStatus, orgStatus, ageStr)
		}

		fmt.Printf("\nCache directory: %s\n", config.CacheDir())
		return nil
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear [provider]",
	Short: "Clear cache data",
	RunE: func(cmd *cobra.Command, args []string) error {
		var providerID string
		if len(args) > 0 {
			providerID = args[0]
			if _, ok := provider.Get(providerID); !ok {
				return fmt.Errorf("unknown provider: %s", providerID)
			}
		}

		config.ClearAllCache(providerID)

		msg := "Cleared all cache"
		if providerID != "" {
			msg = fmt.Sprintf("Cleared cache for %s", providerID)
		}

		if jsonOutput {
			display.OutputJSON(map[string]any{
				"success":  true,
				"message":  msg,
				"provider": providerID,
			})
			return nil
		}

		fmt.Printf("✓ %s\n", msg)
		return nil
	},
}

func init() {
	cacheCmd.AddCommand(cacheShowCmd)
	cacheCmd.AddCommand(cacheClearCmd)
}
