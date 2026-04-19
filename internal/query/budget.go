package query

import "sync"

type BudgetLimits struct {
	MaxTurns  int
	MaxTokens int
	MaxCost   float64
}

type BudgetSnapshot struct {
	MaxTurns   int     `json:"max_turns"`
	UsedTurns  int     `json:"used_turns"`
	MaxTokens  int     `json:"max_tokens"`
	UsedTokens int     `json:"used_tokens"`
	MaxCost    float64 `json:"max_cost"`
	UsedCost   float64 `json:"used_cost"`
	StopReason string  `json:"stop_reason,omitempty"`
}

type Budget struct {
	mu sync.Mutex

	limits BudgetLimits
	used   BudgetSnapshot
}

func NewBudget(limits BudgetLimits) *Budget {
	return &Budget{
		limits: limits,
		used: BudgetSnapshot{
			MaxTurns:  limits.MaxTurns,
			MaxTokens: limits.MaxTokens,
			MaxCost:   limits.MaxCost,
		},
	}
}

func (b *Budget) ConsumeTurn() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.limits.MaxTurns > 0 && b.used.UsedTurns >= b.limits.MaxTurns {
		b.used.StopReason = "budget.max_turns_exceeded"
		return false
	}
	b.used.UsedTurns++
	return true
}

func (b *Budget) AddTokens(tokens int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.used.UsedTokens += tokens
	if b.limits.MaxTokens > 0 && b.used.UsedTokens > b.limits.MaxTokens {
		b.used.StopReason = "budget.max_tokens_exceeded"
		return false
	}
	return true
}

func (b *Budget) AddCost(cost float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.used.UsedCost += cost
	if b.limits.MaxCost > 0 && b.used.UsedCost > b.limits.MaxCost {
		b.used.StopReason = "budget.max_cost_exceeded"
		return false
	}
	return true
}

func (b *Budget) Snapshot() BudgetSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}
