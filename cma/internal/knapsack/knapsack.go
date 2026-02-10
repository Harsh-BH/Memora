package knapsack

import (
	"sort"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
)

// Optimizer implements the 0/1 Knapsack context window packing via
// Lagrangian relaxation as defined in the CMA paper.
//
// Mathematical basis:
//
//	maximize Σ v_i·x_i
//	subject to Σ w_i·x_i ≤ W  and  x_i ∈ {0,1}
//
// Lagrangian relaxation decision rule:
//
//	x_i = 1 iff v_i / w_i ≥ λ
//
// This yields O(n log n) complexity via greedy density sort.
// The optimizer always force-includes the last K conversation turns
// (phonological loop in Baddeley's working memory model).
type Optimizer struct {
	tokenBudget      int     // W: total token budget
	forceRecentTurns int     // K: number of recent turns to always include
	lambdaInit       float64 // initial Lagrange multiplier
}

// NewOptimizer creates a new Knapsack optimizer.
func NewOptimizer(cfg configs.KnapsackConfig) *Optimizer {
	budget := cfg.TokenBudget
	if budget == 0 {
		budget = 4096
	}
	recent := cfg.ForceRecentTurns
	if recent == 0 {
		recent = 3
	}
	lambda := cfg.LambdaInit
	if lambda == 0 {
		lambda = 0.001
	}

	return &Optimizer{
		tokenBudget:      budget,
		forceRecentTurns: recent,
		lambdaInit:       lambda,
	}
}

// SelectionResult contains the selected items and budget utilization.
type SelectionResult struct {
	Selected    []models.KnapsackItem `json:"selected"`
	TotalTokens int                   `json:"total_tokens"`
	TotalValue  float64               `json:"total_value"`
	Utilization float64               `json:"utilization"` // tokens_used / budget
}

// Optimize selects the highest-value items that fit within the token budget.
// forceItems are always included (e.g., last 3 conversation turns).
// candidates are scored by DIG and ranked by information density.
func (o *Optimizer) Optimize(candidates []models.KnapsackItem, recentTurns []models.ConversationTurn) SelectionResult {
	budget := o.tokenBudget
	var selected []models.KnapsackItem
	totalTokens := 0
	totalValue := 0.0

	// Phase 1: Force-include recent conversation turns (phonological loop).
	turnCount := o.forceRecentTurns
	if turnCount > len(recentTurns) {
		turnCount = len(recentTurns)
	}

	for i := len(recentTurns) - turnCount; i < len(recentTurns); i++ {
		turn := recentTurns[i]
		tokenCount := len(turn.Content) / 4 // approximate
		if tokenCount == 0 {
			tokenCount = 1
		}

		item := models.KnapsackItem{
			ID:           "turn_" + turn.Role + "_" + turn.Timestamp.Format("150405"),
			Content:      turn.Content,
			Value:        1000.0, // maximum priority for recent turns
			Weight:       tokenCount,
			ForceInclude: true,
			Density:      1000.0 / float64(tokenCount),
		}

		selected = append(selected, item)
		totalTokens += tokenCount
		totalValue += item.Value
	}

	budget -= totalTokens

	// Phase 2: Compute density for each candidate: v_i / w_i.
	for i := range candidates {
		if candidates[i].Weight > 0 {
			candidates[i].Density = candidates[i].Value / float64(candidates[i].Weight)
		}
	}

	// Phase 3: Sort by density descending (greedy Lagrangian relaxation).
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Density > candidates[j].Density
	})

	// Phase 4: Compute optimal λ via binary search on the sorted candidates.
	// The shadow price λ is the density threshold below which items are excluded.
	lambda := o.findOptimalLambda(candidates, budget)

	// Phase 5: Select items with density ≥ λ that fit within remaining budget.
	for _, item := range candidates {
		if budget <= 0 {
			break
		}

		// Lagrangian decision rule: include if v_i / w_i ≥ λ.
		if item.Density >= lambda && item.Weight <= budget {
			selected = append(selected, item)
			totalTokens += item.Weight
			totalValue += item.Value
			budget -= item.Weight
		}
	}

	utilization := 0.0
	if o.tokenBudget > 0 {
		utilization = float64(totalTokens) / float64(o.tokenBudget)
	}

	return SelectionResult{
		Selected:    selected,
		TotalTokens: totalTokens,
		TotalValue:  totalValue,
		Utilization: utilization,
	}
}

// findOptimalLambda performs binary search to find the Lagrange multiplier λ
// such that the total weight of items with density ≥ λ is approximately
// equal to the budget W.
func (o *Optimizer) findOptimalLambda(candidates []models.KnapsackItem, budget int) float64 {
	if len(candidates) == 0 || budget <= 0 {
		return 0
	}

	lo := 0.0
	hi := 0.0
	for _, c := range candidates {
		if c.Density > hi {
			hi = c.Density
		}
	}
	hi += 1.0

	// Binary search for λ.
	for iter := 0; iter < 50; iter++ {
		mid := (lo + hi) / 2
		totalWeight := 0
		for _, c := range candidates {
			if c.Density >= mid {
				totalWeight += c.Weight
			}
		}

		if totalWeight > budget {
			lo = mid
		} else {
			hi = mid
		}

		if hi-lo < 1e-9 {
			break
		}
	}

	return lo
}

// SetTokenBudget allows dynamic budget override (e.g., from API request).
func (o *Optimizer) SetTokenBudget(budget int) {
	if budget > 0 {
		o.tokenBudget = budget
	}
}
