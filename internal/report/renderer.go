package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"

	"github.com/repo-necromancer/necro/internal/i18n"
)

var fontBytes []byte

func init() {
	// Load Arial Unicode font for CJK character support in PDF generation
	// Fallback paths for different macOS font locations
	fontPaths := []string{
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/System/Library/Fonts/Arial Unicode.ttf",
	}
	for _, path := range fontPaths {
		if data, err := os.ReadFile(path); err == nil {
			fontBytes = data
			break
		}
	}
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) RenderMarkdown(rep NecropsyReport, locale string) (string, error) {
	tr := i18n.GetTranslator()
	t := func(key string) string { return tr.T(locale, key) }

	var b strings.Builder

	b.WriteString("# " + t("app_title") + "\n\n")
	b.WriteString("## 1. " + t("section1_title") + "\n")
	b.WriteString(fmt.Sprintf("- %s: `%s`\n", t("repository"), rep.Repository))
	b.WriteString(fmt.Sprintf("- %s: %s\n", t("snapshot_date"), rep.SnapshotDate))
	b.WriteString(fmt.Sprintf("- %s: %d\n", t("stars"), rep.Stars))
	b.WriteString(fmt.Sprintf("- %s: %s\n\n", t("last_commit_at"), rep.LastCommitAt))

	b.WriteString("## 2. " + t("section2_title") + "\n")
	b.WriteString(fmt.Sprintf("- %s: %d\n\n", t("inactivity_threshold_years"), rep.DeathThresholdYears))

	b.WriteString("## 3. " + t("section3_title") + "\n")
	for _, e := range rep.Evidence {
		b.WriteString(fmt.Sprintf("- [%s] %s (%s) %s\n", e.ID, e.Title, e.Type, e.URL))
	}
	b.WriteString("\n")

	b.WriteString("## 4. " + t("section4_title") + "\n")
	for _, ev := range rep.Timeline {
		b.WriteString(fmt.Sprintf("- %s: **%s** — %s (ref: %s)\n", ev.Timestamp, ev.Title, ev.Description, ev.SourceRef))
	}
	b.WriteString("\n")

	b.WriteString("## 5. " + t("section5_title") + "\n")
	for _, c := range rep.CauseScores {
		b.WriteString(fmt.Sprintf("- `%s`: %s=%.2f %s=%.2f %s=%v %s=%v\n", c.Label, t("score"), c.Score, t("confidence"), c.Confidence, t("evidence_refs"), c.EvidenceRefs, t("counter_evidence"), c.CounterEvidence))
	}
	b.WriteString("\n")

	b.WriteString("## 6. " + t("section6_title") + "\n")
	for _, p := range rep.CorePhilosophy {
		b.WriteString(fmt.Sprintf("- %s\n", p))
	}
	b.WriteString("\n")

	b.WriteString("## 7. " + t("section7_title") + "\n")
	b.WriteString(fmt.Sprintf("- %s: %s\n", t("target_stack"), rep.ReincarnationPlan.TargetStack))
	b.WriteString(fmt.Sprintf("- %s:\n", t("architecture")))
	for _, layer := range rep.ReincarnationPlan.Architecture {
		b.WriteString(fmt.Sprintf("  - %s\n", layer))
	}
	b.WriteString(fmt.Sprintf("- %s:\n", t("migration_plan")))
	for _, step := range rep.ReincarnationPlan.MigrationPlan {
		b.WriteString(fmt.Sprintf("  - %s\n", step))
	}
	b.WriteString("\n")

	b.WriteString("## 8. " + t("section8_title") + "\n")
	for _, m := range rep.Next90Days {
		b.WriteString(fmt.Sprintf("- %s: %s (%s)\n", m.DayRange, m.Objective, strings.Join(m.Deliverables, ", ")))
	}
	b.WriteString("\n")

	b.WriteString("## 9. " + t("section9_title") + "\n")
	for _, rk := range rep.Risks {
		b.WriteString(fmt.Sprintf("- [%s] %s — %s: %s\n", rk.Severity, rk.Title, t("stop_loss_action"), rk.StopLossAction))
	}
	b.WriteString("\n")

	b.WriteString("## 10. " + t("section10_title") + "\n")
	b.WriteString("- " + t("sources_include") + "\n")
	b.WriteString("- " + t("evidence_traceable") + "\n")
	return b.String(), nil
}

func (r *Renderer) RenderJSON(rep NecropsyReport) ([]byte, error) {
	return json.MarshalIndent(rep, "", "  ")
}

