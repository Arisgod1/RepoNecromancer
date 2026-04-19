package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderer_RenderMarkdown(t *testing.T) {
	tests := []struct {
		name string
		rep  NecropsyReport
		check func(t *testing.T, md string)
	}{
		{
			name: "minimal report renders without panic",
			rep: NecropsyReport{
				Repository:          "owner/repo",
				SnapshotDate:        "2026-01-01T00:00:00Z",
				DeathThresholdYears: 2,
				Stars:               100,
				LastCommitAt:        "2024-06-15T10:00:00Z",
				CorePhilosophy:      []string{"Test philosophy"},
				Timeline:            []TimelineEvent{},
				CauseScores:         []CauseScore{},
				Evidence:            []EvidenceItem{},
				ReincarnationPlan:   ReincarnationPlan{TargetStack: "Go", Architecture: []string{}},
				Next90Days:          []Milestone{},
				Risks:               []RiskItem{},
			},
			check: func(t *testing.T, md string) {
				if !strings.Contains(md, "# 验尸报告与转世方案") {
					t.Error("missing report title")
				}
				if !strings.Contains(md, "owner/repo") {
					t.Error("missing repository name")
				}
				if !strings.Contains(md, "Stars: 100") {
					t.Error("missing stars count")
				}
			},
		},
		{
			name: "evidence items are rendered",
			rep: NecropsyReport{
				Repository:          "test/repo",
				SnapshotDate:        "2026-01-01T00:00:00Z",
				DeathThresholdYears: 1,
				Evidence: []EvidenceItem{
					{ID: "E001", Type: "issue", Title: "Bug found", URL: "https://github.com/bug", Timestamp: "2024-01-01", Summary: "details"},
					{ID: "E002", Type: "pr", Title: "Fix bug", URL: "https://github.com/fix", Timestamp: "2024-02-01", Summary: "details"},
				},
				CorePhilosophy:    []string{},
				Timeline:          []TimelineEvent{},
				CauseScores:       []CauseScore{},
				ReincarnationPlan: ReincarnationPlan{},
				Next90Days:        []Milestone{},
				Risks:             []RiskItem{},
			},
			check: func(t *testing.T, md string) {
				if !strings.Contains(md, "[E001]") {
					t.Error("missing evidence E001")
				}
				if !strings.Contains(md, "[E002]") {
					t.Error("missing evidence E002")
				}
				if !strings.Contains(md, "Bug found") {
					t.Error("missing issue title")
				}
				if !strings.Contains(md, "Fix bug") {
					t.Error("missing pr title")
				}
			},
		},
		{
			name: "cause scores rendered with score and confidence",
			rep: NecropsyReport{
				Repository:          "a/b",
				SnapshotDate:        "2026-01-01T00:00:00Z",
				DeathThresholdYears: 1,
				CauseScores: []CauseScore{
					{Label: "maintainer_burnout", Score: 0.85, Confidence: 0.72, EvidenceRefs: []string{"E001"}, CounterEvidence: []string{}},
					{Label: "security_trust_collapse", Score: 0.10, Confidence: 0.30, EvidenceRefs: []string{}, CounterEvidence: []string{"No CVEs"}},
				},
				CorePhilosophy:    []string{},
				Timeline:          []TimelineEvent{},
				Evidence:          []EvidenceItem{},
				ReincarnationPlan: ReincarnationPlan{},
				Next90Days:        []Milestone{},
				Risks:             []RiskItem{},
			},
			check: func(t *testing.T, md string) {
				if !strings.Contains(md, "maintainer_burnout") {
					t.Error("missing maintainer_burnout label")
				}
				if !strings.Contains(md, "score=0.85") {
					t.Error("missing score=0.85")
				}
				if !strings.Contains(md, "confidence=0.72") {
					t.Error("missing confidence=0.72")
				}
			},
		},
		{
			name: "timeline events rendered",
			rep: NecropsyReport{
				Repository:          "a/b",
				SnapshotDate:        "2026-01-01T00:00:00Z",
				DeathThresholdYears: 1,
				Timeline: []TimelineEvent{
					{Timestamp: "2020-01-01", Title: "Repo created", Description: "Initial commit", SourceRef: "https://github.com"},
					{Timestamp: "2024-01-01", Title: "Last push", Description: "Final commit", SourceRef: "https://github.com"},
				},
				CorePhilosophy:    []string{},
				CauseScores:       []CauseScore{},
				Evidence:          []EvidenceItem{},
				ReincarnationPlan: ReincarnationPlan{},
				Next90Days:        []Milestone{},
				Risks:             []RiskItem{},
			},
			check: func(t *testing.T, md string) {
				if !strings.Contains(md, "Repo created") {
					t.Error("missing timeline event title")
				}
				if !strings.Contains(md, "Last push") {
					t.Error("missing last push event")
				}
			},
		},
		{
			name: "reincarnation plan rendered",
			rep: NecropsyReport{
				Repository:          "a/b",
				SnapshotDate:        "2026-01-01T00:00:00Z",
				DeathThresholdYears: 1,
				ReincarnationPlan: ReincarnationPlan{
					TargetStack:  "Go + React",
					Architecture: []string{"API layer", "UI layer", "Data layer"},
				},
				CorePhilosophy: []string{},
				Timeline:       []TimelineEvent{},
				CauseScores:    []CauseScore{},
				Evidence:       []EvidenceItem{},
				Next90Days:     []Milestone{},
				Risks:          []RiskItem{},
			},
			check: func(t *testing.T, md string) {
				if !strings.Contains(md, "Go + React") {
					t.Error("missing target stack")
				}
				if !strings.Contains(md, "API layer") {
					t.Error("missing architecture layer")
				}
			},
		},
		{
			name: "90-day roadmap rendered",
			rep: NecropsyReport{
				Repository:          "a/b",
				SnapshotDate:        "2026-01-01T00:00:00Z",
				DeathThresholdYears: 1,
				Next90Days: []Milestone{
					{DayRange: "1-30", Objective: "Setup", Deliverables: []string{"repo", "pipeline"}},
					{DayRange: "31-60", Objective: "Core features", Deliverables: []string{"auth", "api"}},
					{DayRange: "61-90", Objective: "Launch prep", Deliverables: []string{"docs", "deploy"}},
				},
				CorePhilosophy:    []string{},
				Timeline:          []TimelineEvent{},
				CauseScores:       []CauseScore{},
				Evidence:          []EvidenceItem{},
				ReincarnationPlan: ReincarnationPlan{},
				Risks:             []RiskItem{},
			},
			check: func(t *testing.T, md string) {
				if !strings.Contains(md, "Setup") {
					t.Error("missing roadmap objective")
				}
				if !strings.Contains(md, "repo, pipeline") {
					t.Error("missing deliverables")
				}
			},
		},
		{
			name: "risks section rendered",
			rep: NecropsyReport{
				Repository:          "a/b",
				SnapshotDate:        "2026-01-01T00:00:00Z",
				DeathThresholdYears: 1,
				Risks: []RiskItem{
					{Severity: "high", Title: "Funding gap", StopLossAction: "Seek sponsors"},
					{Severity: "medium", Title: "Scope creep", StopLossAction: "Freeze features"},
				},
				CorePhilosophy:    []string{},
				Timeline:         []TimelineEvent{},
				CauseScores:       []CauseScore{},
				Evidence:          []EvidenceItem{},
				ReincarnationPlan: ReincarnationPlan{},
				Next90Days:        []Milestone{},
			},
			check: func(t *testing.T, md string) {
				if !strings.Contains(md, "Funding gap") {
					t.Error("missing risk title")
				}
				if !strings.Contains(md, "Seek sponsors") {
					t.Error("missing stop-loss action")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRenderer()
			md, err := r.RenderMarkdown(tt.rep)
			if err != nil {
				t.Fatalf("RenderMarkdown returned error: %v", err)
			}
			tt.check(t, md)
		})
	}
}

func TestRenderer_RenderJSON(t *testing.T) {
	rep := NecropsyReport{
		Repository:          "owner/repo",
		SnapshotDate:        "2026-01-01T00:00:00Z",
		DeathThresholdYears: 2,
		Stars:               500,
		LastCommitAt:        "2024-06-15T10:00:00Z",
		CorePhilosophy:      []string{"Philosophy one"},
		CauseScores: []CauseScore{
			{Label: "burnout", Score: 0.8, Confidence: 0.7, EvidenceRefs: []string{"E001"}, CounterEvidence: []string{}},
		},
		Evidence: []EvidenceItem{
			{ID: "E001", Type: "issue", Title: "Title", URL: "https://url", Timestamp: "2024-01-01", Summary: "Summary", Relevance: 0.5},
		},
		ReincarnationPlan: ReincarnationPlan{
			TargetStack:  "Go",
			Architecture: []string{"Layer1"},
		},
		Next90Days: []Milestone{
			{DayRange: "1-30", Objective: "Setup", Deliverables: []string{"a"}},
		},
		Risks: []RiskItem{
			{Title: "Risk1", Severity: "high", StopLossAction: "Stop"},
		},
		Timeline: []TimelineEvent{
			{Timestamp: "2020-01-01", Title: "Created", Description: "Init", SourceRef: "ref"},
		},
	}

	r := NewRenderer()
	data, err := r.RenderJSON(rep)
	if err != nil {
		t.Fatalf("RenderJSON returned error: %v", err)
	}

	// Verify it is valid JSON by unmarshaling
	var unmarshaled NecropsyReport
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("RenderJSON output is not valid JSON: %v", err)
	}

	if unmarshaled.Repository != rep.Repository {
		t.Errorf("Repository: got %q, want %q", unmarshaled.Repository, rep.Repository)
	}
	if unmarshaled.Stars != rep.Stars {
		t.Errorf("Stars: got %d, want %d", unmarshaled.Stars, rep.Stars)
	}
	if len(unmarshaled.CauseScores) != 1 {
		t.Errorf("CauseScores count: got %d, want 1", len(unmarshaled.CauseScores))
	}
	if unmarshaled.CauseScores[0].Score != 0.8 {
		t.Errorf("CauseScores[0].Score: got %f, want 0.8", unmarshaled.CauseScores[0].Score)
	}
}

