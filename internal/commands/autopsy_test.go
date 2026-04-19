package commands

import (
	"testing"

	"github.com/repo-necromancer/necro/internal/report"
)

func TestScoreCausesRuleBased(t *testing.T) {
	tests := []struct {
		name     string
		evidence []report.EvidenceItem
		check    func(t *testing.T, got []report.CauseScore)
	}{
		{
			name:     "empty evidence returns all zero scores",
			evidence: []report.EvidenceItem{},
			check: func(t *testing.T, got []report.CauseScore) {
				if len(got) == 0 {
					t.Fatal("expected non-empty cause scores")
				}
				for _, cs := range got {
					if cs.Score != 0 {
						t.Errorf("empty evidence: expected score=0 for %s, got %v", cs.Label, cs.Score)
					}
				}
			},
		},
		{
			name: "burnout keyword yields non-zero score for maintainer_burnout",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "maintainer burnout is real", Summary: "no time to maintain", Relevance: 0.5},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var burnout *report.CauseScore
				for _, cs := range got {
					if cs.Label == "maintainer_burnout" {
						burnout = &cs
						break
					}
				}
				if burnout == nil {
					t.Fatal("maintainer_burnout cause not found")
				}
				if burnout.Score <= 0 {
					t.Errorf("expected non-zero score for burnout, got %v", burnout.Score)
				}
			},
		},
		{
			name: "security keyword yields non-zero score for security_trust_collapse",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "Critical security vulnerability discovered", Summary: "CVE-2024 exploit found", Relevance: 0.5},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var sec *report.CauseScore
				for _, cs := range got {
					if cs.Label == "security_trust_collapse" {
						sec = &cs
						break
					}
				}
				if sec == nil {
					t.Fatal("security_trust_collapse cause not found")
				}
				if sec.Score <= 0 {
					t.Errorf("expected non-zero score for security, got %v", sec.Score)
				}
			},
		},
		{
			name: "multiple hits increase score up to max of 1",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E002", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E003", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E004", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E005", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E006", Type: "issue", Title: "burnout", Summary: "burnout"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var burnout *report.CauseScore
				for _, cs := range got {
					if cs.Label == "maintainer_burnout" {
						burnout = &cs
						break
					}
				}
				if burnout == nil {
					t.Fatal("maintainer_burnout cause not found")
				}
				if burnout.Score != 1.0 {
					t.Errorf("expected score=1.0 for 5+ hits, got %v", burnout.Score)
				}
			},
		},
		{
			name: "cause scores are sorted descending by score",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "burnout", Summary: "maintainer no time abandoned overwhelmed"},
				{ID: "E002", Type: "issue", Title: "security", Summary: "security cve vulnerability exploit"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				for i := 1; i < len(got); i++ {
					if got[i-1].Score < got[i].Score {
						t.Errorf("scores not sorted descending: [%d]=%v > [%d]=%v",
							i-1, got[i-1].Score, i, got[i].Score)
					}
				}
			},
		},
		{
			name: "confidence increases with hits but caps at 1",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "burnout", Summary: "abandoned"},
				{ID: "E002", Type: "issue", Title: "burnout", Summary: "abandoned"},
				{ID: "E003", Type: "issue", Title: "burnout", Summary: "abandoned"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var burnout *report.CauseScore
				for _, cs := range got {
					if cs.Label == "maintainer_burnout" {
						burnout = &cs
						break
					}
				}
				if burnout == nil {
					t.Fatal("maintainer_burnout cause not found")
				}
				if burnout.Confidence < 0.2 {
					t.Errorf("confidence should be at least 0.2, got %v", burnout.Confidence)
				}
				if burnout.Confidence > 1.0 {
					t.Errorf("confidence should not exceed 1.0, got %v", burnout.Confidence)
				}
			},
		},
		{
			name: "evidence refs limited to 6 per cause",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E002", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E003", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E004", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E005", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E006", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E007", Type: "issue", Title: "burnout", Summary: "burnout"},
				{ID: "E008", Type: "issue", Title: "burnout", Summary: "burnout"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var burnout *report.CauseScore
				for _, cs := range got {
					if cs.Label == "maintainer_burnout" {
						burnout = &cs
						break
					}
				}
				if burnout == nil {
					t.Fatal("maintainer_burnout cause not found")
				}
				if len(burnout.EvidenceRefs) > 6 {
					t.Errorf("evidence refs should be capped at 6, got %d", len(burnout.EvidenceRefs))
				}
			},
		},
		{
			name: "no hits adds counter evidence",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "commit", Title: "normal commit", Summary: "fix typo"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				for _, cs := range got {
					if cs.Label == "maintainer_burnout" && cs.Score == 0 {
						if len(cs.CounterEvidence) == 0 {
							t.Error("expected counter evidence for zero-score cause")
						}
					}
				}
			},
		},
		{
			name: "all taxonomy labels are represented",
			evidence: []report.EvidenceItem{},
			check: func(t *testing.T, got []report.CauseScore) {
				expected := []string{
					"maintainer_burnout",
					"architecture_debt",
					"ecosystem_displacement",
					"security_trust_collapse",
					"governance_failure",
					"funding_absence",
					"scope_drift",
				}
				if len(got) != len(expected) {
					t.Errorf("expected %d causes, got %d", len(expected), len(got))
				}
				labels := make(map[string]bool)
				for _, cs := range got {
					labels[cs.Label] = true
				}
				for _, e := range expected {
					if !labels[e] {
						t.Errorf("missing cause label: %s", e)
					}
				}
			},
		},
		{
			name: "case insensitive keyword matching",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "BURNOUT everywhere", Summary: "MAINTAINER no time"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var burnout *report.CauseScore
				for _, cs := range got {
					if cs.Label == "maintainer_burnout" {
						burnout = &cs
						break
					}
				}
				if burnout == nil {
					t.Fatal("maintainer_burnout cause not found")
				}
				if burnout.Score == 0 {
					t.Error("expected non-zero score for uppercase BURNOUT/MAINTAINER")
				}
			},
		},
		{
			name: "architecture_debt matched by tech debt keyword",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "Tech debt is accumulating", Summary: "legacy code hard to maintain"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var archDebt *report.CauseScore
				for _, cs := range got {
					if cs.Label == "architecture_debt" {
						archDebt = &cs
						break
					}
				}
				if archDebt == nil {
					t.Fatal("architecture_debt cause not found")
				}
				if archDebt.Score == 0 {
					t.Error("expected non-zero score for tech debt/legacy keywords")
				}
			},
		},
		{
			name: "ecosystem_displacement matched by superseded keyword",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "Project superseded by new framework", Summary: "users migrating elsewhere"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var eco *report.CauseScore
				for _, cs := range got {
					if cs.Label == "ecosystem_displacement" {
						eco = &cs
						break
					}
				}
				if eco == nil {
					t.Fatal("ecosystem_displacement cause not found")
				}
				if eco.Score == 0 {
					t.Error("expected non-zero score for superseded/migration keywords")
				}
			},
		},
		{
			name: "governance_failure matched by bus factor keyword",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "Bus factor problem", Summary: "maintainer conflict causing deadlock"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var gov *report.CauseScore
				for _, cs := range got {
					if cs.Label == "governance_failure" {
						gov = &cs
						break
					}
				}
				if gov == nil {
					t.Fatal("governance_failure cause not found")
				}
				if gov.Score == 0 {
					t.Error("expected non-zero score for governance/bus factor keywords")
				}
			},
		},
		{
			name: "funding_absence matched by sponsor keyword",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "Looking for sponsors", Summary: "unpaid maintainers need funding"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var fund *report.CauseScore
				for _, cs := range got {
					if cs.Label == "funding_absence" {
						fund = &cs
						break
					}
				}
				if fund == nil {
					t.Fatal("funding_absence cause not found")
				}
				if fund.Score == 0 {
					t.Error("expected non-zero score for funding/sponsor keywords")
				}
			},
		},
		{
			name: "scope_drift matched by scope creep keyword",
			evidence: []report.EvidenceItem{
				{ID: "E001", Type: "issue", Title: "Scope creep is out of control", Summary: "roadmap chaos and drift"},
			},
			check: func(t *testing.T, got []report.CauseScore) {
				var scope *report.CauseScore
				for _, cs := range got {
					if cs.Label == "scope_drift" {
						scope = &cs
						break
					}
				}
				if scope == nil {
					t.Fatal("scope_drift cause not found")
				}
				if scope.Score == 0 {
					t.Error("expected non-zero score for scope creep/roadmap chaos keywords")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreCausesRuleBased(tt.evidence)
			tt.check(t, got)
		})
	}
}
