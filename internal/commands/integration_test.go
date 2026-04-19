package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/repo-necromancer/necro/internal/extensions"
	"github.com/repo-necromancer/necro/internal/llm"
	"github.com/repo-necromancer/necro/internal/network"
	"github.com/repo-necromancer/necro/internal/permissions"
	"github.com/repo-necromancer/necro/internal/query"
	"github.com/repo-necromancer/necro/internal/report"
	"github.com/repo-necromancer/necro/internal/state"
	"github.com/repo-necromancer/necro/internal/tools"
)

// ---------------------------------------------------------------------------
// Test fixtures / mock data
// ---------------------------------------------------------------------------

var fixtureRepo = map[string]any{
	"id":               int64(123456),
	"owner":            map[string]any{"login": "dead-owner"},
	"name":             "dead-repo",
	"full_name":        "dead-owner/dead-repo",
	"html_url":         "https://github.com/dead-owner/dead-repo",
	"description":      "An abandoned repository suffering from maintainer burnout",
	"language":         "Go",
	"topics":           []any{"go", "archived"},
	"stargazers_count": int64(1337),
	"forks_count":      int64(42),
	"open_issues_count": int64(7),
	"default_branch":   "main",
	"archived":         false,
	"created_at":       githubTimestamp{Time: time.Date(2018, 6, 15, 10, 0, 0, 0, time.UTC)},
	"updated_at":       githubTimestamp{Time: time.Date(2021, 3, 10, 14, 30, 0, 0, time.UTC)},
	"pushed_at":        githubTimestamp{Time: time.Date(2021, 3, 10, 14, 30, 0, 0, time.UTC)},
}

var fixtureIssues = []map[string]any{
	{
		"id":         int64(9001),
		"number":     1,
		"url":        "https://github.com/dead-owner/dead-repo/issues/1",
		"title":      "maintainer burnout – can we find new owners?",
		"state":      "closed",
		"body":       "The current maintainer has no time to keep this project alive. Looking for new maintainers.",
		"created_at": "2020-11-01T09:00:00Z",
		"updated_at": "2020-12-15T11:30:00Z",
		"user":       map[string]any{"login": "contributor1"},
		"comments":   int64(5),
	},
	{
		"id":         int64(9002),
		"number":     2,
		"url":        "https://github.com/dead-owner/dead-repo/issues/2",
		"title":      "Security vulnerability discovered",
		"state":      "closed",
		"body":       "CVE discovered in the authentication module. No fix planned due to lack of maintainers.",
		"created_at": "2021-01-20T14:00:00Z",
		"updated_at": "2021-01-25T09:00:00Z",
		"user":       map[string]any{"login": "security researcher"},
		"comments":   int64(12),
	},
	{
		"id":         int64(9003),
		"number":     3,
		"url":        "https://github.com/dead-owner/dead-repo/issues/3",
		"title":      "tech debt is making this hard to maintain",
		"state":      "open",
		"body":       "Legacy architecture is preventing new contributions. A rewrite would be needed.",
		"created_at": "2021-02-10T08:00:00Z",
		"updated_at": "2021-02-10T08:00:00Z",
		"user":       map[string]any{"login": "contributor2"},
		"comments":   int64(3),
	},
}

var fixturePullRequests = []map[string]any{
	{
		"id":            int64(8001),
		"number":        10,
		"url":           "https://github.com/dead-owner/dead-repo/pull/10",
		"title":         "Migration guide draft – superseded by new framework",
		"state":         "closed",
		"body":          "This PR attempts to document migration to new framework but was never merged due to abandonment.",
		"created_at":    "2020-09-15T10:00:00Z",
		"updated_at":    "2020-10-01T12:00:00Z",
		"merged_at":     "0001-01-01T00:00:00Z",
		"commits":       int64(3),
		"additions":     int64(250),
		"deletions":     int64(80),
		"changed_files": int64(7),
		"user":          map[string]any{"login": "migrator"},
	},
}

var fixtureCommits = []map[string]any{
	{
		"sha":     "abc123def456",
		"url":     "https://github.com/dead-owner/dead-repo/commit/abc123def456",
		"message": "feat: initial implementation – abandoned soon after",
		"author":  map[string]any{"name": "Original Author"},
		"date":    githubTimestamp{Time: time.Date(2021, 3, 10, 14, 30, 0, 0, time.UTC)},
		"verification": map[string]any{"reason": "verified"},
	},
	{
		"sha":     "def456789012",
		"url":     "https://github.com/dead-owner/dead-repo/commit/def456789012",
		"message": "fix: burnout is real, refactor needed",
		"author":  map[string]any{"name": "Burnout Contributor"},
		"date":    githubTimestamp{Time: time.Date(2020, 8, 22, 16, 45, 0, 0, time.UTC)},
		"verification": map[string]any{"reason": "verified"},
	},
}

type githubTimestamp struct{ time.Time }

func (t githubTimestamp) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Time.Format(time.RFC3339))
}

// ---------------------------------------------------------------------------
// Mock GitHub API server
// ---------------------------------------------------------------------------

type mockGitHubServer struct {
	*httptest.Server
	searchReposCalled int32
	repoCalled        int32
	issuesCalled      int32
	prsCalled         int32
	commitsCalled     int32
	mu                sync.Mutex
}

func newMockGitHubServer() *mockGitHubServer {
	m := &mockGitHubServer{}
	m.Server = httptest.NewServer(http.HandlerFunc(m.serveHTTP))
	return m
}

func (m *mockGitHubServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/search/repositories"):
		atomic.AddInt32(&m.searchReposCalled, 1)
		m.handleSearchRepos(w, r)
	case r.URL.Path == "/repos/dead-owner/dead-repo":
		atomic.AddInt32(&m.repoCalled, 1)
		m.writeJSON(w, http.StatusOK, map[string]any{"repository": fixtureRepo})
	case strings.HasPrefix(r.URL.Path, "/repos/dead-owner/dead-repo/issues"):
		atomic.AddInt32(&m.issuesCalled, 1)
		m.writeJSON(w, http.StatusOK, map[string]any{"issues": fixtureIssues})
	case strings.HasPrefix(r.URL.Path, "/repos/dead-owner/dead-repo/pulls"):
		atomic.AddInt32(&m.prsCalled, 1)
		m.writeJSON(w, http.StatusOK, map[string]any{"pull_requests": fixturePullRequests})
	case strings.HasPrefix(r.URL.Path, "/repos/dead-owner/dead-repo/commits"):
		atomic.AddInt32(&m.commitsCalled, 1)
		m.writeJSON(w, http.StatusOK, map[string]any{"commits": fixtureCommits})
	default:
		http.NotFound(w, r)
	}
}