func TestRenderer_WriteArtifacts(t *testing.T) {
	rep := NecropsyReport{
		Repository:          "test/repo",
		SnapshotDate:        "2026-01-01T00:00:00Z",
		DeathThresholdYears: 1,
		Stars:                42,
		LastCommitAt:         "2024-01-01",
		CorePhilosophy:       []string{"Test"},
		Timeline:             []TimelineEvent{},
		CauseScores: []CauseScore{
			{Label: "burnout", Score: 0.5, Confidence: 0.5, EvidenceRefs: []string{}, CounterEvidence: []string{}},
		},
		Evidence: []EvidenceItem{
			{ID: "E001", Type: "issue", Title: "Issue", URL: "https://url", Timestamp: "2024-01-01", Summary: "Sum", Relevance: 0.3},
		},
		ReincarnationPlan: ReincarnationPlan{TargetStack: "Go", Architecture: []string{"a"}},
		Next90Days:        []Milestone{},
		Risks:             []RiskItem{},
	}

	tests := []struct {
		name      string
		format    string
		outDir    string
		wantFiles []string
		wantErr   bool
	}{
		{
			name:      "markdown format",
			format:    "markdown",
			outDir:    t.TempDir(),
			wantFiles: []string{"report.md", "evidence-index.json"},
		},
		{
			name:      "json format",
			format:    "json",
			outDir:    t.TempDir(),
			wantFiles: []string{"report.json", "evidence-index.json"},
		},
		{
			name:      "both formats",
			format:    "both",
			outDir:    t.TempDir(),
			wantFiles: []string{"report.md", "report.json", "evidence-index.json"},
		},
		{
			name:      "default outDir is ./out",
			format:    "markdown",
			outDir:    "",
			wantFiles: []string{"report.md", "evidence-index.json"},
		},
		{
			name:    "unsupported format returns error",
			format:  "xml",
			outDir:  t.TempDir(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRenderer()
			files, err := r.WriteArtifacts(rep, tt.outDir, tt.format)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for unsupported format")
				}
				return
			}
			if err != nil {
				t.Fatalf("WriteArtifacts returned error: %v", err)
			}
			for _, want := range tt.wantFiles {
				found := false
				for _, f := range files {
					if strings.HasSuffix(f, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected file %q in output %v", want, files)
				}
			}
			// Verify files actually exist
			for _, f := range files {
				if _, err := os.Stat(f); os.IsNotExist(err) {
					t.Errorf("file does not exist: %s", f)
				}
			}
		})
	}
}

