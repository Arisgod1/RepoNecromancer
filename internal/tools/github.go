package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
)

func NewGitHubTools(token string) []Tool {
	var client *github.Client
	if strings.TrimSpace(token) != "" {
		client = github.NewClient(&http.Client{
			Transport: &authTokenRoundTripper{
				token: strings.TrimSpace(token),
				base:  http.DefaultTransport,
			},
		})
	} else {
		client = github.NewClient(nil)
	}
	return []Tool{
		&githubSearchRepositoriesTool{client: client},
		&githubRepositoryTool{client: client},
		&githubIssuesTool{client: client},
		&githubPullRequestsTool{client: client},
		&githubCommitsTool{client: client},
	}
}

type githubSearchRepositoriesTool struct {
	client *github.Client
}

func (t *githubSearchRepositoriesTool) Name() string { return "github.search_repositories" }

func (t *githubSearchRepositoriesTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	years := asInt(input["years"], 3)
	minStars := asInt(input["min_stars"], 0)
	limit := asInt(input["limit"], 20)
	language, _ := input["language"].(string)
	topics := asStringSlice(input["topics"])

	cutoff := time.Now().UTC().AddDate(-years, 0, 0)
	query := []string{
		fmt.Sprintf("stars:>=%d", minStars),
		fmt.Sprintf("pushed:<%s", cutoff.Format("2006-01-02")),
		"archived:false",
	}
	if language != "" {
		query = append(query, fmt.Sprintf("language:%s", language))
	}
	for _, topic := range topics {
		query = append(query, "topic:"+topic)
	}
	q := strings.Join(query, " ")

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	res, _, err := t.client.Search.Repositories(ctx, q, &github.SearchOptions{
		Sort:  "stars",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	})
	if err != nil {
		return nil, err
	}

	repos := make([]map[string]any, 0, len(res.Repositories))
	for _, r := range res.Repositories {
		repos = append(repos, map[string]any{
			"id":               r.GetID(),
			"owner":            r.GetOwner().GetLogin(),
			"name":             r.GetName(),
			"full_name":        r.GetFullName(),
			"html_url":         r.GetHTMLURL(),
			"description":      r.GetDescription(),
			"language":         r.GetLanguage(),
			"topics":           r.Topics,
			"stars":            r.GetStargazersCount(),
			"forks":            r.GetForksCount(),
			"open_issues":      r.GetOpenIssuesCount(),
			"pushed_at":        formatOptionalTime(r.GetPushedAt(), ""),
			"updated_at":       formatOptionalTime(r.GetUpdatedAt(), ""),
			"default_branch":   r.GetDefaultBranch(),
			"archived":         r.GetArchived(),
			"inactivity_years": inactivityYears(r.GetPushedAt()),
		})
	}

	return map[string]any{
		"query":        q,
		"total_count":  res.GetTotal(),
		"repositories": repos,
	}, nil
}

type githubRepositoryTool struct {
	client *github.Client
}

func (t *githubRepositoryTool) Name() string { return "github.repository" }

func (t *githubRepositoryTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	owner, _ := input["owner"].(string)
	repo, _ := input["repo"].(string)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	r, _, err := t.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"repository": map[string]any{
			"owner":            owner,
			"name":             repo,
			"full_name":        r.GetFullName(),
			"html_url":         r.GetHTMLURL(),
			"description":      r.GetDescription(),
			"language":         r.GetLanguage(),
			"topics":           r.Topics,
			"stars":            r.GetStargazersCount(),
			"forks":            r.GetForksCount(),
			"open_issues":      r.GetOpenIssuesCount(),
			"default_branch":   r.GetDefaultBranch(),
			"created_at":       formatOptionalTime(r.GetCreatedAt(), ""),
			"updated_at":       formatOptionalTime(r.GetUpdatedAt(), ""),
			"pushed_at":        formatOptionalTime(r.GetPushedAt(), ""),
			"archived":         r.GetArchived(),
			"inactivity_years": inactivityYears(r.GetPushedAt()),
		},
	}, nil
}

type githubIssuesTool struct {
	client *github.Client
}

func (t *githubIssuesTool) Name() string { return "github.issues" }

