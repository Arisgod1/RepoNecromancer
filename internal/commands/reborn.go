package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/repo-necromancer/necro/internal/llm"
	"github.com/repo-necromancer/necro/internal/report"
)

func newRebornCommand() *cobra.Command {
	var targetStack string
	var constraints string
	var outputFormat string
	var outputDir string
	var years int

	cmd := &cobra.Command{
		Use:   "reborn <owner/repo>",
		Short: "Generate a 2026 reincarnation plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCmd(cmd)
			if err != nil {
				return err
			}
			owner, repo, err := parseOwnerRepo(args[0])
			if err != nil {
				return err
			}

			bundle, _, err := collectAnalysisData(cmd.Context(), app, owner, repo, "", "", app.Config.Analysis.MaxItems, modeFull)
			if err != nil {
				return err
			}
			consText := resolveConstraints(constraints)
			evidence := buildEvidenceStreamed(bundle.Issues, bundle.PullReqs, bundle.Commits, app.Config.Analysis.MaxEvidence)
			plan, risks, milestones := buildReincarnationPlan(bundle.Repository, targetStack, consText, evidence, app.LLMClient)

			// Build NecropsyReport for artifact writing
			if years <= 0 {
				years = app.Config.Analysis.DefaultYears
			}
			snapshotDate := time.Now().UTC().Format(time.RFC3339)
			rep := report.NecropsyReport{
				Repository:          fmt.Sprintf("%s/%s", owner, repo),
				SnapshotDate:        snapshotDate,
				DeathThresholdYears: years,
				Stars:               int(floatValue(bundle.Repository["stars"])),
				LastCommitAt:        stringValue(bundle.Repository["pushed_at"]),
				CorePhilosophy:      inferCorePhilosophy(bundle.Repository),
				ReincarnationPlan:   plan,
				Risks:               risks,
				Next90Days:          milestones,
				Evidence:            evidence,
				QueryMetadata: report.QueryMetadata{
					SessionID:  bundle.QueryResult.SessionID,
					StopReason: bundle.QueryResult.StopReason,
					UsedTurns:  bundle.QueryResult.Budget.UsedTurns,
					MaxTurns:   bundle.QueryResult.Budget.MaxTurns,
					Partial:    bundle.QueryResult.Partial,
				},
			}

			// Write artifacts using the renderer
			outDir := outputDir
			if strings.TrimSpace(outDir) == "" {
				outDir = app.Config.App.OutputDir
			}
			format := outputFormat
			if strings.TrimSpace(format) == "" {
				format = "both"
			}

			written, err := app.Renderer.WriteArtifacts(rep, outDir, format)
			if err != nil {
				return fmt.Errorf("write artifacts: %w", err)
			}

			// Print file paths
			for _, p := range written {
				fmt.Fprintln(cmd.OutOrStdout(), p)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&targetStack, "target-stack", "", "Target implementation stack")
	cmd.Flags().StringVar(&constraints, "constraints", "", "Constraint text or file path")
	cmd.Flags().StringVar(&outputFormat, "format", "", "Output format: markdown, json, or both (default: both)")
	cmd.Flags().StringVar(&outputDir, "out", "", "Output directory (default: ./out from config)")
	cmd.Flags().IntVar(&years, "years", 0, "Inactivity threshold context in years (default from config)")
	return cmd
}

func buildReincarnationPlan(repoMeta map[string]any, targetStack, constraints string, evidence []report.EvidenceItem, llmClient *llm.Client) (report.ReincarnationPlan, []report.RiskItem, []report.Milestone) {
	plan, risks, milestones := buildReincarnationPlanRuleBased(repoMeta, targetStack, constraints)
	if llmClient == nil {
		return plan, risks, milestones
	}
	llmPlan, llmRisks, llmMilestones, err := buildReincarnationPlanWithLLM(llmClient, repoMeta, targetStack, constraints, evidence)
	if err != nil {
		return plan, risks, milestones
	}
	return llmPlan, llmRisks, llmMilestones
}

func buildReincarnationPlanRuleBased(repoMeta map[string]any, targetStack, constraints string) (report.ReincarnationPlan, []report.RiskItem, []report.Milestone) {
	if strings.TrimSpace(targetStack) == "" {
		targetStack = "Go 1.26 + gRPC/HTTP API + Postgres + OpenTelemetry + GitHub Actions"
	}
	project := stringValue(repoMeta["full_name"])
	if project == "" {
		project = "target project"
	}
	arch := []string{
		"Domain core: typed business rules and explicit invariants.",
		"Interface adapters: CLI/API boundary with strict input validation.",
		"Data layer: migration-safe persistence + cache invalidation controls.",
		"Observability: structured logs, trace IDs, budget telemetry, and request audits.",
		"Security: permission gate around all external tool/network operations.",
	}
	migration := []string{
		"Week 1-2: freeze feature surface and codify compatibility contract.",
		"Week 3-4: implement modular core and adapter shells behind feature gates.",
		"Week 5-8: backfill parity tests + staged data migration.",
		"Week 9-12: canary rollout with stop-loss metrics and rollback playbook.",
	}
	if constraints != "" {
		migration = append(migration, "Constraint alignment: "+trimText(constraints, 180))
	}
	plan := report.ReincarnationPlan{
		TargetStack:      targetStack,
		Architecture:     arch,
		MigrationPlan:    migration,
		SuccessorSignals: []string{"adoption growth", "issue closure velocity", "release cadence stability"},
	}
	risks := []report.RiskItem{
		{
			Title:          "Scope expansion beyond parity rewrite",
			Severity:       "high",
			StopLossAction: "Reject net-new features until parity baseline reaches 90%.",
		},
		{
			Title:          "Migration churn destabilizes users",
			Severity:       "medium",
			StopLossAction: "Run compatibility layer with telemetry until error rate normalizes.",
		},
		{
			Title:          "Maintainer bandwidth remains constrained",
			Severity:       "high",
			StopLossAction: "Define ownership map + rotate on-call before full launch.",
		},
	}
	milestones := []report.Milestone{
		{
			DayRange:     "Day 1-30",
			Objective:    "Stabilize architecture foundation",
			Deliverables: []string{"Compatibility spec", "Core module skeleton", "Permission matrix"},
		},
		{
			DayRange:     "Day 31-60",
			Objective:    "Complete migration-critical flows",
			Deliverables: []string{"Feature parity map", "Data migration rehearsal", "Canary environment"},
		},
		{
			DayRange:     "Day 61-90",
			Objective:    "Ship guarded production rollout",
			Deliverables: []string{"Operational runbook", "Stop-loss alarms", "Public release notes"},
		},
	}
	_ = project
	return plan, risks, milestones
}

type llmPlanResponse struct {
	TargetStack      string             `json:"target_stack"`
	Architecture     []string           `json:"architecture"`
	MigrationPlan    []string           `json:"migration_plan"`
	SuccessorSignals []string           `json:"successor_signals"`
	Risks            []report.RiskItem  `json:"risks"`
	Milestones       []report.Milestone `json:"milestones"`
}

func buildReincarnationPlanWithLLM(llmClient *llm.Client, repoMeta map[string]any, targetStack, constraints string, evidence []report.EvidenceItem) (report.ReincarnationPlan, []report.RiskItem, []report.Milestone, error) {
	defaultStack := targetStack
	if strings.TrimSpace(defaultStack) == "" {
		defaultStack = "Go 1.26 + gRPC/HTTP API + Postgres + OpenTelemetry + GitHub Actions"
	}
	repoName := stringValue(repoMeta["full_name"])
	if repoName == "" {
		repoName = "unknown/unknown"
	}
	description := trimText(stringValue(repoMeta["description"]), 260)
	topics := strings.Join(asStringSliceAny(repoMeta["topics"]), ", ")

	lines := make([]string, 0, len(evidence))
	for i, item := range evidence {
		if i >= 80 {
			break
		}
		lines = append(lines, fmt.Sprintf("%s | %s | %s | %s", item.ID, item.Type, trimText(item.Title, 100), trimText(item.Summary, 200)))
	}
	if len(lines) == 0 {
		lines = append(lines, "No evidence items available.")
	}

	systemPrompt := "You are a principal engineer modernizing legacy repositories. Return ONLY valid JSON, no markdown, no explanation."
	userPrompt := fmt.Sprintf(
		"Build a structured reincarnation proposal for this repository.\n"+
			"Repository: %s\nDescription: %s\nTopics: %s\nPreferred stack: %s\nConstraints: %s\n\n"+
			"Evidence:\n%s\n\n"+
			"Return ONLY a single JSON object with these exact keys (no markdown, no text outside JSON):\n"+
			"- target_stack: a plain string describing the full tech stack (e.g. \"Go 1.26 + gRPC + Postgres + OpenTelemetry\")\n"+
			"- architecture: JSON array of strings describing architectural layers\n"+
			"- migration_plan: JSON array of strings for migration steps\n"+
			"- successor_signals: JSON array of strings for adoption signals\n"+
			"- risks: JSON array of objects {title (string), severity (string: low|medium|high), stop_loss_action (string)}\n"+
			"- milestones: JSON array of objects {day_range (string), objective (string), deliverables (JSON array of strings)}\n",
		repoName,
		description,
		topics,
		defaultStack,
		trimText(strings.TrimSpace(constraints), 400),
		strings.Join(lines, "\n"),
	)

	raw, err := llmClient.Chat(systemPrompt, userPrompt)
	if err != nil {
		return report.ReincarnationPlan{}, nil, nil, err
	}
	raw = extractJSONObject(raw)
	var parsed llmPlanResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return report.ReincarnationPlan{}, nil, nil, fmt.Errorf("parse llm reincarnation plan: %w", err)
	}

	if strings.TrimSpace(parsed.TargetStack) == "" {
		parsed.TargetStack = defaultStack
	}
	plan := report.ReincarnationPlan{
		TargetStack:      parsed.TargetStack,
		Architecture:     sanitizeLines(parsed.Architecture, 8, 220),
		MigrationPlan:    sanitizeLines(parsed.MigrationPlan, 10, 220),
		SuccessorSignals: sanitizeLines(parsed.SuccessorSignals, 8, 180),
	}
	if len(plan.Architecture) == 0 || len(plan.MigrationPlan) == 0 {
		return report.ReincarnationPlan{}, nil, nil, fmt.Errorf("llm returned incomplete plan")
	}

	risks := make([]report.RiskItem, 0, len(parsed.Risks))
	for _, risk := range parsed.Risks {
		title := trimText(strings.TrimSpace(risk.Title), 140)
		stopLoss := trimText(strings.TrimSpace(risk.StopLossAction), 180)
		severity := strings.ToLower(strings.TrimSpace(risk.Severity))
		if title == "" || stopLoss == "" {
			continue
		}
		if severity != "low" && severity != "medium" && severity != "high" {
			severity = "medium"
		}
		risks = append(risks, report.RiskItem{
			Title:          title,
			Severity:       severity,
			StopLossAction: stopLoss,
		})
		if len(risks) >= 8 {
			break
		}
	}

	milestones := make([]report.Milestone, 0, len(parsed.Milestones))
	for _, ms := range parsed.Milestones {
		dayRange := trimText(strings.TrimSpace(ms.DayRange), 40)
		objective := trimText(strings.TrimSpace(ms.Objective), 160)
		deliverables := sanitizeLines(ms.Deliverables, 6, 120)
		if dayRange == "" || objective == "" || len(deliverables) == 0 {
			continue
		}
		milestones = append(milestones, report.Milestone{
			DayRange:     dayRange,
			Objective:    objective,
			Deliverables: deliverables,
		})
		if len(milestones) >= 6 {
			break
		}
	}
	if len(milestones) == 0 {
		return report.ReincarnationPlan{}, nil, nil, fmt.Errorf("llm returned no milestones")
	}

	return plan, risks, milestones, nil
}

func resolveConstraints(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	if st, err := os.Stat(value); err == nil && !st.IsDir() {
		raw, readErr := os.ReadFile(value)
		if readErr == nil {
			return string(raw)
		}
	}
	return value
}