func TestRenderer_WriteArtifacts_CreatesDirectory(t *testing.T) {
	r := NewRenderer()
	rep := minimalReport()
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "subdir", "nested")

	files, err := r.WriteArtifacts(rep, outDir, "markdown")
	if err != nil {
		t.Fatalf("WriteArtifacts should create nested directories: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected at least one file")
	}
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("file was not created: %s", f)
		}
	}
}

func TestRenderer_RenderMarkdown_LargeEvidenceSet(t *testing.T) {
	r := NewRenderer()
	items := make([]EvidenceItem, 300)
	for i := range items {
		items[i] = EvidenceItem{
			ID:        "E001",
			Type:      "issue",
			Title:     "Title",
			URL:       "https://url",
			Timestamp: "2024-01-01",
			Summary:   "Summary",
			Relevance: 0.5,
		}
	}
	rep := NecropsyReport{
		Repository:          "a/b",
		SnapshotDate:        "2026-01-01T00:00:00Z",
		DeathThresholdYears: 1,
		Evidence:            items,
		CorePhilosophy:      []string{},
		Timeline:            []TimelineEvent{},
		CauseScores:         []CauseScore{},
		ReincarnationPlan:   ReincarnationPlan{},
		Next90Days:          []Milestone{},
		Risks:               []RiskItem{},
	}

	md, err := r.RenderMarkdown(rep)
	if err != nil {
		t.Fatalf("RenderMarkdown failed on large evidence set: %v", err)
	}
	if md == "" {
		t.Error("expected non-empty markdown output")
	}
}