func (r *Renderer) RenderPDF(rep NecropsyReport, locale string) ([]byte, error) {
	tr := i18n.GetTranslator()
	t := func(key string) string { return tr.T(locale, key) }

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Register CJK-capable font (both regular and bold styles from TTC)
	if len(fontBytes) > 0 {
		pdf.AddUTF8FontFromBytes("ArialUnicode", "", fontBytes)
		pdf.AddUTF8FontFromBytes("ArialUnicode", "B", fontBytes)
	}

	fontFamily := "Arial"
	if len(fontBytes) > 0 {
		fontFamily = "ArialUnicode"
	}

	// Title
	pdf.SetFont(fontFamily, "B", 18)
	pdf.MultiCell(0, 10, t("app_title"), "", "C", false)
	pdf.Ln(5)

	// Section 1: Project profile
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "1. "+t("section1_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("%s: %s", t("repository"), rep.Repository))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("%s: %s", t("snapshot_date"), rep.SnapshotDate))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("%s: %d", t("stars"), rep.Stars))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("%s: %s", t("last_commit_at"), rep.LastCommitAt))
	pdf.Ln(6)

	// Section 2: Death qualification criteria
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "2. "+t("section2_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("%s: %d", t("inactivity_threshold_years"), rep.DeathThresholdYears))
	pdf.Ln(6)

	// Section 3: Evidence index
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "3. "+t("section3_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 9)
	for _, e := range rep.Evidence {
		pdf.Cell(0, 5, fmt.Sprintf("- [%s] %s (%s) %s", e.ID, e.Title, e.Type, e.URL))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 4: Timeline
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "4. "+t("section4_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 9)
	for _, ev := range rep.Timeline {
		pdf.Cell(0, 5, fmt.Sprintf("- %s: %s -- %s (ref: %s)", ev.Timestamp, ev.Title, ev.Description, ev.SourceRef))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 5: Cause analysis
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "5. "+t("section5_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 9)
	for _, c := range rep.CauseScores {
		pdf.Cell(0, 5, fmt.Sprintf("- %s: %s=%.2f %s=%.2f", c.Label, t("score"), c.Score, t("confidence"), c.Confidence))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 6: Core philosophy
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "6. "+t("section6_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 10)
	for _, p := range rep.CorePhilosophy {
		pdf.Cell(0, 5, fmt.Sprintf("- %s", p))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 7: Reincarnation architecture
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "7. "+t("section7_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("%s: %s", t("target_stack"), rep.ReincarnationPlan.TargetStack))
	pdf.Ln(5)
	for _, layer := range rep.ReincarnationPlan.Architecture {
		pdf.Cell(0, 5, fmt.Sprintf("- %s", layer))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 8: 90-day roadmap
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "8. "+t("section8_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 9)
	for _, m := range rep.Next90Days {
		pdf.Cell(0, 5, fmt.Sprintf("- %s: %s (%s)", m.DayRange, m.Objective, strings.Join(m.Deliverables, ", ")))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 9: Risks
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "9. "+t("section9_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 9)
	for _, rk := range rep.Risks {
		pdf.Cell(0, 5, fmt.Sprintf("- [%s] %s -- %s: %s", rk.Severity, rk.Title, t("stop_loss_action"), rk.StopLossAction))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Section 10: Attribution
	pdf.SetFont(fontFamily, "B", 12)
	pdf.Cell(0, 8, "10. "+t("section10_title"))
	pdf.Ln(8)
	pdf.SetFont(fontFamily, "", 10)
	pdf.Cell(0, 6, t("sources_include"))
	pdf.Ln(6)
	pdf.Cell(0, 6, t("evidence_traceable"))

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *Renderer) WriteArtifacts(rep NecropsyReport, outDir, format, locale string) ([]string, error) {
	if strings.TrimSpace(outDir) == "" {
		outDir = "./out"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	if format == "" {
		format = "both"
	}

	tr := i18n.GetTranslator()
	_ = tr // reserved for future use

	written := make([]string, 0, 3)
	switch format {
	case "markdown":
		md, err := r.RenderMarkdown(rep, locale)
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
		pdfData, err := r.RenderPDF(rep, locale)
		if err != nil {
			return nil, err
		}
		p := filepath.Join(outDir, "report.pdf")
		if err := os.WriteFile(p, pdfData, 0o644); err != nil {
			return nil, err
		}
		written = append(written, p)
	case "both":
		md, err := r.RenderMarkdown(rep, locale)
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

		pdfData, err := r.RenderPDF(rep, locale)
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
