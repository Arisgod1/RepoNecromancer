package query

import (
	"sync"
	"testing"
)

func TestBudget_ConsumeTurn(t *testing.T) {
	tests := []struct {
		name          string
		limits        BudgetLimits
		calls         int
		wantRemaining []bool
	}{
		{
			name:          "zero max turns means unlimited",
			limits:        BudgetLimits{MaxTurns: 0},
			calls:         10,
			wantRemaining: []bool{true, true, true, true, true, true, true, true, true, true},
		},
		{
			name:          "exact limit reached",
			limits:        BudgetLimits{MaxTurns: 3},
			calls:         3,
			wantRemaining: []bool{true, true, true},
		},
		{
			name:          "limit exceeded returns false",
			limits:        BudgetLimits{MaxTurns: 2},
			calls:         4,
			wantRemaining: []bool{true, true, false, false},
		},
		{
			name:          "single turn limit",
			limits:        BudgetLimits{MaxTurns: 1},
			calls:         2,
			wantRemaining: []bool{true, false},
		},
		{
			name:          "large limit",
			limits:        BudgetLimits{MaxTurns: 100},
			calls:         100,
			wantRemaining: func() []bool {
				r := make([]bool, 100)
				for i := range r {
					r[i] = true
				}
				return r
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBudget(tt.limits)
			for i := 0; i < tt.calls; i++ {
				got := b.ConsumeTurn()
				if got != tt.wantRemaining[i] {
					t.Errorf("ConsumeTurn call %d: got %v, want %v", i+1, got, tt.wantRemaining[i])
				}
			}
		})
	}
}

func TestBudget_AddTokens(t *testing.T) {
	tests := []struct {
		name       string
		limits     BudgetLimits
		addTokens  []int
		wantOk     []bool
	}{
		{
			name:       "zero max tokens means unlimited",
			limits:     BudgetLimits{MaxTokens: 0},
			addTokens:  []int{100, 200, 500},
			wantOk:     []bool{true, true, true},
		},
		{
			name:       "exact limit",
			limits:     BudgetLimits{MaxTokens: 1000},
			addTokens:  []int{500, 500},
			wantOk:     []bool{true, true},
		},
		{
			name:       "exceed limit",
			limits:     BudgetLimits{MaxTokens: 1000},
			addTokens:  []int{600, 500},
			wantOk:     []bool{true, false},
		},
		{
			name:       "gradual accumulation",
			limits:     BudgetLimits{MaxTokens: 100},
			addTokens:  []int{30, 30, 30, 30},
			wantOk:     []bool{true, true, true, false},
		},
		{
			name:       "first add already exceeds",
			limits:     BudgetLimits{MaxTokens: 50},
			addTokens:  []int{100},
			wantOk:     []bool{false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBudget(tt.limits)
			for i, tokens := range tt.addTokens {
				got := b.AddTokens(tokens)
				if got != tt.wantOk[i] {
					t.Errorf("AddTokens(%d) call %d: got %v, want %v", tokens, i+1, got, tt.wantOk[i])
				}
			}
		})
	}
}

func TestBudget_AddCost(t *testing.T) {
	tests := []struct {
		name     string
		limits   BudgetLimits
		addCosts []float64
		wantOk   []bool
	}{
		{
			name:     "zero max cost means unlimited",
			limits:   BudgetLimits{MaxCost: 0},
			addCosts: []float64{0.5, 1.0, 2.0},
			wantOk:   []bool{true, true, true},
		},
		{
			name:     "exact limit",
			limits:   BudgetLimits{MaxCost: 10.0},
			addCosts: []float64{5.0, 5.0},
			wantOk:   []bool{true, true},
		},
		{
			name:     "exceed limit",
			limits:   BudgetLimits{MaxCost: 10.0},
			addCosts: []float64{6.0, 5.0},
			wantOk:   []bool{true, false},
		},
		{
			name:     "small increments",
			limits:   BudgetLimits{MaxCost: 1.0},
			addCosts: []float64{0.3, 0.3, 0.3, 0.3},
			wantOk:   []bool{true, true, true, false},
		},
		{
			name:     "fractional precision",
			limits:   BudgetLimits{MaxCost: 0.01},
			addCosts: []float64{0.005, 0.005, 0.005},
			wantOk:   []bool{true, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBudget(tt.limits)
			for i, cost := range tt.addCosts {
				got := b.AddCost(cost)
				if got != tt.wantOk[i] {
					t.Errorf("AddCost(%f) call %d: got %v, want %v", cost, i+1, got, tt.wantOk[i])
				}
			}
		})
	}
}

func TestBudget_Snapshot(t *testing.T) {
	b := NewBudget(BudgetLimits{MaxTurns: 5, MaxTokens: 1000, MaxCost: 5.0})
	b.ConsumeTurn()
	b.ConsumeTurn()
	b.AddTokens(300)
	b.AddTokens(200)
	b.AddCost(2.0)

	snap := b.Snapshot()
	if snap.MaxTurns != 5 {
		t.Errorf("MaxTurns: got %d, want 5", snap.MaxTurns)
	}
	if snap.UsedTurns != 2 {
		t.Errorf("UsedTurns: got %d, want 2", snap.UsedTurns)
	}
	if snap.MaxTokens != 1000 {
		t.Errorf("MaxTokens: got %d, want 1000", snap.MaxTokens)
	}
	if snap.UsedTokens != 500 {
		t.Errorf("UsedTokens: got %d, want 500", snap.UsedTokens)
	}
	if snap.MaxCost != 5.0 {
		t.Errorf("MaxCost: got %f, want 5.0", snap.MaxCost)
	}
	if snap.UsedCost != 2.0 {
		t.Errorf("UsedCost: got %f, want 2.0", snap.UsedCost)
	}
}

