package commands

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/repo-necromancer/necro/internal/llm"
	"github.com/repo-necromancer/necro/internal/query"
	"github.com/repo-necromancer/necro/internal/report"
)

// Fetch mode for memory-efficient processing
type fetchMode string

const (
	modeFull   fetchMode = "full"
	modeSample fetchMode = "sample"
	modeLite   fetchMode = "lite"
)

type analysisBundle struct {
	Repository  map[string]any
	Issues      []map[string]any
	PullReqs    []map[string]any
	Commits     []map[string]any
	QueryResult query.QueryResult
}

func newAutopsyCommand() *cobra.Command {
	var years int
	var since string
	var until string
	var maxItems int
	var maxEvidence int
	var mode string

	cmd := &cobra.Command{
		Use:   "autopsy <owner/repo>",
		Short: "Perform repository death-cause analysis",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if years <= 0 {
				return fmt.Errorf("--years must be > 0")
			}
			fetchMode := fetchMode(mode)
			if fetchMode != modeFull && fetchMode != modeSample && fetchMode != modeLite {
				return fmt.Errorf("--mode must be one of: full, sample, lite")
			}
			if maxEvidence <= 0 {
				maxEvidence = 250
			}
			if maxEvidence > 2000 {
				maxEvidence = 2000
			}
			app, err := appFromCmd(cmd)
			if err != nil {
				return err
			}

			owner, repo, err := parseOwnerRepo(args[0])
			if err != nil {
				return err
			}
			bundle, totalCount, err := collectAnalysisData(cmd.Context(), app, owner, repo, since, until, maxItems, fetchMode)
			if err != nil {
				return err
			}
			autopsyReport := buildNecropsyReport(owner, repo, years, bundle, app.LLMClient, maxEvidence)

			fmt.Fprintf(cmd.OutOrStdout(), "Autopsy for %s/%s\n", owner, repo)
			fmt.Fprintf(cmd.OutOrStdout(), "Stars: %d | Last commit: %s\n", autopsyReport.Stars, autopsyReport.LastCommitAt)
			fmt.Fprintln(cmd.OutOrStdout(), "Cause scores:")
			for _, c := range autopsyReport.CauseScores {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s score=%.2f confidence=%.2f\n", c.Label, c.Score, c.Confidence)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Evidence indexed: %d\n", len(autopsyReport.Evidence))

			// Print sampling summary for sample mode
			if fetchMode == modeSample {
				fmt.Fprintf(cmd.OutOrStdout(), "Mode: sample (memory-efficient, sampled %d recent commits + recent 2yr issues/PRs)\n", maxItems)
				fmt.Fprintf(cmd.OutOrStdout(), "Evidence indexed: %d (capped from ~%d total)\n", len(autopsyReport.Evidence), totalCount)
				fmt.Fprintln(cmd.OutOrStdout(), "Sampling bias: Recent activity bias — historical patterns may be underrepresented")
			} else if fetchMode == modeLite {
				fmt.Fprintf(cmd.OutOrStdout(), "Mode: lite (repo metadata only + recent 30 days, rule-based scoring)\n")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&years, "years", 0, "Inactivity threshold context in years")
	cmd.Flags().StringVar(&since, "since", "", "Optional evidence lower bound (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Optional evidence upper bound (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().IntVar(&maxItems, "max-items", 200, "Maximum issues/prs/commits to fetch")
	cmd.Flags().IntVar(&maxEvidence, "max-evidence", 250, "Maximum evidence items to collect (max 2000)")
	cmd.Flags().StringVar(&mode, "mode", "full", "Fetch mode: full, sample, or lite")
	_ = cmd.MarkFlagRequired("years")
	return cmd
}

func collectAnalysisData(ctx context.Context, app *App, owner, repo, since, until string, maxItems int, mode fetchMode) (analysisBundle, int, error) {
	// For lite mode, only fetch repository metadata
	if mode == modeLite {
		req := query.QueryRequest{
			Command:   "autopsy",
			SessionID: app.SessionID,
			Budget: query.BudgetLimits{
				MaxTurns:  app.Config.Query.MaxTurns,
				MaxTokens: app.Config.Query.MaxTokens,
				MaxCost:   app.Config.Query.MaxCost,
			},
			Actions: []query.Action{
				{
					ToolName: "github.repository",
					Input:    map[string]any{"owner": owner, "repo": repo},
				},
			},
		}
		res, err := app.Query.Run(ctx, req)
		if err != nil {
			return analysisBundle{}, 0, err
		}
		bundle := analysisBundle{QueryResult: res}
		for _, ex := range res.Executions {
			if ex.Error != "" {
				continue
			}
			if ex.ToolName == "github.repository" {
				if repoObj, ok := ex.Output["repository"].(map[string]any); ok {
					bundle.Repository = repoObj
				}
			}
		}
		if len(bundle.Repository) == 0 {
			return analysisBundle{}, 0, fmt.Errorf("failed to fetch repository metadata")
		}
		return bundle, 0, nil
	}

	// For sample mode, calculate "since" date for 2 years ago if not provided
	sinceDate := since
	if mode == modeSample && sinceDate == "" {
		sinceDate = time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
	}

	// Build actions based on mode
	actions := []query.Action{
		{
			ToolName: "github.repository",
			Input:    map[string]any{"owner": owner, "repo": repo},
		},
		{
			ToolName: "github.issues",
			Input: map[string]any{
				"owner":     owner,
				"repo":      repo,
				"since":     sinceDate,
				"until":     until,
				"max_items": maxItems,
			},
		},
		{
			ToolName: "github.pull_requests",
			Input: map[string]any{
				"owner":     owner,
				"repo":      repo,
				"since":     sinceDate,
				"until":     until,
				"max_items": maxItems,
			},
		},
		{
			ToolName: "github.commits",
			Input: map[string]any{
				"owner":     owner,
				"repo":      repo,
				"since":     sinceDate,
				"until":     until,
				"max_items": maxItems,
			},
		},
	}

	// For sample mode, use increased max_items but with since date filter
	if mode == modeSample {
		// Override max_items to 500 for commits in sample mode
		actions[3].Input["max_items"] = 500
	}

	req := query.QueryRequest{
		Command:   "autopsy",
		SessionID: app.SessionID,
		Budget: query.BudgetLimits{
			MaxTurns:  app.Config.Query.MaxTurns,
			MaxTokens: app.Config.Query.MaxTokens,
			MaxCost:   app.Config.Query.MaxCost,
		},
		Actions: actions,
	}
	res, err := app.Query.Run(ctx, req)
	if err != nil {
		return analysisBundle{}, 0, err
	}
	bundle := analysisBundle{QueryResult: res}
	totalCount := 0
	for _, ex := range res.Executions {
		if ex.Error != "" {
			continue
		}
		switch ex.ToolName {
		case "github.repository":
			if repoObj, ok := ex.Output["repository"].(map[string]any); ok {
				bundle.Repository = repoObj
			}
		case "github.issues":
			bundle.Issues = asMapSlice(ex.Output["issues"])
			totalCount += len(bundle.Issues)
		case "github.pull_requests":
			bundle.PullReqs = asMapSlice(ex.Output["pull_requests"])
			totalCount += len(bundle.PullReqs)
		case "github.commits":
			bundle.Commits = asMapSlice(ex.Output["commits"])
			totalCount += len(bundle.Commits)
		}
	}
	if len(bundle.Repository) == 0 {
		return analysisBundle{}, 0, fmt.Errorf("failed to fetch repository metadata")
	}
	return bundle, totalCount, nil
}

func buildNecropsyReport(owner, repo string, years int, data analysisBundle, llmClient *llm.Client, maxEvidence int) report.NecropsyReport {
	evidence := buildEvidenceStreamed(data.Issues, data.PullReqs, data.Commits, maxEvidence)
	// For lite mode, skip LLM cause scoring and use rule-based only
	var causes []report.CauseScore
	if len(evidence) == 0 {
		causes = scoreCausesRuleBased(evidence)
	} else {
		causes = scoreCauses(evidence, llmClient)
	}
	timeline := buildTimeline(data, evidence)

	return report.NecropsyReport{
		Repository:          owner + "/" + repo,
		SnapshotDate:        time.Now().UTC().Format(time.RFC3339),
		DeathThresholdYears: years,
		Stars:               int(floatValue(data.Repository["stars"])),
		LastCommitAt:        stringValue(data.Repository["pushed_at"]),
		CorePhilosophy:      inferCorePhilosophy(data.Repository),
		Timeline:            timeline,
		CauseScores:         causes,
		Evidence:            evidence,
		QueryMetadata: report.QueryMetadata{
			SessionID:  data.QueryResult.SessionID,
			StopReason: data.QueryResult.StopReason,
			UsedTurns:  data.QueryResult.Budget.UsedTurns,
			MaxTurns:   data.QueryResult.Budget.MaxTurns,
			UsedTokens: data.QueryResult.Budget.UsedTokens,
			MaxTokens:  data.QueryResult.Budget.MaxTokens,
			UsedCost:   data.QueryResult.Budget.UsedCost,
			MaxCost:    data.QueryResult.Budget.MaxCost,
			Partial:    data.QueryResult.Partial,
		},
	}
}

// evidenceHeap is a min-heap of EvidenceItem by Relevance score
type evidenceHeap []report.EvidenceItem

func (h evidenceHeap) Len() int { return len(h) }
func (h evidenceHeap) Less(i, j int) bool { return h[i].Relevance < h[j].Relevance }
func (h evidenceHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *evidenceHeap) Push(x any) {
	*h = append(*h, x.(report.EvidenceItem))
}

func (h *evidenceHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// buildEvidenceStreamed uses a min-heap to keep only the top N items by relevance
// This avoids holding all items in memory simultaneously
func buildEvidenceStreamed(issues, prs, commits []map[string]any, maxItems int) []report.EvidenceItem {
	if maxItems <= 0 {
		maxItems = 250
	}

	h := &evidenceHeap{}
	heap.Init(h)
	id := 1

	// Process issues
	for _, issue := range issues {
		item := report.EvidenceItem{
			ID:        fmt.Sprintf("E%03d", id),
			Type:      "issue",
			URL:       stringValue(issue["url"]),
			Title:     stringValue(issue["title"]),
			Timestamp: stringValue(issue["created_at"]),
			Summary:   trimText(stringValue(issue["body"]), 200),
			Relevance: relevanceScore(stringValue(issue["title"]) + " " + stringValue(issue["body"])),
		}
		id++

		if h.Len() < maxItems {
			heap.Push(h, item)
		} else if item.Relevance > (*h)[0].Relevance {
			heap.Pop(h)
			heap.Push(h, item)
		}
	}

	// Process pull requests
	for _, pr := range prs {
		item := report.EvidenceItem{
			ID:        fmt.Sprintf("E%03d", id),
			Type:      "pr",
			URL:       stringValue(pr["url"]),
			Title:     stringValue(pr["title"]),
			Timestamp: stringValue(pr["created_at"]),
			Summary:   trimText(stringValue(pr["body"]), 200),
			Relevance: relevanceScore(stringValue(pr["title"]) + " " + stringValue(pr["body"])),
		}
		id++

		if h.Len() < maxItems {
			heap.Push(h, item)
		} else if item.Relevance > (*h)[0].Relevance {
			heap.Pop(h)
			heap.Push(h, item)
		}
	}

	// Process commits
	for _, commit := range commits {
		item := report.EvidenceItem{
			ID:        fmt.Sprintf("E%03d", id),
			Type:      "commit",
			URL:       stringValue(commit["url"]),
			Title:     trimText(stringValue(commit["message"]), 80),
			Timestamp: stringValue(commit["date"]),
			Summary:   trimText(stringValue(commit["message"]), 200),
			Relevance: relevanceScore(stringValue(commit["message"])),
		}
		id++

		if h.Len() < maxItems {
			heap.Push(h, item)
		} else if item.Relevance > (*h)[0].Relevance {
			heap.Pop(h)
			heap.Push(h, item)
		}
	}

	// Extract and sort by timestamp ascending (oldest first for timeline)
	events := make([]report.EvidenceItem, 0, h.Len())
	for h.Len() > 0 {
		events = append(events, heap.Pop(h).(report.EvidenceItem))
	}

	sort.SliceStable(events, func(i, j int) bool {
		ti := parseTime(events[i].Timestamp)
		tj := parseTime(events[j].Timestamp)
		return ti.Before(tj)
	})

	return events
}

func parseOwnerRepo(v string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(v), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository %q, expected owner/repo", v)
	}
	return parts[0], parts[1], nil
}

func scoreCauses(evidence []report.EvidenceItem, llmClient *llm.Client) []report.CauseScore {
	fallback := scoreCausesRuleBased(evidence)
	if llmClient == nil || len(evidence) == 0 {
		return fallback
	}
	llmScores, err := scoreCausesWithLLM(llmClient, evidence, fallback)
	if err != nil {
		log.Printf("scoreCauses: LLM failed, using rule-based fallback: %v", err)
		return fallback
	}
	hasNonZero := false
	for _, cs := range llmScores {
		if cs.Score > 0 {
			hasNonZero = true
			break
		}
	}
	if hasNonZero {
		log.Printf("scoreCauses: LLM succeeded with non-zero scores for %d causes", len(llmScores))
	}
	return llmScores
}

func scoreCausesRuleBased(evidence []report.EvidenceItem) []report.CauseScore {
	taxonomy := map[string][]string{
		"maintainer_burnout":      {"burnout", "maintainer", "no time", "abandoned", "overwhelmed"},
		"architecture_debt":       {"rewrite", "tech debt", "legacy", "refactor", "hard to maintain"},
		"ecosystem_displacement":  {"superseded", "replaced", "new framework", "migration"},
		"security_trust_collapse": {"security", "cve", "vulnerability", "exploit", "compromise"},
		"governance_failure":      {"governance", "maintainer conflict", "bus factor", "decision deadlock"},
		"funding_absence":         {"funding", "sponsor", "sustain", "money", "unpaid"},
		"scope_drift":             {"scope creep", "too many features", "roadmap chaos", "drift"},
	}

	out := make([]report.CauseScore, 0, len(taxonomy))
	for label, keywords := range taxonomy {
		hits := 0
		refs := make([]string, 0, 6)
		for _, e := range evidence {
			text := strings.ToLower(e.Title + " " + e.Summary)
			for _, kw := range keywords {
				if strings.Contains(text, kw) {
					hits++
					if len(refs) < 6 {
						refs = append(refs, e.ID)
					}
					break
				}
			}
		}
		score := math.Min(1, float64(hits)/5.0)
		confidence := 0.2
		if hits > 0 {
			confidence = math.Min(1, 0.45+float64(hits)/15.0)
		}
		counter := []string{}
		if hits == 0 {
			counter = append(counter, "No direct evidence in indexed artifacts.")
		}
		out = append(out, report.CauseScore{
			Label:           label,
			Score:           round2(score),
			Confidence:      round2(confidence),
			EvidenceRefs:    refs,
			CounterEvidence: counter,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
}

type llmCauseScoreResponse struct {
	Causes []struct {
		Label           string   `json:"label"`
		Score           float64  `json:"score"`
		Confidence      float64  `json:"confidence"`
		EvidenceRefs    []string `json:"evidence_refs"`
		CounterEvidence []string `json:"counter_evidence"`
	} `json:"causes"`
}

func scoreCausesWithLLM(llmClient *llm.Client, evidence []report.EvidenceItem, fallback []report.CauseScore) ([]report.CauseScore, error) {
	evidenceIDs := make(map[string]struct{}, len(evidence))
	evidenceLines := make([]string, 0, len(evidence))
	for i, item := range evidence {
		evidenceIDs[item.ID] = struct{}{}
		if i >= 120 {
			break
		}
		evidenceLines = append(evidenceLines,
			fmt.Sprintf("%s | %s | %s | %s", item.ID, item.Type, trimText(item.Title, 100), trimText(item.Summary, 220)),
		)
	}
	if len(evidenceLines) == 0 {
		return fallback, nil
	}

	labels := make([]string, 0, len(fallback))
	for _, cause := range fallback {
		labels = append(labels, cause.Label)
	}
	systemPrompt := "You are a repository failure analyst. Return only valid JSON."
	userPrompt := fmt.Sprintf(
		"Analyze repository evidence and score causes.\nUse only these labels: %s\n"+
			"Output exactly this JSON object shape:\n"+
			"{\"causes\":[{\"label\":\"...\",\"score\":0..1,\"confidence\":0..1,\"evidence_refs\":[\"E001\"],\"counter_evidence\":[\"...\"]}]}\n"+
			"Evidence:\n%s",
		strings.Join(labels, ", "),
		strings.Join(evidenceLines, "\n"),
	)

	raw, err := llmClient.Chat(systemPrompt, userPrompt)
	if err != nil {
		log.Printf("scoreCausesWithLLM: LLM Chat error: %v", err)
		return nil, err
	}
	log.Printf("scoreCausesWithLLM: raw LLM response (first 500 chars): %q", trimText(raw, 500))
	raw = extractJSONObject(raw)
	log.Printf("scoreCausesWithLLM: after extractJSONObject: %q", trimText(raw, 500))

	var parsed llmCauseScoreResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		log.Printf("scoreCausesWithLLM: JSON unmarshal failed: %v, raw was: %q", err, trimText(raw, 200))
		return nil, fmt.Errorf("parse llm cause scores: %w", err)
	}
	if len(parsed.Causes) == 0 {
		log.Printf("scoreCausesWithLLM: parsed.Causes is empty, raw: %q", trimText(raw, 300))
		return nil, fmt.Errorf("llm returned no causes")
	}
	log.Printf("scoreCausesWithLLM: successfully parsed %d causes", len(parsed.Causes))

	byLabel := make(map[string]report.CauseScore, len(fallback))
	for _, item := range fallback {
		byLabel[item.Label] = item
	}
	for _, item := range parsed.Causes {
		current, ok := byLabel[item.Label]
		if !ok {
			continue
		}
		current.Score = round2(clamp01(item.Score))
		current.Confidence = round2(clamp01(item.Confidence))
		current.EvidenceRefs = filterEvidenceRefs(item.EvidenceRefs, evidenceIDs)
		current.CounterEvidence = sanitizeLines(item.CounterEvidence, 3, 200)
		byLabel[item.Label] = current
	}

	out := make([]report.CauseScore, 0, len(fallback))
	for _, base := range fallback {
		out = append(out, byLabel[base.Label])
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out, nil
}

func buildTimeline(data analysisBundle, evidence []report.EvidenceItem) []report.TimelineEvent {
	out := make([]report.TimelineEvent, 0, 8)
	if created := stringValue(data.Repository["created_at"]); created != "" {
		out = append(out, report.TimelineEvent{
			Timestamp:   created,
			Title:       "Repository created",
			Description: "Project initialized.",
			SourceRef:   stringValue(data.Repository["html_url"]),
		})
	}
	if pushed := stringValue(data.Repository["pushed_at"]); pushed != "" {
		out = append(out, report.TimelineEvent{
			Timestamp:   pushed,
			Title:       "Last activity",
			Description: "Last observed commit push timestamp.",
			SourceRef:   stringValue(data.Repository["html_url"]),
		})
	}
	top := evidence
	if len(top) > 6 {
		top = top[:6]
	}
	for _, ev := range top {
		out = append(out, report.TimelineEvent{
			Timestamp:   ev.Timestamp,
			Title:       ev.Title,
			Description: ev.Type + " evidence",
			SourceRef:   ev.ID,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return parseTime(out[i].Timestamp).Before(parseTime(out[j].Timestamp))
	})
	return out
}

func inferCorePhilosophy(repoMeta map[string]any) []string {
	description := strings.ToLower(stringValue(repoMeta["description"]))
	topics := strings.ToLower(strings.Join(asStringSliceAny(repoMeta["topics"]), " "))
	philosophy := []string{"Pragmatic maintainability and clear developer workflow."}

	if strings.Contains(description, "simple") || strings.Contains(topics, "minimal") {
		philosophy = append(philosophy, "Keep the system simple and predictable.")
	}
	if strings.Contains(topics, "performance") || strings.Contains(description, "fast") {
		philosophy = append(philosophy, "Prioritize performance and low overhead.")
	}
	if strings.Contains(topics, "security") {
		philosophy = append(philosophy, "Treat security guarantees as first-class design constraints.")
	}
	philosophy = append(philosophy, "Preserve original project purpose while modernizing execution model.")
	return philosophy
}

func asStringSliceAny(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func relevanceScore(text string) float64 {
	lower := strings.ToLower(text)
	weight := 0.0
	keywords := []string{"deprecated", "abandoned", "security", "rewrite", "migration", "maintainer", "roadmap", "broken"}
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			weight += 0.14
		}
	}
	if weight < 0.1 {
		weight = 0.1
	}
	return round2(math.Min(1, weight))
}

func trimText(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func filterEvidenceRefs(refs []string, evidenceIDs map[string]struct{}) []string {
	out := make([]string, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if _, ok := evidenceIDs[ref]; !ok {
			continue
		}
		if _, dup := seen[ref]; dup {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

func sanitizeLines(lines []string, maxItems, maxLen int) []string {
	out := make([]string, 0, maxItems)
	for _, line := range lines {
		line = trimText(strings.TrimSpace(line), maxLen)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func extractJSONObject(raw string) string {
	text := strings.TrimSpace(raw)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func parseTime(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t
	}
	return time.Time{}
}
