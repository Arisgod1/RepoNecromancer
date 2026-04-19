package query

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/repo-necromancer/necro/internal/permissions"
	"github.com/repo-necromancer/necro/internal/state"
	"github.com/repo-necromancer/necro/internal/tools"
)

type Action struct {
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input"`
}

type QueryRequest struct {
	Command   string   `json:"command"`
	SessionID string   `json:"session_id"`
	Actions   []Action `json:"actions"`
	Budget    BudgetLimits
}

type ActionExecution struct {
	ToolName   string                         `json:"tool_name"`
	Decision   permissions.PermissionDecision `json:"decision"`
	Output     map[string]any                 `json:"output,omitempty"`
	Error      string                         `json:"error,omitempty"`
	StartedAt  string                         `json:"started_at"`
	FinishedAt string                         `json:"finished_at"`
}

type QueryResult struct {
	Command    string            `json:"command"`
	SessionID  string            `json:"session_id"`
	Executions []ActionExecution `json:"executions"`
	StopReason string            `json:"stop_reason"`
	Partial    bool              `json:"partial"`
	Budget     BudgetSnapshot    `json:"budget"`
}

type Engine struct {
	registry    *tools.Registry
	permissions *permissions.Engine
	store       state.Store
}

func NewEngine(registry *tools.Registry, permissionsEngine *permissions.Engine, store state.Store) *Engine {
	return &Engine{
		registry:    registry,
		permissions: permissionsEngine,
		store:       store,
	}
}

func (e *Engine) Run(ctx context.Context, req QueryRequest) (QueryResult, error) {
	if strings.TrimSpace(req.Command) == "" {
		return QueryResult{}, errors.New("query command is required")
	}
	if req.Budget.MaxTurns <= 0 {
		req.Budget.MaxTurns = 16
	}
	budget := NewBudget(req.Budget)
	result := QueryResult{
		Command:   req.Command,
		SessionID: req.SessionID,
		Partial:   false,
	}

	for _, action := range req.Actions {
		if err := ctx.Err(); err != nil {
			result.StopReason = "interrupted"
			result.Partial = true
			break
		}
		if !budget.ConsumeTurn() {
			result.StopReason = budget.Snapshot().StopReason
			result.Partial = true
			break
		}

		normalizedInput := normalizeInput(action.Input)
		decision, err := e.permissions.CanUseTool(ctx, action.ToolName, normalizedInput)
		if err != nil {
			return result, fmt.Errorf("permission engine failed for %s: %w", action.ToolName, err)
		}

		ex := ActionExecution{
			ToolName:  action.ToolName,
			Decision:  decision,
			StartedAt: time.Now().UTC().Format(time.RFC3339),
		}

		if decision.Behavior != string(permissions.BehaviorAllow) {
			ex.Error = fmt.Sprintf("tool not executed; decision=%s reason=%s", decision.Behavior, decision.Reason)
			ex.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			result.Executions = append(result.Executions, ex)
			continue
		}

		tool, ok := e.registry.Get(action.ToolName)
		if !ok {
			ex.Error = "tool missing from registry"
			ex.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			result.Executions = append(result.Executions, ex)
			continue
		}

		output, runErr := tool.Run(ctx, normalizedInput)
		if runErr != nil {
			ex.Error = runErr.Error()
		} else {
			ex.Output = output
		}
		ex.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		result.Executions = append(result.Executions, ex)
	}

	if result.StopReason == "" {
		result.StopReason = "completed"
	}
	result.Budget = budget.Snapshot()
	e.store.Set("query:last_result", result)
	if req.SessionID != "" {
		e.store.Set("query:session:"+req.SessionID, result)
	}
	return result, nil
}

func normalizeInput(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		key := strings.TrimSpace(strings.ToLower(k))
		if key == "" {
			continue
		}
		out[key] = normalizeValue(v)
	}
	return out
}

func normalizeValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return normalizeInput(x)
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			out = append(out, normalizeValue(item))
		}
		return out
	default:
		return v
	}
}
