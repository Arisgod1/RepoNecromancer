package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/repo-necromancer/necro/internal/i18n"
	"github.com/repo-necromancer/necro/internal/query"
)

func newScanCommand() *cobra.Command {
	var years int
	var minStars int
	var language string
	var topics []string
	var limit int
	var repos string
	var parallel int

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
			if parallel <= 0 {
				parallel = 4
			}

			app, err := appFromCmd(cmd)
			if err != nil {
				return err
			}

			// Handle multi-repo scanning
			if repos != "" {
				return runMultiRepoScan(cmd, app, repos, parallel, years, minStars, language, topics, limit)
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
				tr := i18n.GetTranslator()
				lang := app.Config.App.Language
				if lang == "" {
					lang = "zh"
				}
				fmt.Fprintln(cmd.OutOrStdout(), tr.T(lang, "no_repositories_matched"))
				return nil
			}

			tr := i18n.GetTranslator()
			lang := app.Config.App.Language
			if lang == "" {
				lang = "zh"
			}
			t := func(key string) string { return tr.T(lang, key) }

			fmt.Fprintf(cmd.OutOrStdout(), "%s (%d):\n", t("ranked_dead_repository_candidates"), len(repos))
			for i, r := range repos {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%2d. %-40s %s=%-7v %s=%.2f %s=%s\n",
					i+1,
					stringValue(r["full_name"]),
					t("stars"),
					r["stars"],
					t("inactivity_years"),
					floatValue(r["inactivity_years"]),
					t("language"),
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
	cmd.Flags().StringVar(&repos, "repos", "", "Comma-separated list of owner/repo to scan")
	cmd.Flags().IntVar(&parallel, "parallel", 4, "Max concurrent scans (default: 4)")
	_ = cmd.MarkFlagRequired("years")
	_ = cmd.MarkFlagRequired("min-stars")
	return cmd
}

// repoResult holds scan results for a single repo
type repoResult struct {
	fullName        string
	stars           any
	inactivityYears float64
	language        string
}

// runMultiRepoScan scans multiple repositories in parallel and aggregates results
func runMultiRepoScan(cmd *cobra.Command, app *App, repos string, parallel int, years, minStars int, language string, topics []string, limit int) error {
	// Parse and deduplicate repos
	seen := make(map[string]bool)
	var repoList []string
	for _, r := range strings.Split(repos, ",") {
		r = strings.TrimSpace(r)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		repoList = append(repoList, r)
	}

	if len(repoList) == 0 {
		return fmt.Errorf("no valid repositories provided")
	}

	// Run scans in parallel with errgroup
	ctx := cmd.Context()
	results := make([]repoResult, 0, len(repoList))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	eg, ctx := errgroup.WithContext(ctx)

	for _, repo := range repoList {
		owner, repoName, err := parseOwnerRepo(repo)
		if err != nil {
			continue
		}

		wg.Add(1)
		go func(owner, repoName string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			req := query.QueryRequest{
				Command:   "autopsy",
				SessionID: app.SessionID + "-scan-" + owner + "-" + repoName,
				Budget: query.BudgetLimits{
					MaxTurns:  app.Config.Query.MaxTurns,
					MaxTokens: app.Config.Query.MaxTokens,
					MaxCost:   app.Config.Query.MaxCost,
				},
				Actions: []query.Action{
					{
						ToolName: "github.repository",
						Input:    map[string]any{"owner": owner, "repo": repoName},
					},
				},
			}

			res, err := app.Query.Run(ctx, req)
			if err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, ex := range res.Executions {
				if ex.Error != "" || ex.ToolName != "github.repository" {
					continue
				}
				if repoObj, ok := ex.Output["repository"].(map[string]any); ok {
					inactivityYears := float64(years) // default to threshold if not calculable
					if py := floatValue(repoObj["pushed_at"]); py > 0 {
						// pushed_at is a Unix timestamp
						inactivityYears = float64(time.Now().Unix()-int64(py)) / (365.25 * 24 * 60 * 60)
					}
					results = append(results, repoResult{
						fullName:        stringValue(repoObj["full_name"]),
						stars:           repoObj["stars"],
						inactivityYears: inactivityYears,
						language:        strings.TrimSpace(stringValue(repoObj["language"])),
					})
				}
			}
		}(owner, repoName)
	}

	wg.Wait()
	_ = eg.Wait() // Already using waitgroup, eg is for context cancellation

	lang := app.Config.App.Language
	if lang == "" {
		lang = "zh"
	}
	tr := i18n.GetTranslator()
	t := func(key string) string { return tr.T(lang, key) }

	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), t("no_repositories_found"))
		return nil
	}

	// Sort by stars descending
	sort.Slice(results, func(i, j int) bool {
		return floatValue(results[i].stars) > floatValue(results[j].stars)
	})

	fmt.Fprintf(cmd.OutOrStdout(), "%s (%d):\n", t("multi_repo_scan_results"), len(results))
	for i, r := range results {
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"%2d. %-40s %s=%-7v %s=%.2f %s=%s\n",
			i+1,
			r.fullName,
			t("stars"),
			r.stars,
			t("inactivity_years"),
			r.inactivityYears,
			t("language"),
			r.language,
		)
	}
	return nil
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