func (t *githubIssuesTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	owner, _ := input["owner"].(string)
	repo, _ := input["repo"].(string)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	maxItems := asInt(input["max_items"], 200)
	if maxItems < 1 {
		maxItems = 200
	}
	since := parseOptionalTime(input["since"])
	until := parseOptionalTime(input["until"])

	items := make([]map[string]any, 0, maxItems)
	opt := &github.IssueListByRepoOptions{
		State:     "all",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}
	if !since.IsZero() {
		opt.Since = since
	}
	for len(items) < maxItems {
		issues, resp, err := t.client.Issues.ListByRepo(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		for _, is := range issues {
			if is.GetPullRequestLinks() != nil {
				continue
			}
			created := is.GetCreatedAt()
			if !until.IsZero() && created.After(until) {
				continue
			}
			items = append(items, map[string]any{
				"id":         is.GetID(),
				"number":     is.GetNumber(),
				"url":        is.GetHTMLURL(),
				"title":      is.GetTitle(),
				"state":      is.GetState(),
				"body":       is.GetBody(),
				"created_at": created.Format(time.RFC3339),
				"updated_at": is.GetUpdatedAt().Format(time.RFC3339),
				"user":       is.GetUser().GetLogin(),
				"comments":   is.GetComments(),
			})
			if len(items) >= maxItems {
				break
			}
		}
		if resp == nil || resp.NextPage == 0 || len(items) >= maxItems {
			break
		}
		opt.Page = resp.NextPage
	}
	return map[string]any{"issues": items}, nil
}

type githubPullRequestsTool struct {
	client *github.Client
}

func (t *githubPullRequestsTool) Name() string { return "github.pull_requests" }

func (t *githubPullRequestsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	owner, _ := input["owner"].(string)
	repo, _ := input["repo"].(string)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	maxItems := asInt(input["max_items"], 200)
	if maxItems < 1 {
		maxItems = 200
	}
	since := parseOptionalTime(input["since"])
	until := parseOptionalTime(input["until"])

	items := make([]map[string]any, 0, maxItems)
	opt := &github.PullRequestListOptions{
		State:     "all",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}
	for len(items) < maxItems {
		prs, resp, err := t.client.PullRequests.List(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		for _, pr := range prs {
			created := pr.GetCreatedAt()
			if !since.IsZero() && created.Before(since) {
				continue
			}
			if !until.IsZero() && created.After(until) {
				continue
			}
			items = append(items, map[string]any{
				"id":            pr.GetID(),
				"number":        pr.GetNumber(),
				"url":           pr.GetHTMLURL(),
				"title":         pr.GetTitle(),
				"state":         pr.GetState(),
				"body":          pr.GetBody(),
				"created_at":    created.Format(time.RFC3339),
				"updated_at":    pr.GetUpdatedAt().Format(time.RFC3339),
				"merged_at":     pr.GetMergedAt().Format(time.RFC3339),
				"commits":       pr.GetCommits(),
				"additions":     pr.GetAdditions(),
				"deletions":     pr.GetDeletions(),
				"changed_files": pr.GetChangedFiles(),
				"user":          pr.GetUser().GetLogin(),
			})
			if len(items) >= maxItems {
				break
			}
		}
		if resp == nil || resp.NextPage == 0 || len(items) >= maxItems {
			break
		}
		opt.Page = resp.NextPage
	}
	return map[string]any{"pull_requests": items}, nil
}

type githubCommitsTool struct {
	client *github.Client
}

func (t *githubCommitsTool) Name() string { return "github.commits" }

func (t *githubCommitsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	owner, _ := input["owner"].(string)
	repo, _ := input["repo"].(string)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	maxItems := asInt(input["max_items"], 200)
	if maxItems < 1 {
		maxItems = 200
	}
	since := parseOptionalTime(input["since"])
	until := parseOptionalTime(input["until"])

	items := make([]map[string]any, 0, maxItems)
	opt := &github.CommitsListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}
	if !since.IsZero() {
		opt.Since = since
	}
	if !until.IsZero() {
		opt.Until = until
	}
	for len(items) < maxItems {
		commits, resp, err := t.client.Repositories.ListCommits(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		for _, c := range commits {
			date := c.GetCommit().GetAuthor().GetDate()
			items = append(items, map[string]any{
				"sha":          c.GetSHA(),
				"url":          c.GetHTMLURL(),
				"message":      c.GetCommit().GetMessage(),
				"author":       c.GetCommit().GetAuthor().GetName(),
				"date":         date.Format(time.RFC3339),
				"verification": c.GetCommit().GetVerification().GetReason(),
			})
			if len(items) >= maxItems {
				break
			}
		}
		if resp == nil || resp.NextPage == 0 || len(items) >= maxItems {
			break
		}
		opt.Page = resp.NextPage
	}
	return map[string]any{"commits": items}, nil
}

type authTokenRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (t *authTokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	cloned.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(cloned)
}

func parseOptionalTime(v any) time.Time {
	s, _ := v.(string)
	if strings.TrimSpace(s) == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		return t
	}
	return time.Time{}
}

func formatOptionalTime(ts github.Timestamp, fallback string) string {
	if ts.Time.IsZero() {
		return fallback
	}
	return ts.Time.Format(time.RFC3339)
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func asInt(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return fallback
	}
}

func inactivityYears(ts github.Timestamp) float64 {
	if ts.Time.IsZero() {
		return 0
	}
	return time.Since(ts.Time).Hours() / (24 * 365)
}