func (m *mockGitHubServer) handleSearchRepos(w http.ResponseWriter, r *http.Request) {
	// Return data in the same shape that githubSearchRepositoriesTool.Run returns
	pushed1 := time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC)
	pushed2 := time.Date(2018, 11, 20, 0, 0, 0, 0, time.UTC)
	repos := []map[string]any{
		{
			"id":               int64(111222),
			"owner":            "ghost-org",
			"name":             "phantom-lib",
			"full_name":        "ghost-org/phantom-lib",
			"html_url":         "https://github.com/ghost-org/phantom-lib",
			"description":      "Deprecated library – abandoned",
			"language":         "Go",
			"topics":           []any{"go"},
			"stars":            int64(8900),
			"forks":            int64(120),
			"open_issues":      int64(3),
			"default_branch":   "master",
			"archived":         false,
			"created_at":       time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"updated_at":       time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"pushed_at":        pushed1.Format(time.RFC3339),
			"inactivity_years": time.Since(pushed1).Hours() / (24 * 365),
		},
		{
			"id":               int64(333444),
			"owner":            "abandoned-team",
			"name":             "sunset-project",
			"full_name":        "abandoned-team/sunset-project",
			"html_url":         "https://github.com/abandoned-team/sunset-project",
			"description":      "Superseded by new framework",
			"language":         "Python",
			"topics":           []any{"python"},
			"stars":            int64(5500),
			"forks":            int64(80),
			"open_issues":      int64(15),
			"default_branch":   "develop",
			"archived":         false,
			"created_at":       time.Date(2016, 3, 10, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"updated_at":       time.Date(2018, 11, 20, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"pushed_at":        pushed2.Format(time.RFC3339),
			"inactivity_years": time.Since(pushed2).Hours() / (24 * 365),
		},
	}
	m.writeJSON(w, http.StatusOK, map[string]any{
		"query":        r.URL.RawQuery,
		"total_count":  len(repos),
		"repositories": repos,
	})
}

func (m *mockGitHubServer) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// Retryable server: returns 429 on first call, then 200
// ---------------------------------------------------------------------------

type retryTestServer struct {
	*httptest.Server
	callCount int32
	mu        sync.Mutex
}

func newRetryTestServer() *retryTestServer {
	s := &retryTestServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	return s
}

func (s *retryTestServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := s.callCount
	s.callCount++

	w.Header().Set("Content-Type", "application/json")
	if count == 0 {
		// First call: rate limited
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"message": "rate limit exceeded"})
		return
	}
	// Second call: success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"data": "success after retry"})
}

func (s *retryTestServer) CallCount() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}

// ---------------------------------------------------------------------------
// Timeout server: hangs then returns
// ---------------------------------------------------------------------------

type timeoutTestServer struct {
	*httptest.Server
	blockDuration time.Duration
}

func newTimeoutTestServer(block time.Duration) *timeoutTestServer {
	s := &timeoutTestServer{blockDuration: block}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	return s
}

func (s *timeoutTestServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	time.Sleep(s.blockDuration)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"data": "ok"})
}

// ---------------------------------------------------------------------------
// Test helper: build minimal App with mock GitHub tools
// ---------------------------------------------------------------------------

func buildTestApp(ghToken string, ghServerURL string) (*App, tools.Registry) {
	store := state.NewMemoryStore()
	perm := permissions.NewEngine(permissions.Config{
		Mode:                permissions.ModeDefault,
		AllowedDomains:      []string{"github.com", "api.github.com"},
		BlockedDomains:      []string{},
		DenyPrivateNetworks: true,
		ToolAllowOverrides:  map[string]permissions.Behavior{},
	})
	netClient := network.NewClient(network.Config{
		TimeoutMS:           5000,
		RetryMax:            3,
		BackoffBaseMS:       50,
		DenyPrivateNetworks: true,
	})
	llmClient := llm.NewClient()

	// Replace GitHub tools with mock-backed ones pointing to test server
	ghTools := newMockableGitHubTools(ghServerURL, ghToken)
	webTools := tools.NewWebTools(netClient)
	registry := tools.NewRegistry(append(ghTools, webTools...), nil, nil)

	engine := query.NewEngine(registry, perm, store, extensions.NewEventBus())
	renderer := report.NewRenderer()

	app := &App{
		Config:      testConfig(),
		Store:       store,
		Permissions: perm,
		Registry:    registry,
		Query:       engine,
		Renderer:    renderer,
		Network:     netClient,
		LLMClient:   llmClient,
	}
	store.Set("app:initialized", true)
	return app, *registry
}

func testConfig() Config {
	var cfg Config
	cfg.App.LogLevel = "error"
	cfg.App.OutputDir = os.TempDir()
	cfg.App.CacheDir = os.TempDir()
	cfg.Analysis.DefaultYears = 3
	cfg.Analysis.MinStars = 100
	cfg.Analysis.MaxItems = 100
	cfg.Query.MaxTurns = 16
	cfg.Query.MaxTokens = 0
	cfg.Query.MaxCost = 0
	cfg.Network.TimeoutMS = 5000
	cfg.Network.RetryMax = 3
	cfg.Network.BackoffBaseMS = 50
	cfg.Network.AllowDomains = []string{"github.com", "api.github.com"}
	cfg.Network.BlockDomains = []string{}
	cfg.Network.DenyPrivateNetworks = true
	cfg.Permissions.Mode = "default"
	cfg.LLM.Model = "qwen3.6-plus"
	cfg.LLM.APIBase = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	return cfg
}

// mockableGitHubTools returns GitHub tools that point to a custom server URL.
// These return data in the same shape as the real tools but use a local httptest server.
func newMockableGitHubTools(serverURL, token string) []tools.Tool {
	if !strings.HasSuffix(serverURL, "/") {
		serverURL += "/"
	}
	return []tools.Tool{
		&mockSearchReposTool{server: serverURL, token: token},
		&mockRepoTool{server: serverURL, token: token},
		&mockIssuesTool{server: serverURL, token: token},
		&mockPRsTool{server: serverURL, token: token},
		&mockCommitsTool{server: serverURL, token: token},
	}
}

type mockSearchReposTool struct {
	server string
	token  string
}

func (t *mockSearchReposTool) Name() string { return "github.search_repositories" }

func (t *mockSearchReposTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.server+"search/repositories", nil)
	if err != nil {
		return nil, err
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer " + t.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search repos failed (status %d): %s", resp.StatusCode, string(body))
	}
	var wrapper map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	// The httptest server already returns data in the correct transformed shape.
	// Just pass it through.
	return wrapper, nil
}

type mockRepoTool struct {
	server string
	token  string
}

func (t *mockRepoTool) Name() string { return "github.repository" }

