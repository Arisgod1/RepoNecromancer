package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
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

func (r *Renderer) RenderPDF(rep NecropsyReport) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Title
	pdf.SetFont("Arial", "B", 18)
	pdf.MultiCell(0, 10, "验尸报告与转世方案", "", "C", false)
	pdf.Ln(5)

	// Section 1: Project profile
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "1. Project profile")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("Repository: %s", rep.Repository))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("Snapshot date: %s", rep.SnapshotDate))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("Stars: %d", rep.Stars))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("Last commit at: %s", rep.LastCommitAt))
	pdf.Ln(6)

	// Section 2: Death qualification criteria
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "2. Death qualification criteria")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("Inactivity threshold (years): %d", rep.DeathThresholdYears))
	pdf.Ln(6)

	// Section 3: Evidence index
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "3. Evidence index")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 9)
	for _, e := range rep.Evidence {
		pdf.Cell(0, 5, fmt.Sprintf("- [%s] %s (%s) %s", e.ID, e.Title, e.Type, e.URL))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 4: Timeline
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "4. Timeline and turning points")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 9)
	for _, ev := range rep.Timeline {
		pdf.Cell(0, 5, fmt.Sprintf("- %s: %s -- %s (ref: %s)", ev.Timestamp, ev.Title, ev.Description, ev.SourceRef))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 5: Cause analysis
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "5. Cause analysis")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 9)
	for _, c := range rep.CauseScores {
		pdf.Cell(0, 5, fmt.Sprintf("- %s: score=%.2f confidence=%.2f", c.Label, c.Score, c.Confidence))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 6: Core philosophy
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "6. Core philosophy extraction")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	for _, p := range rep.CorePhilosophy {
		pdf.Cell(0, 5, fmt.Sprintf("- %s", p))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 7: Reincarnation architecture
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "7. 2026 reincarnation architecture")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("Target stack: %s", rep.ReincarnationPlan.TargetStack))
	pdf.Ln(5)
	for _, layer := range rep.ReincarnationPlan.Architecture {
		pdf.Cell(0, 5, fmt.Sprintf("- %s", layer))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 8: 90-day roadmap
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "8. 90-day implementation roadmap")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 9)
	for _, m := range rep.Next90Days {
		pdf.Cell(0, 5, fmt.Sprintf("- %s: %s (%s)", m.DayRange, m.Objective, strings.Join(m.Deliverables, ", ")))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 9: Risks
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "9. Risks and stop-loss actions")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 9)
	for _, rk := range rep.Risks {
		pdf.Cell(0, 5, fmt.Sprintf("- [%s] %s -- stop-loss: %s", rk.Severity, rk.Title, rk.StopLossAction))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 10: Attribution
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "10. Method and source attribution")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 6, "Sources include GitHub metadata/issues/PRs/commits and permission-gated network tools.")
	pdf.Ln(6)
	pdf.Cell(0, 6, "All evidence entries include URL/time traceability.")

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	case "pdf":
		pdfData, err := r.RenderPDF(rep)
		if err != nil {
			return nil, err
		}
		p := filepath.Join(outDir, "report.pdf")
		if err := os.WriteFile(p, pdfData, 0o644); err != nil {
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

		pdfData, err := r.RenderPDF(rep)
		if err != nil {
			return nil, err
		}
		pdfPath := filepath.Join(outDir, "report.pdf")
		if err := os.WriteFile(pdfPath, pdfData, 0o644); err != nil {
			return nil, err
		}
		written = append(written, pdfPath)
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
