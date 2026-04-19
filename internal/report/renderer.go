package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) RenderMarkdown(rep NecropsyReport) (string, error) {
	var b strings.Builder

	b.WriteString("# 验尸报告与转世方案\n\n")
	b.WriteString("## 1. Project profile\n")
	b.WriteString(fmt.Sprintf("- Repository: `%s`\n", rep.Repository))
	b.WriteString(fmt.Sprintf("- Snapshot date: %s\n", rep.SnapshotDate))
	b.WriteString(fmt.Sprintf("- Stars: %d\n", rep.Stars))
	b.WriteString(fmt.Sprintf("- Last commit at: %s\n\n", rep.LastCommitAt))

	b.WriteString("## 2. Death qualification criteria\n")
	b.WriteString(fmt.Sprintf("- Inactivity threshold (years): %d\n\n", rep.DeathThresholdYears))

	b.WriteString("## 3. Evidence index\n")
	for _, e := range rep.Evidence {
		b.WriteString(fmt.Sprintf("- [%s] %s (%s) %s\n", e.ID, e.Title, e.Type, e.URL))
	}
	b.WriteString("\n")

	b.WriteString("## 4. Timeline and turning points\n")
	for _, ev := range rep.Timeline {
		b.WriteString(fmt.Sprintf("- %s: **%s** — %s (ref: %s)\n", ev.Timestamp, ev.Title, ev.Description, ev.SourceRef))
	}
	b.WriteString("\n")

	b.WriteString("## 5. Cause analysis (with confidence + counter-evidence)\n")
	for _, c := range rep.CauseScores {
		b.WriteString(fmt.Sprintf("- `%s`: score=%.2f confidence=%.2f evidence=%v counter=%v\n", c.Label, c.Score, c.Confidence, c.EvidenceRefs, c.CounterEvidence))
	}
	b.WriteString("\n")

	b.WriteString("## 6. Core philosophy extraction\n")
	for _, p := range rep.CorePhilosophy {
		b.WriteString(fmt.Sprintf("- %s\n", p))
	}
	b.WriteString("\n")

	b.WriteString("## 7. 2026 reincarnation architecture\n")
	b.WriteString(fmt.Sprintf("- Target stack: %s\n", rep.ReincarnationPlan.TargetStack))
	for _, layer := range rep.ReincarnationPlan.Architecture {
		b.WriteString(fmt.Sprintf("- %s\n", layer))
	}
	b.WriteString("\n")

	b.WriteString("## 8. 90-day implementation roadmap\n")
	for _, m := range rep.Next90Days {
		b.WriteString(fmt.Sprintf("- %s: %s (%s)\n", m.DayRange, m.Objective, strings.Join(m.Deliverables, ", ")))
	}
	b.WriteString("\n")

	b.WriteString("## 9. Risks and stop-loss actions\n")
	for _, rk := range rep.Risks {
		b.WriteString(fmt.Sprintf("- [%s] %s — stop-loss: %s\n", rk.Severity, rk.Title, rk.StopLossAction))
	}
	b.WriteString("\n")

	b.WriteString("## 10. Method and source attribution\n")
	b.WriteString("- Sources include GitHub metadata/issues/PRs/commits and permission-gated network tools.\n")
	b.WriteString("- All evidence entries include URL/time traceability.\n")
	return b.String(), nil
}

func (r *Renderer) RenderJSON(rep NecropsyReport) ([]byte, error) {
	return json.MarshalIndent(rep, "", "  ")
}

func (r *Renderer) WriteArtifacts(rep NecropsyReport, outDir, format string) ([]string, error) {
	if strings.TrimSpace(outDir) == "" {
		outDir = "./out"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	if format == "" {
		format = "both"
	}

	written := make([]string, 0, 3)
	switch format {
	case "markdown":
		md, err := r.RenderMarkdown(rep)
		if err != nil {
			return nil, err
		}
		p := filepath.Join(outDir, "report.md")
		if err := os.WriteFile(p, []byte(md), 0o644); err != nil {
			return nil, err
		}
		written = append(written, p)
	case "json":
		js, err := r.RenderJSON(rep)
		if err != nil {
			return nil, err
		}
		p := filepath.Join(outDir, "report.json")
		if err := os.WriteFile(p, js, 0o644); err != nil {
			return nil, err
		}
		written = append(written, p)
	case "both":
		md, err := r.RenderMarkdown(rep)
		if err != nil {
			return nil, err
		}
		mdPath := filepath.Join(outDir, "report.md")
		if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
			return nil, err
		}
		written = append(written, mdPath)

		js, err := r.RenderJSON(rep)
		if err != nil {
			return nil, err
		}
		jsPath := filepath.Join(outDir, "report.json")
		if err := os.WriteFile(jsPath, js, 0o644); err != nil {
			return nil, err
		}
		written = append(written, jsPath)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}

	evidenceJSON, err := json.MarshalIndent(rep.Evidence, "", "  ")
	if err != nil {
		return nil, err
	}
	evidencePath := filepath.Join(outDir, "evidence-index.json")
	if err := os.WriteFile(evidencePath, evidenceJSON, 0o644); err != nil {
		return nil, err
	}
	written = append(written, evidencePath)
	return written, nil
}
