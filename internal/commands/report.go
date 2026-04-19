package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/repo-necromancer/necro/internal/i18n"
	"github.com/repo-necromancer/necro/internal/report"
)

func newReportCommand() *cobra.Command {
	var format string
	var outDir string
	var years int
	var since string
	var until string
	var maxItems int
	var targetStack string
	var constraints string

	cmd := &cobra.Command{
		Use:   "report <owner/repo>",
		Short: "Run end-to-end pipeline and generate final report artifacts",
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
			if years <= 0 {
				years = app.Config.Analysis.DefaultYears
			}
			if maxItems <= 0 {
				maxItems = app.Config.Analysis.MaxItems
			}
			if strings.TrimSpace(outDir) == "" {
				outDir = app.Config.App.OutputDir
			}

			bundle, _, err := collectAnalysisData(cmd.Context(), app, owner, repo, since, until, maxItems, modeFull)
			if err != nil {
				return err
			}

			// Extract evidence for use in parallel LLM calls
			evidence := buildEvidenceStreamed(bundle.Issues, bundle.PullReqs, bundle.Commits, app.Config.Analysis.MaxEvidence)

			// Run buildNecropsyReport and buildReincarnationPlan in parallel
			type planResult struct {
				plan       report.ReincarnationPlan
				risks      []report.RiskItem
				milestones []report.Milestone
			}
			planCh := make(chan planResult, 1)

			go func() {
				p, r, m := buildReincarnationPlan(bundle.Repository, targetStack, resolveConstraints(constraints), evidence, app.LLMClient)
				planCh <- planResult{p, r, m}
			}()

			rep := buildNecropsyReport(owner, repo, years, bundle, app.LLMClient, app.Config.Analysis.MaxEvidence)

			select {
			case pr := <-planCh:
				rep.ReincarnationPlan = pr.plan
				rep.Risks = pr.risks
				rep.Next90Days = pr.milestones
			default:
				// Plan goroutine not ready yet, it will use fallback rule-based plan
			}
			rep.QueryMetadata = report.QueryMetadata{
				SessionID:  bundle.QueryResult.SessionID,
				StopReason: bundle.QueryResult.StopReason,
				UsedTurns:  bundle.QueryResult.Budget.UsedTurns,
				MaxTurns:   bundle.QueryResult.Budget.MaxTurns,
				Partial:    bundle.QueryResult.Partial,
			}

			lang := app.Config.App.Language
			if lang == "" {
				lang = "zh"
			}

			written, err := app.Renderer.WriteArtifacts(rep, outDir, format, lang)
			if err != nil {
				return err
			}
			tr := i18n.GetTranslator()
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s/%s:\n", tr.T(lang, "generated_report_artifacts"), owner, repo)
			for _, p := range written {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", p)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "both", "Output format: markdown|json|both")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory")
	cmd.Flags().IntVar(&years, "years", 0, "Inactivity threshold context (optional, defaults from config)")
	cmd.Flags().StringVar(&since, "since", "", "Optional evidence lower bound")
	cmd.Flags().StringVar(&until, "until", "", "Optional evidence upper bound")
	cmd.Flags().IntVar(&maxItems, "max-items", 0, "Maximum artifact items to fetch")
	cmd.Flags().StringVar(&targetStack, "target-stack", "", "Override target stack used in reincarnation plan")
	cmd.Flags().StringVar(&constraints, "constraints", "", "Constraint text or file path for migration design")
	return cmd
}
