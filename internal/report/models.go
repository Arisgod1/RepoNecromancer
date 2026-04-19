package report

type NecropsyReport struct {
	Repository          string            `json:"repository"`
	SnapshotDate        string            `json:"snapshot_date"`
	DeathThresholdYears int               `json:"death_threshold_years"`
	Stars               int               `json:"stars"`
	LastCommitAt        string            `json:"last_commit_at"`
	CorePhilosophy      []string          `json:"core_philosophy"`
	Timeline            []TimelineEvent   `json:"timeline"`
	CauseScores         []CauseScore      `json:"cause_scores"`
	Evidence            []EvidenceItem    `json:"evidence"`
	ReincarnationPlan   ReincarnationPlan `json:"reincarnation_plan"`
	Risks               []RiskItem        `json:"risks"`
	Next90Days          []Milestone       `json:"next_90_days"`
	QueryMetadata       QueryMetadata     `json:"query_metadata,omitempty"`
}

type QueryMetadata struct {
	SessionID   string `json:"session_id,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
	UsedTurns   int    `json:"used_turns,omitempty"`
	MaxTurns    int    `json:"max_turns,omitempty"`
	UsedTokens  int    `json:"used_tokens,omitempty"`
	MaxTokens   int    `json:"max_tokens,omitempty"`
	UsedCost    float64 `json:"used_cost,omitempty"`
	MaxCost     float64 `json:"max_cost,omitempty"`
	Partial     bool    `json:"partial,omitempty"`
}

type TimelineEvent struct {
	Timestamp   string `json:"timestamp"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SourceRef   string `json:"source_ref"`
}

type CauseScore struct {
	Label           string   `json:"label"`
	Score           float64  `json:"score"`
	Confidence      float64  `json:"confidence"`
	EvidenceRefs    []string `json:"evidence_refs"`
	CounterEvidence []string `json:"counter_evidence"`
}

type EvidenceItem struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	URL       string  `json:"url"`
	Title     string  `json:"title"`
	Timestamp string  `json:"timestamp"`
	Summary   string  `json:"summary"`
	Relevance float64 `json:"relevance"`
}

type ReincarnationPlan struct {
	TargetStack      string   `json:"target_stack"`
	Architecture     []string `json:"architecture"`
	MigrationPlan    []string `json:"migration_plan"`
	SuccessorSignals []string `json:"successor_signals"`
}

type RiskItem struct {
	Title          string `json:"title"`
	Severity       string `json:"severity"`
	StopLossAction string `json:"stop_loss_action"`
}

type Milestone struct {
	DayRange     string   `json:"day_range"`
	Objective    string   `json:"objective"`
	Deliverables []string `json:"deliverables"`
}