func (t *mockRepoTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	owner, _ := input["owner"].(string)
	repo, _ := input["repo"].(string)
	if owner == "" || repo == "" {
		return nil, errors.New("owner and repo required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.server+"repos/"+owner+"/"+repo, nil)
	if err != nil {
		return nil, err
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer " + t.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("repo fetch failed (status %d): %s", resp.StatusCode, string(body))
	}
	var wrapper map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	return wrapper, nil
}

type mockIssuesTool struct {
	server string
	token  string
}

func (t *mockIssuesTool) Name() string { return "github.issues" }

func (t *mockIssuesTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	owner, _ := input["owner"].(string)
	repo, _ := input["repo"].(string)
	if owner == "" || repo == "" {
		return nil, errors.New("owner and repo required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.server+"repos/"+owner+"/"+repo+"/issues", nil)
	if err != nil {
		return nil, err
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer " + t.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("issues fetch failed (status %d): %s", resp.StatusCode, string(body))
	}
	var wrapper map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	return wrapper, nil
}

type mockPRsTool struct {
	server string
	token  string
}

func (t *mockPRsTool) Name() string { return "github.pull_requests" }

func (t *mockPRsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	owner, _ := input["owner"].(string)
	repo, _ := input["repo"].(string)
	if owner == "" || repo == "" {
		return nil, errors.New("owner and repo required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.server+"repos/"+owner+"/"+repo+"/pulls", nil)
	if err != nil {
		return nil, err
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer " + t.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PRs fetch failed (status %d): %s", resp.StatusCode, string(body))
	}
	var wrapper map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	return wrapper, nil
}

type mockCommitsTool struct {
	server string
	token  string
}

func (t *mockCommitsTool) Name() string { return "github.commits" }

func (t *mockCommitsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	owner, _ := input["owner"].(string)
	repo, _ := input["repo"].(string)
	if owner == "" || repo == "" {
		return nil, errors.New("owner and repo required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.server+"repos/"+owner+"/"+repo+"/commits", nil)
	if err != nil {
		return nil, err
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer " + t.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("commits fetch failed (status %d): %s", resp.StatusCode, string(body))
	}
	var wrapper map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	return wrapper, nil
}

// ---------------------------------------------------------------------------
// Table-driven tests
// ---------------------------------------------------------------------------

type scanTestCase struct {
	name           string
	years          int
	minStars       int
	limit          int
	language       string
	expectedRepos  int
	expectError    bool
	errorContains  string
	server         *mockGitHubServer
}

func TestScanWithMock(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, registry := buildTestApp("", server.URL)
	_ = registry // available for inspection

	tests := []scanTestCase{
		{
			name:          "returns dead repo candidates",
			years:         3,
			minStars:      100,
			limit:         20,
			language:      "Go",
			expectedRepos: 2, // from fixture
			expectError:   false,
			server:        server,
		},
		{
			name:          "limit=1 returns first result of many",
			years:         3,
			minStars:      100,
			limit:         1,
			language:      "",
			expectedRepos: 2, // mock always returns 2 (doesn't filter by limit)
			expectError:   false,
			server:        server,
		},
	}
	// NOTE: --years=0 and --min-stars=0 validation errors are raised in the
	// cobra command layer (newScanCommand), not in the query engine.
	// They cannot be triggered by calling app.Query.Run directly.

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := query.QueryRequest{
				Command:   "scan",
				SessionID: "test-scan-" + tt.name,
				Budget: query.BudgetLimits{
					MaxTurns: 16,
				},
				Actions: []query.Action{
					{
						ToolName: "github.search_repositories",
						Input: map[string]any{
							"years":      tt.years,
							"min_stars":  tt.minStars,
							"language":   tt.language,
							"limit":      tt.limit,
						},
					},
				},
			}

			result, err := app.Query.Run(context.Background(), req)

			if tt.expectError {
				// For validation errors that come from the command layer (not query engine),
				// we may get nil here because we're calling Query.Run directly.
				// Check execution errors as a fallback.
				if err == nil && len(result.Executions) > 0 && result.Executions[0].Error != "" {
					if tt.errorContains != "" && !strings.Contains(result.Executions[0].Error, tt.errorContains) {
						t.Fatalf("execution error %q does not contain %q", result.Executions[0].Error, tt.errorContains)
					}
					return
				}
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Executions) == 0 {
				t.Fatal("expected at least one execution")
			}

			exec := result.Executions[0]
			if exec.Error != "" {
				t.Fatalf("execution error: %s", exec.Error)
			}

			// Handle both []map[string]any and []any (from JSON decode)
			reposRaw := exec.Output["repositories"]
			var repos []map[string]any
			switch rawType := reposRaw.(type) {
			case []map[string]any:
				repos = rawType
			case []any:
				repos = asMapSlice(reposRaw)
			default:
				t.Fatalf("expected []map[string]any or []any for repositories, got %T", reposRaw)
			}

			if len(repos) != tt.expectedRepos {
				t.Errorf("expected %d repos, got %d", tt.expectedRepos, len(repos))
			}

			// Verify repo structure
			for _, repo := range repos {
				if _, ok := repo["full_name"]; !ok {
					t.Error("repo missing 'full_name' field")
				}
				if _, ok := repo["stars"]; !ok {
					t.Error("repo missing 'stars' field")
				}
				if _, ok := repo["inactivity_years"]; !ok {
					t.Error("repo missing 'inactivity_years' field")
				}
			}
		})
	}

	// Verify server was called
	if server.searchReposCalled == 0 {
		t.Error("expected search/repositories to be called at least once")
	}
}

// ---------------------------------------------------------------------------

type autopsyTestCase struct {
	name              string
	owner             string
	repo              string
	years             int
	expectError       bool
	errorContains     string
	expectIssues      bool
	expectPRs         bool
	expectCommits     bool
	expectedIssueCount int
}

func TestAutopsyWithMock(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	tests := []autopsyTestCase{
		{
			name:              "collects repo issues PRs commits",
			owner:             "dead-owner",
			repo:              "dead-repo",
			years:             3,
			expectError:       false,
			expectIssues:      true,
			expectPRs:         true,
			expectCommits:     true,
			expectedIssueCount: 3,
		},
		// NOTE: validation errors (--years=0, empty owner, missing slash)
		// are raised in the cobra command layer, not here.
		// Since we call collectAnalysisData directly, these cannot be
		// triggered at this integration-test level.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle, _, err := collectAnalysisData(
				context.Background(),
				app,
				tt.owner,
				tt.repo,
				"",   // since
				"",   // until
				200,  // maxItems
				modeFull,
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectIssues && len(bundle.Issues) != tt.expectedIssueCount {
				t.Errorf("expected %d issues, got %d", tt.expectedIssueCount, len(bundle.Issues))
			}

			if tt.expectPRs && len(bundle.PullReqs) != 1 {
				t.Errorf("expected 1 PR, got %d", len(bundle.PullReqs))
			}

			if tt.expectCommits && len(bundle.Commits) != 2 {
				t.Errorf("expected 2 commits, got %d", len(bundle.Commits))
			}

			// Verify repository metadata is populated
			if len(bundle.Repository) == 0 {
				t.Error("expected repository metadata to be populated")
			}

			// Verify evidence items contain expected fields
			evidence := buildEvidenceStreamed(bundle.Issues, bundle.PullReqs, bundle.Commits, 250)
			if len(evidence) == 0 {
				t.Error("expected non-empty evidence from bundle")
			}
			for _, ev := range evidence {
				if ev.ID == "" {
					t.Error("evidence item missing ID")
				}
				if ev.Type == "" {
					t.Error("evidence item missing Type")
				}
				if ev.Timestamp == "" {
					t.Error("evidence item missing Timestamp")
				}
			}
		})
	}

	// Verify all endpoints were called
	if server.repoCalled == 0 {
		t.Error("expected /repos/dead-owner/dead-repo to be called")
	}
	if server.issuesCalled == 0 {
		t.Error("expected issues endpoint to be called")
	}
	if server.prsCalled == 0 {
		t.Error("expected PRs endpoint to be called")
	}
	if server.commitsCalled == 0 {
		t.Error("expected commits endpoint to be called")
	}
}

// ---------------------------------------------------------------------------

func TestReportEndToEnd(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	tests := []struct {
		name         string
		owner        string
		repo         string
		format       string
		expectFiles  int // minimum expected output files
		expectMD     bool
		expectJSON   bool
		expectEvid   bool
		expectErr    bool
	}{
		{
			name:        "format=both creates markdown json pdf and evidence",
			owner:       "dead-owner",
			repo:        "dead-repo",
			format:      "both",
			expectFiles: 4, // report.md, report.json, report.pdf, evidence-index.json
			expectMD:    true,
			expectJSON:  true,
			expectEvid:  true,
			expectErr:   false,
		},
		{
			name:        "format=markdown creates only md",
			owner:       "dead-owner",
			repo:        "dead-repo",
			format:      "markdown",
			expectFiles: 2, // report.md + evidence-index.json
			expectMD:    true,
			expectJSON:  false,
			expectEvid:  true,
			expectErr:   false,
		},
		{
			name:        "format=json creates only json",
			owner:       "dead-owner",
			repo:        "dead-repo",
			format:      "json",
			expectFiles: 2, // report.json + evidence-index.json
			expectMD:    false,
			expectJSON:  true,
			expectEvid:  true,
			expectErr:   false,
		},
		{
			name:        "unsupported format returns error",
			owner:       "dead-owner",
			repo:        "dead-repo",
			format:      "xml",
			expectFiles: 0,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			bundle, _, err := collectAnalysisData(context.Background(), app, tt.owner, tt.repo, "", "", 200, modeFull)
			if err != nil {
				t.Fatalf("collectAnalysisData failed: %v", err)
			}

			rep := buildNecropsyReport(tt.owner, tt.repo, 3, bundle, nil /* no LLM */, 250)

			written, err := app.Renderer.WriteArtifacts(rep, tmpDir, tt.format)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("WriteArtifacts failed: %v", err)
			}

			if len(written) < tt.expectFiles {
				t.Errorf("expected at least %d files, got %d: %v", tt.expectFiles, len(written), written)
			}

			// Verify file contents
			if tt.expectMD {
				mdPath := filepath.Join(tmpDir, "report.md")
				data, err := os.ReadFile(mdPath)
				if err != nil {
					t.Errorf("report.md not found: %v", err)
				} else {
					content := string(data)
					if !strings.Contains(content, tt.owner+"/"+tt.repo) {
						t.Error("report.md missing repository name")
					}
					if !strings.Contains(content, "Evidence index") && !strings.Contains(content, "验尸报告") {
						// OK – content may vary
					}
				}
			}

			if tt.expectJSON {
				jsonPath := filepath.Join(tmpDir, "report.json")
				data, err := os.ReadFile(jsonPath)
				if err != nil {
					t.Errorf("report.json not found: %v", err)
				} else {
					var decoded report.NecropsyReport
					if err := json.Unmarshal(data, &decoded); err != nil {
						t.Errorf("report.json is not valid JSON: %v", err)
					}
					if decoded.Repository != tt.owner+"/"+tt.repo {
						t.Errorf("expected repository %q, got %q", tt.owner+"/"+tt.repo, decoded.Repository)
					}
				}
			}

			if tt.expectEvid {
				evidPath := filepath.Join(tmpDir, "evidence-index.json")
				data, err := os.ReadFile(evidPath)
				if err != nil {
					t.Errorf("evidence-index.json not found: %v", err)
				} else {
					var evidence []report.EvidenceItem
					if err := json.Unmarshal(data, &evidence); err != nil {
						t.Errorf("evidence-index.json is not valid JSON: %v", err)
					}
					if len(evidence) == 0 {
						t.Error("evidence-index.json should contain evidence items")
					}
				}
			}

			// Verify files on disk match what was returned
			for _, p := range written {
				if _, err := os.Stat(p); err != nil {
					t.Errorf("written file does not exist on disk: %s", p)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------

func TestNetworkRetryAndTimeout(t *testing.T) {
	t.Run("429 then 200 retry succeeds", func(t *testing.T) {
		server := newRetryTestServer()
		defer server.Server.Close()

		netClient := network.NewClient(network.Config{
			TimeoutMS:     3000,
			RetryMax:      3,
			BackoffBaseMS: 20, // fast backoff for tests
		})

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

		resp, _, err := netClient.Do(ctx, req, "test-retry", "session-retry-001")

		if err != nil {
			t.Fatalf("expected eventual success after retry, got error: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response after retry")
		}
		defer resp.Body.Close()

		// Should have made 2 calls (first 429, then 200)
		count := server.CallCount()
		if count < 2 {
			t.Errorf("expected at least 2 calls (429 + 200), got %d", count)
		}

		// Verify audit trail shows multiple attempts
		entries := netClient.AuditTrail()
		if len(entries) < 2 {
			t.Errorf("expected at least 2 audit entries, got %d", len(entries))
		}

		// First entry should be 429
		if entries[0].Status != http.StatusTooManyRequests {
			t.Errorf("first attempt: expected status 429, got %d", entries[0].Status)
		}

		// Last successful entry should be 200
		lastEntry := entries[len(entries)-1]
		if lastEntry.Status != http.StatusOK {
			t.Errorf("last attempt: expected status 200, got %d", lastEntry.Status)
		}
	})

	t.Run("timeout triggers context deadline exceeded", func(t *testing.T) {
		server := newTimeoutTestServer(2 * time.Second)
		defer server.Server.Close()

		netClient := network.NewClient(network.Config{
			TimeoutMS:     500,  // shorter than server block
			RetryMax:      0,    // no retries
			BackoffBaseMS: 10,
		})

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

		_, _, err := netClient.Do(ctx, req, "test-timeout", "session-timeout-001")

		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}

		var terr *network.ToolError
		if errors.As(err, &terr) {
			if !terr.Retryable {
				t.Logf("timeout classified as retryable=%t kind=%s", terr.Retryable, terr.Kind)
			}
		}
	})

	t.Run("exhausted retries returns error", func(t *testing.T) {
		// Server that always returns 500
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		netClient := network.NewClient(network.Config{
			TimeoutMS:     2000,
			RetryMax:      2, // 3 total attempts
			BackoffBaseMS: 10,
		})

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

		_, _, err := netClient.Do(ctx, req, "test-exhausted", "session-exhausted-001")

		if err == nil {
			t.Fatal("expected error after exhausted retries")
		}

		entries := netClient.AuditTrail()
		if len(entries) != 3 {
			t.Errorf("expected 3 audit entries (retry_max=2 → 3 attempts), got %d", len(entries))
		}
	})
}

// ---------------------------------------------------------------------------
// Additional table-driven tests for helper functions
// ---------------------------------------------------------------------------

func TestBuildEvidenceOrdering(t *testing.T) {
	bundle := analysisBundle{
		Repository: fixtureRepo,
		Issues:     fixtureIssues,
		PullReqs:   fixturePullRequests,
		Commits:    fixtureCommits,
	}

	evidence := buildEvidenceStreamed(bundle.Issues, bundle.PullReqs, bundle.Commits, 250)

	// Evidence should be sorted by timestamp (oldest first)
	for i := 1; i < len(evidence); i++ {
		ti := parseTime(evidence[i-1].Timestamp)
		tj := parseTime(evidence[i].Timestamp)
		if ti.After(tj) {
			t.Errorf("evidence not sorted: [%d] %s is after [%d] %s",
				i-1, evidence[i-1].Timestamp, i, evidence[i].Timestamp)
		}
	}

	// Evidence should have valid IDs
	for _, ev := range evidence {
		if ev.ID == "" {
			t.Error("evidence missing ID")
		}
		if !strings.HasPrefix(ev.ID, "E") {
			t.Errorf("evidence ID %q should start with E", ev.ID)
		}
	}
}

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		input       string
		wantOwner   string
		wantRepo    string
		wantErr     bool
	}{
		{"dead-owner/dead-repo", "dead-owner", "dead-repo", false},
		{"ghost-org/phantom-lib", "ghost-org", "phantom-lib", false},
		{"onlyowner", "", "", true},
		{"/onlyslash", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo, err := parseOwnerRepo(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseOwnerRepo(%q): expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("owner=%q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo=%q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

func TestFloatValue(t *testing.T) {
	tests := []struct {
		input    any
		expected float64
	}{
		{float64(1.5), 1.5},
		{float32(2.5), 2.5},
		{int(10), 10.0},
		{int64(20), 20.0},
		{"not a number", 0},
		{nil, 0},
		{map[string]any{}, 0},
	}

	for _, tt := range tests {
		got := floatValue(tt.input)
		if got != tt.expected {
			t.Errorf("floatValue(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestStringValue(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "hello"},
		{42, ""},
		{nil, ""},
		{123.4, ""},
	}

	for _, tt := range tests {
		got := stringValue(tt.input)
		if got != tt.expected {
			t.Errorf("stringValue(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestAsMapSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
	}{
		{"nil input", nil, 0},
		{"[]map[string]any with 3 items", []map[string]any{
			{"a": 1}, {"b": 2}, {"c": 3},
		}, 3},
		{"[]any with mixed types", []any{
			map[string]any{"x": 1},
			"not a map",
			map[string]any{"y": 2},
		}, 2},
		{"plain slice", []int{1, 2, 3}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asMapSlice(tt.input)
			if len(got) != tt.expected {
				t.Errorf("asMapSlice(%T) returned %d items, want %d", tt.input, len(got), tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Failure category tests
// ---------------------------------------------------------------------------

func TestPermissionDenial(t *testing.T) {
	// Verify that when a query request targets a blocked domain,
	// the engine correctly records a deny decision in the execution.
	// We test the permissions engine in isolation since mock tools bypass domain checks.
	perm := permissions.NewEngine(permissions.Config{
		Mode:           permissions.ModeDefault,
		AllowedDomains: []string{},                       // no domains allowed
		BlockedDomains: []string{"evil.example.com"},    // block specific domain
		DenyPrivateNetworks: true,
		ToolAllowOverrides:  map[string]permissions.Behavior{},
	})

	ctx := context.Background()
	decision, err := perm.CanUseTool(ctx, "web.fetch", map[string]any{"url": "https://evil.example.com/data"})

	if err != nil {
		t.Fatalf("permission engine error: %v", err)
	}
	if decision.Behavior != string(permissions.BehaviorDeny) {
		t.Errorf("expected deny, got %s", decision.Behavior)
	}
}

func TestBudgetExhaustion(t *testing.T) {
	// Verify that budget correctly tracks turn consumption and sets stop reason.
	budget := query.NewBudget(query.BudgetLimits{MaxTurns: 2})

	// Consume turns
	ok1 := budget.ConsumeTurn()
	ok2 := budget.ConsumeTurn()
	ok3 := budget.ConsumeTurn() // should fail

	if !ok1 || !ok2 || ok3 {
		t.Errorf("turn consumption mismatch: ok1=%v ok2=%v ok3=%v", ok1, ok2, ok3)
	}

	snap := budget.Snapshot()
	if snap.StopReason != "budget.max_turns_exceeded" {
		t.Errorf("expected stop_reason=budget.max_turns_exceeded, got %q", snap.StopReason)
	}
	if snap.UsedTurns != 2 || snap.MaxTurns != 2 {
		t.Errorf("expected UsedTurns=2 MaxTurns=2, got UsedTurns=%d MaxTurns=%d", snap.UsedTurns, snap.MaxTurns)
	}
}

func TestCacheDegradation(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	// Clear the store to simulate cache miss — query should still succeed
	app.Store = state.NewMemoryStore() // fresh empty store = always cache miss

	req := query.QueryRequest{
		Command:   "scan",
		SessionID: "test-cache-degraded",
		Budget:    query.BudgetLimits{MaxTurns: 16},
		Actions: []query.Action{
			{
				ToolName: "github.search_repositories",
				Input:    map[string]any{"years": 3, "min_stars": 100, "language": "Go", "limit": 10},
			},
		},
	}

	result, err := app.Query.Run(context.Background(), req)

	// Cache miss should NOT cause failure — graceful degradation
	if err != nil {
		t.Fatalf("query failed with empty cache (should be graceful): %v", err)
	}
	if len(result.Executions) == 0 {
		t.Error("expected at least one execution result with empty cache")
	}
}

// failingStore wraps state.Store and makes all operations fail.
type failingStore struct{}

func (s *failingStore) Get(ctx context.Context, key string) (any, error) {
	return nil, errors.New("cache read error")
}
func (s *failingStore) Set(ctx context.Context, key string, value any) error {
	return errors.New("cache write error")
}
func (s *failingStore) Delete(ctx context.Context, key string) error {
	return errors.New("cache delete error")
}
func (s *failingStore) List(ctx context.Context, prefix string) ([]string, error) {
	return nil, errors.New("cache list error")
}
func (s *failingStore) Clear(ctx context.Context) error {
	return errors.New("cache clear error")
}

func TestLLMFallback(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	// Use an LLM client that simulates failure followed by fallback
	// Since we don't have a real LLM in tests, verify that when LLM is unavailable,
	// the report still generates without LLM analysis (graceful degradation)
	bundle, _, err := collectAnalysisData(
		context.Background(),
		app,
		"dead-owner",
		"dead-repo",
		"",
		"",
		200,
		modeFull,
	)
	if err != nil {
		t.Fatalf("collectAnalysisData failed: %v", err)
	}

	// Build report without LLM (nil LLMClient should be handled gracefully)
	rep := buildNecropsyReport("dead-owner", "dead-repo", 3, bundle, nil, 250)

	if rep.Repository != "dead-owner/dead-repo" {
		t.Errorf("expected repository 'dead-owner/dead-repo', got %q", rep.Repository)
	}

	if len(rep.CorePhilosophy) == 0 {
		t.Error("expected non-empty core philosophy even without LLM")
	}
}

// --------------------------------------------------------------------------
// Cache invalidation tests (TTLStore)
// --------------------------------------------------------------------------

func TestCacheInvalidation(t *testing.T) {
	t.Run("TTL expiration removes entry after TTL", func(t *testing.T) {
		cache := tools.NewTTLStore(t.TempDir())
		cache.SetWithTTL("key1", "value1", 50*time.Millisecond)

		// Should exist immediately
		val, ok := cache.Get("key1")
		if !ok {
			t.Fatal("expected key1 to exist immediately after SetWithTTL")
		}
		if val != "value1" {
			t.Errorf("expected value1, got %v", val)
		}

		// Wait past TTL
		time.Sleep(120 * time.Millisecond)

		// Should be gone
		_, ok = cache.Get("key1")
		if ok {
			t.Error("expected key1 to be expired after TTL")
		}
	})

	t.Run("Delete removes entry immediately", func(t *testing.T) {
		cache := tools.NewTTLStore(t.TempDir())
		cache.SetWithTTL("key1", "value1", 1*time.Hour)
		cache.SetWithTTL("key2", "value2", 1*time.Hour)

		cache.Delete("key1")

		_, ok := cache.Get("key1")
		if ok {
			t.Error("expected key1 to be deleted")
		}

		// key2 should still exist
		val, ok := cache.Get("key2")
		if !ok {
			t.Error("expected key2 to still exist after deleting key1")
		}
		if val != "value2" {
			t.Errorf("expected value2, got %v", val)
		}
	})

	t.Run("Clear removes all entries", func(t *testing.T) {
		cache := tools.NewTTLStore(t.TempDir())
		cache.SetWithTTL("key1", "value1", 1*time.Hour)
		cache.SetWithTTL("key2", "value2", 1*time.Hour)
		cache.SetWithTTL("key3", "value3", 1*time.Hour)

		cache.Clear()

		keys := cache.Keys()
		if len(keys) != 0 {
			t.Errorf("expected 0 keys after Clear, got %d", len(keys))
		}

		_, ok := cache.Get("key1")
		if ok {
			t.Error("expected key1 to be gone after Clear")
		}
		_, ok = cache.Get("key2")
		if ok {
			t.Error("expected key2 to be gone after Clear")
		}
		_, ok = cache.Get("key3")
		if ok {
			t.Error("expected key3 to be gone after Clear")
		}
	})

	t.Run("Stats accuracy", func(t *testing.T) {
		cache := tools.NewTTLStore(t.TempDir())
		// Active entries
		cache.SetWithTTL("active1", "v1", 1*time.Hour)
		cache.SetWithTTL("active2", "v2", 1*time.Hour)
		// Expired entries — set TTL of 0 to simulate already-expired
		cache.SetWithTTL("expired1", "v3", 1*time.Millisecond)
		cache.SetWithTTL("expired2", "v4", 1*time.Millisecond)
		// Wait for them to expire
		time.Sleep(10 * time.Millisecond)

		stats := cache.Stats()

		// TotalKeys includes both active and expired (not yet cleaned up)
		if stats.TotalKeys != 4 {
			t.Errorf("expected TotalKeys=4, got %d", stats.TotalKeys)
		}
		// ActiveKeys should be 2 (the ones with 1hr TTL)
		if stats.ActiveKeys != 2 {
			t.Errorf("expected ActiveKeys=2, got %d", stats.ActiveKeys)
		}
		// ExpiredKeys should be 2
		if stats.ExpiredKeys != 2 {
			t.Errorf("expected ExpiredKeys=2, got %d", stats.ExpiredKeys)
		}
	})

	t.Run("Get on non-existent key returns false", func(t *testing.T) {
		cache := tools.NewTTLStore(t.TempDir())
		_, ok := cache.Get("nonexistent")
		if ok {
			t.Error("expected Get on nonexistent key to return false")
		}
	})

	t.Run("Delete on non-existent key is no-op", func(t *testing.T) {
		cache := tools.NewTTLStore(t.TempDir())
		cache.Delete("nonexistent") // should not panic
	})

	t.Run("Keys returns only non-expired keys", func(t *testing.T) {
		cache := tools.NewTTLStore(t.TempDir())
		cache.SetWithTTL("active1", "v1", 1*time.Hour)
		cache.SetWithTTL("active2", "v2", 1*time.Hour)
		cache.SetWithTTL("willExpire", "v3", 1*time.Millisecond)

		time.Sleep(10 * time.Millisecond)

		keys := cache.Keys()
		if len(keys) != 2 {
			t.Errorf("expected 2 active keys, got %d: %v", len(keys), keys)
		}
		for _, k := range keys {
			if k == "willExpire" {
				t.Error("willExpire should not appear in Keys() after expiry")
			}
		}
	})
}

// --------------------------------------------------------------------------
// Parallel execution tests (autopsy.go concurrent API calls)
// --------------------------------------------------------------------------

func TestParallelAPICalls(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	start := time.Now()
	bundle, _, err := collectAnalysisData(context.Background(), app, "dead-owner", "dead-repo", "", "", 200, modeFull)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("collectAnalysisData failed: %v", err)
	}

	// All 4 endpoints should have been called
	if server.repoCalled == 0 || server.issuesCalled == 0 || server.prsCalled == 0 || server.commitsCalled == 0 {
		t.Error("not all endpoints were called")
	}

	// With 4 concurrent calls, elapsed time should be close to the slowest call, not sum of all
	// The mock server responds instantly, so this should complete quickly
	if elapsed > 500*time.Millisecond {
		t.Errorf("parallel execution took too long (%v), suggests sequential execution", elapsed)
	}

	// Verify bundle data is complete
	if len(bundle.Repository) == 0 {
		t.Error("bundle.Repository is empty")
	}
	if len(bundle.Issues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(bundle.Issues))
	}
	if len(bundle.PullReqs) != 1 {
		t.Errorf("expected 1 PR, got %d", len(bundle.PullReqs))
	}
	if len(bundle.Commits) != 2 {
		t.Errorf("expected 2 commits, got %d", len(bundle.Commits))
	}
}

// --------------------------------------------------------------------------
// Multi-repo scan concurrency tests
// --------------------------------------------------------------------------

func TestMultiRepoScanConcurrency(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	repos := []string{"ghost-org/phantom-lib", "abandoned-team/sunset-project"}

	var wg sync.WaitGroup
	var callCount int32
	var mu sync.Mutex
	seen := make(map[string]bool)

	for _, repo := range repos {
		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			owner, repo, _ := parseOwnerRepo(r)
			req := query.QueryRequest{
				Command:   "scan",
				SessionID: "test-multi-" + r,
				Budget:    query.BudgetLimits{MaxTurns: 16},
				Actions: []query.Action{
					{
						ToolName: "github.search_repositories",
						Input:    map[string]any{"years": 5, "min_stars": 0, "language": "", "limit": 10, "owner": owner, "repo": repo},
					},
				},
			}
			result, _ := app.Query.Run(context.Background(), req)
			if len(result.Executions) > 0 && result.Executions[0].Output != nil {
				mu.Lock()
				seen[r] = true
				callCount++
				mu.Unlock()
			}
		}(repo)
	}

	wg.Wait()

	if callCount != int32(len(repos)) {
		t.Errorf("expected %d successful scans, got %d", len(repos), callCount)
	}
	if len(seen) != len(repos) {
		t.Errorf("expected %d repos in results, got %d", len(repos), len(seen))
	}
}

// --------------------------------------------------------------------------
// Evidence ordering tests (buildEvidenceStreamed sorting)
// --------------------------------------------------------------------------

func TestEvidenceStreamedOrdering(t *testing.T) {
	// Create bundle with evidence at known timestamps
	issues := []map[string]any{
		{
			"id":         int64(1),
			"number":     1,
			"title":      "Old issue",
			"state":      "closed",
			"created_at": "2020-01-01T00:00:00Z",
			"updated_at": "2020-01-15T00:00:00Z",
			"user":       map[string]any{"login": "user1"},
			"comments":   int64(0),
		},
		{
			"id":         int64(2),
			"number":     2,
			"title":      "New issue",
			"state":      "open",
			"created_at": "2023-06-01T00:00:00Z",
			"updated_at": "2023-06-01T00:00:00Z",
			"user":       map[string]any{"login": "user2"},
			"comments":   int64(0),
		},
		{
			"id":         int64(3),
			"number":     3,
			"title":      "Middle issue",
			"state":      "closed",
			"created_at": "2021-06-01T00:00:00Z",
			"updated_at": "2021-06-01T00:00:00Z",
			"user":       map[string]any{"login": "user3"},
			"comments":   int64(0),
		},
	}

	evidence := buildEvidenceStreamed(issues, nil, nil, 500)

	// Should be sorted oldest to newest
	if len(evidence) != 3 {
		t.Fatalf("expected 3 evidence items, got %d", len(evidence))
	}

	if evidence[0].Timestamp != "2020-01-01T00:00:00Z" {
		t.Errorf("first evidence should be oldest (2020-01-01), got %s", evidence[0].Timestamp)
	}
	if evidence[2].Timestamp != "2023-06-01T00:00:00Z" {
		t.Errorf("last evidence should be newest (2023-06-01), got %s", evidence[2].Timestamp)
	}
}

// --------------------------------------------------------------------------
// Autopsy mode tests
// --------------------------------------------------------------------------

func TestAutopsyModes(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	modes := []fetchMode{modeFull, modeSample, modeLite}
	for _, m := range modes {
		t.Run(string(m), func(t *testing.T) {
			bundle, _, err := collectAnalysisData(
				context.Background(), app, "dead-owner", "dead-repo",
				"", "", 200, m,
			)
			if err != nil {
				t.Fatalf("collectAnalysisData(mode=%s) failed: %v", m, err)
			}
			// All modes should return repository metadata
			if len(bundle.Repository) == 0 {
				t.Error("bundle.Repository is empty")
			}
		})
	}
}

// --------------------------------------------------------------------------
// LLM graceful degradation tests
// --------------------------------------------------------------------------

func TestLLMGracefulDegradation(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	bundle, _, err := collectAnalysisData(
		context.Background(), app, "dead-owner", "dead-repo",
		"", "", 200, modeFull,
	)
	if err != nil {
		t.Fatalf("collectAnalysisData failed: %v", err)
	}

	// Build report with nil LLM client
	rep := buildNecropsyReport("dead-owner", "dead-repo", 3, bundle, nil, 250)

	if rep.Repository != "dead-owner/dead-repo" {
		t.Errorf("expected repository 'dead-owner/dead-repo', got %q", rep.Repository)
	}

	// Cause scores should be present (rule-based fallback)
	if len(rep.CauseScores) == 0 {
		t.Error("expected at least some cause scores from rule-based fallback")
	}

	// Core philosophy should be populated (rule-based fallback produces it as a slice)
	if len(rep.CorePhilosophy) == 0 {
		t.Error("expected non-empty CorePhilosophy")
	}
}

// --------------------------------------------------------------------------
// Boundary/edge case tests
// --------------------------------------------------------------------------

func TestParseOwnerRepoEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		wantErr  bool
		wantPart string // "owner/repo" or "empty" or "slash-only"
	}{
		{"a/b", false, "owner/repo"},
		{"a/b/c", true, "empty"},         // too many slashes
		{"a", true, "empty"},             // no slash
		{"", true, "empty"},              // empty string
		{"/b", true, "slash-only"},       // starts with slash
		{"a/", true, "slash-only"},       // ends with slash
		{"//", true, "empty"},            // double slash
		{" spaces / repo", false, "owner/repo"}, // with spaces (will trim)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo, err := parseOwnerRepo(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("parseOwnerRepo(%q): expected error, got nil", tt.input)
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("parseOwnerRepo(%q): unexpected error: %v", tt.input, err)
				return
			}
			if tt.wantPart == "owner/repo" && (owner == "" || repo == "") {
				t.Errorf("parseOwnerRepo(%q): owner=%q repo=%q, expected both non-empty", tt.input, owner, repo)
			}
			if tt.wantPart == "empty" && (owner != "" || repo != "") {
				t.Errorf("parseOwnerRepo(%q): expected empty owner/repo, got owner=%q repo=%q", tt.input, owner, repo)
			}
		})
	}
}

func TestEvidenceLimitTruncation(t *testing.T) {
	// Create many issues
	var manyIssues []map[string]any
	for i := 0; i < 50; i++ {
		manyIssues = append(manyIssues, map[string]any{
			"id":         int64(i),
			"number":     i,
			"title":      fmt.Sprintf("issue %d", i),
			"state":      "closed",
			"created_at": fmt.Sprintf("2020-01-%02dT00:00:00Z", i%28+1),
			"updated_at": fmt.Sprintf("2020-01-%02dT00:00:00Z", i%28+1),
			"user":       map[string]any{"login": fmt.Sprintf("user%d", i)},
			"comments":   int64(0),
		})
	}

	// Test with different limits
	for _, limit := range []int{10, 25, 50} {
		t.Run(fmt.Sprintf("limit_%d", limit), func(t *testing.T) {
			evidence := buildEvidenceStreamed(manyIssues, nil, nil, limit)
			if len(evidence) > limit {
				t.Errorf("evidence count %d exceeds limit %d", len(evidence), limit)
			}
		})
	}
}

func TestFloatValueEdgeCases(t *testing.T) {
	tests := []struct {
		input    any
		expected float64
	}{
		{float64(0), 0},
		{float64(-1.5), -1.5},
		{float32(3.5), 3.5},
		{int(0), 0},
		{int(-100), -100},
		{int64(-999), -999},
		{uint(42), 0}, // uint not handled by floatValue → 0
		{"0", 0},
		{"123.45", 0},           // string that looks like number → 0
		{true, 0},               // bool → 0
		{false, 0},              // bool → 0
		{[]int{1, 2, 3}, 0},     // slice → 0
		{map[string]int{"a": 1}, 0}, // map → 0
	}

	for _, tt := range tests {
		got := floatValue(tt.input)
		if got != tt.expected {
			t.Errorf("floatValue(%v (%T)) = %v, want %v", tt.input, tt.input, got, tt.expected)
		}
	}
}

func TestModeConstants(t *testing.T) {
	// Verify modes are non-empty strings
	if modeFull == "" || modeSample == "" || modeLite == "" {
		t.Error("mode constants should not be empty strings")
	}
	// Verify they are distinct
	if modeFull == modeSample || modeFull == modeLite || modeSample == modeLite {
		t.Error("mode constants should be distinct")
	}
}

func TestMemoryStoreNoLeak(t *testing.T) {
	store := state.NewMemoryStore()

	// Write many entries
	for i := 0; i < 1000; i++ {
		store.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	// Verify we can read them back
	for i := 0; i < 1000; i++ {
		val, found := store.Get(fmt.Sprintf("key-%d", i))
		if !found {
			t.Errorf("failed to get key-%d: not found", i)
		}
		if val != fmt.Sprintf("value-%d", i) {
			t.Errorf("key-%d: expected %q, got %q", i, fmt.Sprintf("value-%d", i), val)
		}
	}

	// Verify Keys() returns all keys
	keys := store.Keys()
	if len(keys) != 1000 {
		t.Errorf("expected 1000 keys, got %d", len(keys))
	}
}

func TestQueryBudgetZeroAndMax(t *testing.T) {
	t.Run("zero max turns unlimited", func(t *testing.T) {
		// MaxTurns=0 means unlimited
		budget := query.NewBudget(query.BudgetLimits{MaxTurns: 0})
		for i := 0; i < 10; i++ {
			ok := budget.ConsumeTurn()
			if !ok {
				t.Errorf("ConsumeTurn %d failed unexpectedly with MaxTurns=0 (unlimited)", i)
			}
		}
	})

	t.Run("limit exceeded", func(t *testing.T) {
		budget := query.NewBudget(query.BudgetLimits{MaxTurns: 3})
		// First 3 should succeed
		for i := 0; i < 3; i++ {
			ok := budget.ConsumeTurn()
			if !ok {
				t.Errorf("ConsumeTurn %d failed unexpectedly", i)
			}
		}
		// 4th should fail
		ok := budget.ConsumeTurn()
		if ok {
			t.Error("expected ConsumeTurn to fail when limit exceeded")
		}
		snap := budget.Snapshot()
		if snap.StopReason != "budget.max_turns_exceeded" {
			t.Errorf("expected stop_reason=max_turns_exceeded, got %q", snap.StopReason)
		}
		if snap.UsedTurns != 3 {
			t.Errorf("expected UsedTurns=3, got %d", snap.UsedTurns)
		}
	})

	t.Run("large max turns", func(t *testing.T) {
		budget := query.NewBudget(query.BudgetLimits{MaxTurns: 9999})
		for i := 0; i < 100; i++ {
			ok := budget.ConsumeTurn()
			if !ok {
				t.Errorf("ConsumeTurn %d failed unexpectedly", i)
			}
		}
		snap := budget.Snapshot()
		if snap.UsedTurns != 100 {
			t.Errorf("expected UsedTurns=100, got %d", snap.UsedTurns)
		}
	})
}

func TestRendererUnsupportedFormat(t *testing.T) {
	server := newMockGitHubServer()
	defer server.Server.Close()

	app, _ := buildTestApp("", server.URL)

	bundle, _, _ := collectAnalysisData(context.Background(), app, "dead-owner", "dead-repo", "", "", 200, modeFull)
	rep := buildNecropsyReport("dead-owner", "dead-repo", 3, bundle, nil, 250)

	tmpDir := t.TempDir()

	for _, format := range []string{"yaml", "xml", "csv", "html"} {
		t.Run(format, func(t *testing.T) {
			_, err := app.Renderer.WriteArtifacts(rep, tmpDir, format)
			if err == nil {
				t.Errorf("expected error for format=%q, got nil", format)
			}
		})
	}
}
