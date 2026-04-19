package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/repo-necromancer/necro/internal/query"
)

func newScanCommand() *cobra.Command {
	var years int
	var minStars int
	var language string
	var topics []string
	var limit int

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Discover candidate dead repositories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if years <= 0 {
				return fmt.Errorf("--years must be > 0")
			}
			if minStars <= 0 {
				return fmt.Errorf("--min-stars must be > 0")
			}
			if limit <= 0 {
				return fmt.Errorf("--limit must be > 0")
			}

			app, err := appFromCmd(cmd)
			if err != nil {
				return err
			}

			req := query.QueryRequest{
				Command:   "scan",
				SessionID: app.SessionID,
				Budget: query.BudgetLimits{
					MaxTurns:  app.Config.Query.MaxTurns,
					MaxTokens: app.Config.Query.MaxTokens,
					MaxCost:   app.Config.Query.MaxCost,
				},
				Actions: []query.Action{
					{
						ToolName: "github.search_repositories",
						Input: map[string]any{
							"years":     years,
							"min_stars": minStars,
							"language":  language,
							"topics":    topics,
							"limit":     limit,
						},
					},
				},
			}

			result, err := app.Query.Run(context.Background(), req)
			if err != nil {
				return err
			}
			if len(result.Executions) == 0 {
				return fmt.Errorf("scan did not run any actions")
			}
			exec := result.Executions[0]
			if exec.Error != "" {
				return fmt.Errorf("scan failed: %s", exec.Error)
			}
			repos := asMapSlice(exec.Output["repositories"])
			if len(repos) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No repositories matched the dead-repo criteria.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Ranked dead repository candidates (%d):\n", len(repos))
			for i, r := range repos {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%2d. %-40s stars=%-7v inactivity_years=%.2f language=%s\n",
					i+1,
					stringValue(r["full_name"]),
					r["stars"],
					floatValue(r["inactivity_years"]),
					strings.TrimSpace(stringValue(r["language"])),
				)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&years, "years", 0, "Inactivity threshold in years")
	cmd.Flags().IntVar(&minStars, "min-stars", 0, "Minimum star count")
	cmd.Flags().StringVar(&language, "language", "", "Filter by language")
	cmd.Flags().StringSliceVar(&topics, "topic", nil, "Filter by topic (repeatable)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results")
	_ = cmd.MarkFlagRequired("years")
	_ = cmd.MarkFlagRequired("min-stars")
	return cmd
}

func asMapSlice(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func floatValue(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}