func TestRenderer_RenderMarkdown_AllCauseScores(t *testing.T) {
	r := NewRenderer()
	labels := []string{"maintainer_burnout", "architecture_debt", "ecosystem_displacement",
		"security_trust_collapse", "governance_failure", "funding_absence", "scope_drift"}
	causes := make([]CauseScore, len(labels))
	for i, l := range labels {
		causes[i] = CauseScore{
			Label:           l,
			Score:           0.1 * float64(i+1),
			Confidence:      0.5,
			EvidenceRefs:    []string{"E001"},
			CounterEvidence: []string{},
		}
	}
	rep := NecropsyReport{
		Repository:          "a/b",
		SnapshotDate:        "2026-01-01T00:00:00Z",
		DeathThresholdYears: 1,
		CauseScores:         causes,
		CorePhilosophy:      []string{},
		Timeline:            []TimelineEvent{},
		Evidence:            []EvidenceItem{},
		ReincarnationPlan:   ReincarnationPlan{},
		Next90Days:          []Milestone{},
		Risks:               []RiskItem{},
	}

	md, err := r.RenderMarkdown(rep)
	if err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}
	for _, l := range labels {
		if !strings.Contains(md, l) {
			t.Errorf("cause label %q not found in markdown", l)
		}
	}
}

func TestRenderer_WriteArtifacts_Overwrites(t *testing.T) {
	r := NewRenderer()
	rep := minimalReport()
	tmpDir := t.TempDir()

	// Write twice - should not error
	_, err := r.WriteArtifacts(rep, tmpDir, "markdown")
	if err != nil {
		t.Fatalf("first WriteArtifacts failed: %v", err)
	}
	_, err = r.WriteArtifacts(rep, tmpDir, "markdown")
	if err != nil {
		t.Fatalf("second WriteArtifacts (overwrite) failed: %v", err)
	}
}

func minimalReport() NecropsyReport {
	return NecropsyReport{
		Repository:          "a/b",
		SnapshotDate:        "2026-01-01T00:00:00Z",
		DeathThresholdYears: 1,
		CorePhilosophy:      []string{},
		Timeline:            []TimelineEvent{},
		CauseScores:         []CauseScore{},
		Evidence:            []EvidenceItem{},
		ReincarnationPlan:   ReincarnationPlan{},
		Next90Days:          []Milestone{},
		Risks:               []RiskItem{},
	}
}