func TestBudget_Snapshot_AfterExhaustion(t *testing.T) {
	b := NewBudget(BudgetLimits{MaxTurns: 2, MaxTokens: 100, MaxCost: 1.0})
	b.ConsumeTurn()
	b.ConsumeTurn()
	b.ConsumeTurn() // should fail, but UsedTurns already incremented
	b.AddTokens(150) // should fail, but UsedTokens already incremented
	b.AddCost(2.0)    // should fail, but UsedCost already incremented

	snap := b.Snapshot()
	if snap.StopReason == "" {
		t.Error("expected StopReason to be set after budget exhaustion")
	}
	// Turns: actual limit is 2, but internal counter goes to 2 then fails at 3rd attempt
	// So UsedTurns = 2 (fails at 3rd call but doesn't increment again)
	if snap.UsedTurns != 2 {
		t.Errorf("UsedTurns: got %d, want 2", snap.UsedTurns)
	}
	// AddTokens increments THEN checks, so 150 is added before failure
	if snap.UsedTokens != 150 {
		t.Errorf("UsedTokens: got %d, want 150", snap.UsedTokens)
	}
	// AddCost increments THEN checks, so 2.0 is added before failure
	if snap.UsedCost != 2.0 {
		t.Errorf("UsedCost: got %f, want 2.0", snap.UsedCost)
	}
}

func TestBudget_StopReason_RecordsLastExceeded(t *testing.T) {
	// Cost is exceeded last, so that is what StopReason records
	b := NewBudget(BudgetLimits{MaxTurns: 100, MaxTokens: 100, MaxCost: 5.0})
	b.ConsumeTurn() // 1
	b.ConsumeTurn() // 2
	b.ConsumeTurn() // 3
	// UsedTurns = 3, StopReason not set yet (MaxTurns = 100)
	b.AddTokens(60) // 60
	b.AddTokens(60) // 120 > 100, exceeds tokens
	// UsedTokens = 120, StopReason = "budget.max_tokens_exceeded"
	b.AddCost(3.0)  // 3.0
	b.AddCost(3.0)  // 6.0 > 5.0, exceeds cost
	// UsedCost = 6.0, StopReason = "budget.max_cost_exceeded" (overwrites)

	snap := b.Snapshot()
	// The last exceeded reason wins
	if snap.StopReason != "budget.max_cost_exceeded" {
		t.Errorf("StopReason: got %q, want last exceeded reason", snap.StopReason)
	}
}

func TestBudget_ConcurrentAccess(t *testing.T) {
	b := NewBudget(BudgetLimits{MaxTurns: 1000, MaxTokens: 100000, MaxCost: 10000.0})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				b.ConsumeTurn()
				b.AddTokens(10)
				b.AddCost(0.5)
			}
		}()
	}
	wg.Wait()

	snap := b.Snapshot()
	if snap.UsedTurns != 1000 {
		t.Errorf("UsedTurns: got %d, want 1000", snap.UsedTurns)
	}
	if snap.UsedTokens != 10000 {
		t.Errorf("UsedTokens: got %d, want 10000", snap.UsedTokens)
	}
	if snap.UsedCost != 500.0 {
		t.Errorf("UsedCost: got %f, want 500.0", snap.UsedCost)
	}
}

func TestNewBudget(t *testing.T) {
	limits := BudgetLimits{MaxTurns: 5, MaxTokens: 1000, MaxCost: 2.5}
	b := NewBudget(limits)

	if b.limits.MaxTurns != 5 {
		t.Errorf("limits.MaxTurns: got %d, want 5", b.limits.MaxTurns)
	}
	if b.limits.MaxTokens != 1000 {
		t.Errorf("limits.MaxTokens: got %d, want 1000", b.limits.MaxTokens)
	}
	if b.limits.MaxCost != 2.5 {
		t.Errorf("limits.MaxCost: got %f, want 2.5", b.limits.MaxCost)
	}

	snap := b.Snapshot()
	if snap.MaxTurns != 5 || snap.MaxTokens != 1000 || snap.MaxCost != 2.5 {
		t.Errorf("snapshot not initialized from limits")
	}
}

func TestBudgetLimits_ZeroMeansUnlimited(t *testing.T) {
	// When MaxXxx is 0, the corresponding resource has no limit
	b := NewBudget(BudgetLimits{MaxTurns: 0, MaxTokens: 0, MaxCost: 0})

	// Should all succeed even with huge numbers
	for i := 0; i < 1000; i++ {
		if !b.ConsumeTurn() {
			t.Errorf("ConsumeTurn() returned false at iteration %d", i)
		}
	}
	for i := 0; i < 1000; i++ {
		if !b.AddTokens(1000000) {
			t.Errorf("AddTokens() returned false at iteration %d", i)
		}
	}
	for i := 0; i < 1000; i++ {
		if !b.AddCost(1e9) {
			t.Errorf("AddCost() returned false at iteration %d", i)
		}
	}
}
