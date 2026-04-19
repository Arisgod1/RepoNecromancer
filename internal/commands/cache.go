package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/repo-necromancer/necro/internal/tools"
)

func newCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache <subcommand>",
		Short: "Manage the GitHub API response cache",
		Long: `Manage the TTL-based in-memory cache used for GitHub API responses.

Subcommands:
  stats  - Show cache statistics (total, active, expired keys)
  list   - List all cached keys with their TTL status
  clear  - Clear all entries from the cache

TTL Policies:
  - Normal entries:    5 minutes
  - GitHub HIT (repo found): 2 minutes  
  - 404 Dead repos:    1 hour
  - Errors:            5 minutes`,
	}

	cmd.AddCommand(
		newCacheStatsCommand(),
		newCacheListCommand(),
		newCacheClearCommand(),
	)

	return cmd
}

func newCacheStatsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show cache statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			stats := tools.GlobalCache().Stats()
			fmt.Fprintf(cmd.OutOrStdout(), "Cache Statistics:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Total keys:   %d\n", stats.TotalKeys)
			fmt.Fprintf(cmd.OutOrStdout(), "  Active keys: %d\n", stats.ActiveKeys)
			fmt.Fprintf(cmd.OutOrStdout(), "  Expired keys: %d\n", stats.ExpiredKeys)
			return nil
		},
	}
}

func newCacheListCommand() *cobra.Command {
	var showExpired bool

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all cached keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := tools.GlobalCache().Keys()
			if len(keys) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No active cache entries.")
				return nil
			}

			sort.Strings(keys)
			fmt.Fprintf(cmd.OutOrStdout(), "Active cache entries (%d):\n", len(keys))
			for _, key := range keys {
				// Categorize by prefix
				prefix := categorizeKey(key)
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", prefix, key)
			}

			if showExpired {
				stats := tools.GlobalCache().Stats()
				if stats.ExpiredKeys > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "\nExpired entries: %d (auto-cleaned on next access)\n", stats.ExpiredKeys)
				}
			}

			return nil
		},
	}

	listCmd.Flags().BoolVar(&showExpired, "show-expired", false, "Also show expired key count")
	return listCmd
}

func newCacheClearCommand() *cobra.Command {
	var force bool

	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear all cache entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			stats := tools.GlobalCache().Stats()
			if stats.TotalKeys == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Cache is already empty.")
				return nil
			}

			if !force {
				fmt.Fprintf(cmd.OutOrStdout(), "This will clear %d entries. Use --force to confirm.\n", stats.TotalKeys)
				return nil
			}

			tools.GlobalCache().Clear()
			fmt.Fprintf(cmd.OutOrStdout(), "Cache cleared (%d entries removed).\n", stats.TotalKeys)
			return nil
		},
	}

	clearCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return clearCmd
}

func categorizeKey(key string) string {
	if strings.HasPrefix(key, "github:repo:") {
		if strings.HasSuffix(key, ":404") {
			return "404-DEAD"
		}
		if strings.HasSuffix(key, ":error") {
			return "ERROR"
		}
		return "REPO"
	}
	if strings.HasPrefix(key, "github:search:") {
		return "SEARCH"
	}
	if strings.HasPrefix(key, "github:issues:") {
		return "ISSUES"
	}
	if strings.HasPrefix(key, "github:prs:") {
		return "PRs"
	}
	if strings.HasPrefix(key, "github:commits:") {
		return "COMMITS"
	}
	return "OTHER"
}
