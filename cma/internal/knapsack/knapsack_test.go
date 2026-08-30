package knapsack

import (
	"math/rand"
	"testing"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
)

// defaultBudget is the token budget NewOptimizer falls back to when the config
// leaves TokenBudget at zero (knapsack.go:35) and is also the value shipped in
// configs/config.yaml. Every invariant below is stated against this number.
const defaultBudget = 4096

// newDefaultOptimizer builds the optimizer exactly the way production does when
// the operator has not overridden anything: budget 4096, force 3 recent turns.
func newDefaultOptimizer() *Optimizer {
	return NewOptimizer(configs.KnapsackConfig{})
}

// tokensOf mirrors the production weight approximation byte-for-byte
// (workspace.go:97 and knapsack.go:78). It is len(content)/4, floored at 1.
// Tests use the same formula so measured token counts transfer to production.
func tokensOf(content string) int {
	n := len(content) / 4
	if n == 0 {
		n = 1
	}
	return n
}

// filler returns a deterministic ASCII string of exactly n bytes. Only the
// length matters to the optimizer, never the bytes.
func filler(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + (i % 26))
	}
	return string(b)
}

// genCandidates builds n synthetic KnapsackItems from a fixed seed.
//
// Content length 120..1600 bytes => weight 30..400 tokens, which is the range
// configs/config.yaml pins for an episode (min_episode_tokens 50,
// max_episode_tokens 500).
//
// Value is a DIG score. dig.go:74 keeps a candidate when DIGScore > minScore
// and configs/config.yaml sets dig.min_score to -0.5, so NEGATIVE and ZERO
// values genuinely reach the knapsack in production. The generator spreads
// values over [-0.5, 3.0] to reflect that.
func genCandidates(seed int64, n int) []models.KnapsackItem {
	rng := rand.New(rand.NewSource(seed))
	items := make([]models.KnapsackItem, n)
	for i := 0; i < n; i++ {
		content := filler(120 + rng.Intn(1481))
		items[i] = models.KnapsackItem{
			ID:      "cand_" + itoa(i),
			Content: content,
			Value:   -0.5 + rng.Float64()*3.5,
			Weight:  tokensOf(content),
		}
	}
	return items
}

// itoa avoids pulling strconv in for one call site.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}

func poolTokens(items []models.KnapsackItem) int {
	total := 0
	for _, it := range items {
		total += it.Weight
	}
	return total
}

func countNonPositiveValue(items []models.KnapsackItem) int {
	n := 0
	for _, it := range items {
		if it.Value <= 0 {
			n++
		}
	}
	return n
}

// fixedTurns builds K conversation turns of exactly charsPerTurn bytes each,
// with fixed timestamps so the generated item IDs are deterministic.
func fixedTurns(k, charsPerTurn int) []models.ConversationTurn {
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	turns := make([]models.ConversationTurn, k)
	for i := 0; i < k; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		turns[i] = models.ConversationTurn{
			Role:      role,
			Content:   filler(charsPerTurn),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		}
	}
	return turns
}

// -----------------------------------------------------------------------------
// (a) CONTEXT BUDGET INVARIANT: however large the candidate pool grows, the
// assembled context stays inside the 4096-token budget.
// -----------------------------------------------------------------------------

func TestBudgetInvariantAcrossCandidateSetSizes(t *testing.T) {
	sizes := []int{10, 50, 100, 500, 1000, 5000, 20000}
	opt := newDefaultOptimizer()

	t.Logf("budget=%d  seed=42  weight=len(content)/4  values in [-0.5,3.0]", defaultBudget)
	t.Logf("%7s %13s %9s %12s %12s %11s %8s %10s",
		"n", "poolTokens", "selected", "totalTokens", "utilization", "totalValue", "negSel", "shrink")

	for _, n := range sizes {
		items := genCandidates(42, n)
		pool := poolTokens(items)

		res := opt.Optimize(items, nil)

		negSel := countNonPositiveValue(res.Selected)
		shrink := float64(pool) / float64(res.TotalTokens)

		if res.TotalTokens > defaultBudget {
			t.Errorf("n=%d: BUDGET BREACHED: TotalTokens=%d > budget=%d",
				n, res.TotalTokens, defaultBudget)
		}
		if res.Utilization > 1.0 {
			t.Errorf("n=%d: Utilization=%.6f > 1.0", n, res.Utilization)
		}
		for _, it := range res.Selected {
			if it.Weight > defaultBudget {
				t.Errorf("n=%d: selected item %s weighs %d, more than the whole budget",
					n, it.ID, it.Weight)
			}
		}

		t.Logf("%7d %13d %9d %12d %12.4f %11.2f %8d %9.1fx",
			n, pool, len(res.Selected), res.TotalTokens, res.Utilization,
			res.TotalValue, negSel, shrink)
	}
}

// Same sweep, but with three normal-sized conversation turns force-included,
// which is what the workspace actually calls.
func TestBudgetInvariantWithForcedRecentTurns(t *testing.T) {
	sizes := []int{10, 100, 1000, 5000}
	turns := fixedTurns(5, 600) // 5 turns; the optimizer force-includes the last 3
	forced := tokensOf(filler(600)) * 3

	opt := newDefaultOptimizer()
	t.Logf("budget=%d  5 turns of 600 chars, last 3 forced => %d forced tokens",
		defaultBudget, forced)
	t.Logf("%7s %9s %12s %12s %11s", "n", "selected", "totalTokens", "utilization", "forcedIn")

	for _, n := range sizes {
		items := genCandidates(7, n)
		res := opt.Optimize(items, turns)

		forcedIn := 0
		for _, it := range res.Selected {
			if it.ForceInclude {
				forcedIn++
			}
		}

		if res.TotalTokens > defaultBudget {
			t.Errorf("n=%d: BUDGET BREACHED: TotalTokens=%d > budget=%d",
				n, res.TotalTokens, defaultBudget)
		}
		if forcedIn != 3 {
			t.Errorf("n=%d: expected 3 force-included turns, got %d", n, forcedIn)
		}

		t.Logf("%7d %9d %12d %11.4f %11d", n, len(res.Selected), res.TotalTokens,
			res.Utilization, forcedIn)
	}
}

// -----------------------------------------------------------------------------
// (b) THE FORCE-INCLUDE BREACH. Phase 1 of Optimize adds the last K turns
// unconditionally and only THEN subtracts them from the budget. Nothing caps
// them. This test measures where the invariant actually breaks and asserts the
// TRUE behaviour rather than the intended one.
// -----------------------------------------------------------------------------

func TestForceIncludeCanBreachBudget(t *testing.T) {
	opt := newDefaultOptimizer()
	candidates := genCandidates(11, 200)

	// 3 turns x 8000 chars = 3 x 2000 tokens = 6000 tokens, vs a 4096 budget.
	turns := fixedTurns(3, 8000)
	res := opt.Optimize(append([]models.KnapsackItem(nil), candidates...), turns)

	forcedTokens := 3 * tokensOf(filler(8000))
	t.Logf("3 forced turns of 8000 chars = %d tokens against a %d-token budget",
		forcedTokens, defaultBudget)
	t.Logf("RESULT: TotalTokens=%d  Utilization=%.4f  selected=%d  overshoot=%+d tokens (%.1f%% of budget)",
		res.TotalTokens, res.Utilization, len(res.Selected),
		res.TotalTokens-defaultBudget,
		100*float64(res.TotalTokens-defaultBudget)/float64(defaultBudget))

	// DOCUMENTING PRODUCTION BEHAVIOUR, NOT ENDORSING IT: the budget invariant
	// does not hold once the forced turns alone exceed the budget.
	if res.TotalTokens <= defaultBudget {
		t.Fatalf("expected the documented breach (TotalTokens > %d), got %d; "+
			"if this now passes, knapsack.go grew a cap and this test should be inverted",
			defaultBudget, res.TotalTokens)
	}
	if res.TotalTokens != forcedTokens {
		t.Errorf("expected exactly the forced turns and nothing else (%d tokens), got %d",
			forcedTokens, res.TotalTokens)
	}
	if len(res.Selected) != 3 {
		t.Errorf("expected 3 selected items (the forced turns only), got %d", len(res.Selected))
	}
	for _, it := range res.Selected {
		if !it.ForceInclude {
			t.Errorf("candidate %s slipped in on a negative budget", it.ID)
		}
	}
}

// Sweep the forced-turn size to locate the exact threshold at which the
// invariant starts failing, so the breach can be quoted with a number.
func TestForceIncludeBreachThreshold(t *testing.T) {
	opt := newDefaultOptimizer()
	candidates := genCandidates(13, 300)

	t.Logf("budget=%d, 3 forced turns, sweeping chars-per-turn", defaultBudget)
	t.Logf("%12s %13s %12s %12s %9s %9s",
		"charsPerTurn", "forcedTokens", "totalTokens", "utilization", "selected", "breached")

	for _, chars := range []int{400, 2000, 4000, 5460, 5464, 8000, 40000} {
		turns := fixedTurns(3, chars)
		res := opt.Optimize(append([]models.KnapsackItem(nil), candidates...), turns)
		forced := 3 * tokensOf(filler(chars))
		breached := res.TotalTokens > defaultBudget

		// The breach must be attributable to the forced turns alone; the
		// candidate loop is never allowed to push past the budget on its own.
		if breached && forced <= defaultBudget {
			t.Errorf("chars=%d: budget exceeded (%d) even though forced turns only take %d",
				chars, res.TotalTokens, forced)
		}

		t.Logf("%12d %13d %12d %12.4f %9d %9v",
			chars, forced, res.TotalTokens, res.Utilization, len(res.Selected), breached)
	}
}

// -----------------------------------------------------------------------------
// (c) CAN A NEGATIVE-DENSITY ITEM EVER BE SELECTED? Settling the audit dispute.
// -----------------------------------------------------------------------------

// findOptimalLambda seeds lo at 0.0 and only ever raises it, so the returned
// multiplier is >= 0 for every possible input. Proven by sweeping seeded
// instances, including all-negative ones.
func TestFindOptimalLambdaIsNeverNegative(t *testing.T) {
	opt := newDefaultOptimizer()
	minLambda, maxLambda := 0.0, 0.0
	first := true

	for seed := int64(0); seed < 25; seed++ {
		for _, n := range []int{1, 10, 200, 2000} {
			items := genCandidates(seed, n)
			for i := range items {
				items[i].Density = items[i].Value / float64(items[i].Weight)
			}
			for _, budget := range []int{-1, 0, 1, 128, defaultBudget, 1 << 20} {
				lambda := opt.findOptimalLambda(items, budget)
				if lambda < 0 {
					t.Fatalf("seed=%d n=%d budget=%d: findOptimalLambda returned %g < 0",
						seed, n, budget, lambda)
				}
				if first || lambda < minLambda {
					minLambda = lambda
				}
				if first || lambda > maxLambda {
					maxLambda = lambda
				}
				first = false
			}
		}
	}

	// All-negative pool: lambda must still floor at zero.
	allNeg := make([]models.KnapsackItem, 100)
	for i := range allNeg {
		allNeg[i] = models.KnapsackItem{Weight: 50, Value: -0.4, Density: -0.008}
	}
	if lambda := opt.findOptimalLambda(allNeg, defaultBudget); lambda != 0 {
		t.Errorf("all-negative pool: expected lambda 0, got %g", lambda)
	}

	t.Logf("findOptimalLambda over 25 seeds x 4 sizes x 6 budgets = 600 calls: "+
		"min=%.9f max=%.9f, never negative", minLambda, maxLambda)
}

// The decision rule is Density >= lambda with lambda >= 0, so an item with
// negative density is unreachable. Measured on an all-negative pool that would
// otherwise fit the budget many times over.
func TestNegativeValueItemsAreNeverSelected(t *testing.T) {
	opt := newDefaultOptimizer()

	// Pool of 40 items x 50 tokens = 2000 tokens, comfortably under 4096, so
	// only the sign of the value can keep them out.
	items := make([]models.KnapsackItem, 40)
	for i := range items {
		content := filler(200) // 50 tokens
		items[i] = models.KnapsackItem{
			ID:      "neg_" + itoa(i),
			Content: content,
			Value:   -0.5 + float64(i)*0.0001, // all strictly negative
			Weight:  tokensOf(content),
		}
	}

	res := opt.Optimize(items, nil)
	t.Logf("all-negative pool: %d items / %d tokens offered, budget %d -> selected=%d tokens=%d value=%.4f",
		len(items), 40*50, defaultBudget, len(res.Selected), res.TotalTokens, res.TotalValue)

	if len(res.Selected) != 0 {
		t.Errorf("negative-value items were admitted: %d selected, TotalValue=%.4f",
			len(res.Selected), res.TotalValue)
	}
	if res.TotalTokens != 0 {
		t.Errorf("expected 0 tokens from an all-negative pool, got %d", res.TotalTokens)
	}
}

// Mixed pool that fits entirely inside the budget: every positive item should
// be taken, every negative one dropped, and zero-value items are the boundary
// case (Density 0 >= lambda 0 passes, so they DO get in and burn tokens).
func TestMixedSignPoolSelectsOnlyNonNegativeDensity(t *testing.T) {
	opt := newDefaultOptimizer()

	var items []models.KnapsackItem
	content := filler(120) // 30 tokens
	for i := 0; i < 30; i++ {
		items = append(items, models.KnapsackItem{
			ID: "pos_" + itoa(i), Content: content, Value: 0.05 + float64(i)*0.1, Weight: tokensOf(content),
		})
	}
	for i := 0; i < 30; i++ {
		items = append(items, models.KnapsackItem{
			ID: "neg_" + itoa(i), Content: content, Value: -0.01 - float64(i)*0.01, Weight: tokensOf(content),
		})
	}
	for i := 0; i < 5; i++ {
		items = append(items, models.KnapsackItem{
			ID: "zero_" + itoa(i), Content: content, Value: 0, Weight: tokensOf(content),
		})
	}
	// 65 items x 30 tokens = 1950 tokens, under the 4096 budget.

	res := opt.Optimize(items, nil)

	pos, neg, zero := 0, 0, 0
	for _, it := range res.Selected {
		switch {
		case it.Value > 0:
			pos++
		case it.Value < 0:
			neg++
		default:
			zero++
		}
	}
	t.Logf("mixed pool (30 pos / 30 neg / 5 zero, 1950 tokens offered, budget %d): "+
		"selected pos=%d neg=%d zero=%d, tokens=%d", defaultBudget, pos, neg, zero, res.TotalTokens)

	if neg != 0 {
		t.Errorf("negative-value items admitted: %d", neg)
	}
	if pos != 30 {
		t.Errorf("expected all 30 positive items to fit, got %d", pos)
	}
	// Documented waste: zero-value items pass "Density >= lambda" when lambda
	// is 0 and consume real budget for no information gain.
	if zero != 5 {
		t.Errorf("expected the 5 zero-value items to be admitted (Density 0 >= lambda 0), got %d", zero)
	}
	t.Logf("zero-value items consumed %d tokens (%.1f%% of budget) for 0.0 value",
		zero*30, 100*float64(zero*30)/float64(defaultBudget))
}

// The one hole in the guard: Phase 2 only recomputes Density when Weight > 0,
// so a caller-supplied Density on a zero-weight item survives into Phase 5 and
// a negative-VALUE item can be selected. Not reachable from workspace.go, which
// floors tokenCount at 1, but the optimizer itself does not defend against it.
func TestZeroWeightItemBypassesTheDensityGuard(t *testing.T) {
	opt := newDefaultOptimizer()

	items := []models.KnapsackItem{
		{ID: "trojan", Content: "", Value: -100, Weight: 0, Density: 5.0},
		{ID: "honest", Content: filler(400), Value: 2.0, Weight: 100},
	}
	res := opt.Optimize(items, nil)

	trojanIn := false
	for _, it := range res.Selected {
		if it.ID == "trojan" {
			trojanIn = true
		}
	}
	t.Logf("zero-weight item with a stale positive Density and Value=-100: selected=%v; "+
		"result TotalTokens=%d TotalValue=%.2f", trojanIn, res.TotalTokens, res.TotalValue)

	if !trojanIn {
		t.Fatalf("expected the zero-weight bypass to still exist; if knapsack.go now " +
			"recomputes or validates Density for Weight<=0, invert this test")
	}
	if res.TotalValue >= 0 {
		t.Errorf("expected the negative-value trojan to drag TotalValue below zero, got %.2f",
			res.TotalValue)
	}
}

// -----------------------------------------------------------------------------
// (d) OPTIMALITY GAP: greedy Lagrangian vs an exact 0/1 knapsack DP.
// -----------------------------------------------------------------------------

// exactKnapsack solves 0/1 knapsack exactly by dynamic programming over integer
// weights. dp[w] is the best value using weight <= w, so items with a negative
// value are simply never taken. O(n*W); with n<=18 and W<=600 this is trivial.
func exactKnapsack(items []models.KnapsackItem, budget int) (float64, int) {
	if budget <= 0 {
		return 0, 0
	}
	dp := make([]float64, budget+1)
	take := make([][]bool, len(items))
	for i := range take {
		take[i] = make([]bool, budget+1)
	}
	for i, it := range items {
		if it.Weight <= 0 || it.Weight > budget {
			continue
		}
		for w := budget; w >= it.Weight; w-- {
			if cand := dp[w-it.Weight] + it.Value; cand > dp[w] {
				dp[w] = cand
				take[i][w] = true
			}
		}
	}
	// Recover the chosen weight for reporting.
	w, count := budget, 0
	for i := len(items) - 1; i >= 0; i-- {
		if w >= 0 && w < len(take[i]) && take[i][w] {
			count++
			w -= items[i].Weight
		}
	}
	return dp[budget], count
}

// genSmallInstance builds a tractable instance: n items, weights 20..160
// tokens, DIG-like values over [-0.5, 3.0] so negatives are present.
func genSmallInstance(seed int64, n int) []models.KnapsackItem {
	rng := rand.New(rand.NewSource(seed))
	items := make([]models.KnapsackItem, n)
	for i := 0; i < n; i++ {
		w := 20 + rng.Intn(141)
		items[i] = models.KnapsackItem{
			ID:      "s" + itoa(i),
			Content: filler(w * 4),
			Value:   -0.5 + rng.Float64()*3.5,
			Weight:  w,
		}
	}
	return items
}

func TestOptimalityGapVersusExactDP(t *testing.T) {
	const (
		n         = 18
		smallBudg = 600
		seeds     = 30
	)

	opt := NewOptimizer(configs.KnapsackConfig{TokenBudget: smallBudg})

	t.Logf("exact 0/1 knapsack DP vs Optimize, n=%d items, budget=%d tokens, %d seeded instances",
		n, smallBudg, seeds)
	t.Logf("%5s %10s %8s %10s %8s %8s %9s",
		"seed", "greedyVal", "greedyW", "exactVal", "exactN", "ratio", "lostVal")

	minRatio, sumRatio := 2.0, 0.0
	exactHits := 0

	for seed := int64(1); seed <= seeds; seed++ {
		items := genSmallInstance(seed, n)

		exactVal, exactCount := exactKnapsack(items, smallBudg)

		// Optimize sorts its argument in place, so hand it a copy.
		res := opt.Optimize(append([]models.KnapsackItem(nil), items...), nil)

		if res.TotalTokens > smallBudg {
			t.Errorf("seed=%d: budget breached without forced turns: %d > %d",
				seed, res.TotalTokens, smallBudg)
		}
		if res.TotalValue > exactVal+1e-9 {
			t.Fatalf("seed=%d: greedy value %.6f beats the exact optimum %.6f; "+
				"the DP reference is wrong", seed, res.TotalValue, exactVal)
		}

		ratio := 1.0
		if exactVal > 0 {
			ratio = res.TotalValue / exactVal
		}
		if ratio < minRatio {
			minRatio = ratio
		}
		sumRatio += ratio
		if ratio > 1-1e-9 {
			exactHits++
		}

		t.Logf("%5d %10.4f %8d %10.4f %8d %8.4f %9.4f",
			seed, res.TotalValue, res.TotalTokens, exactVal, exactCount, ratio, exactVal-res.TotalValue)
	}

	t.Logf("OPTIMALITY GAP over %d instances: worst ratio=%.4f  mean ratio=%.4f  "+
		"exact optimum hit %d/%d times (%.1f%%)",
		seeds, minRatio, sumRatio/float64(seeds), exactHits, seeds,
		100*float64(exactHits)/float64(seeds))

	// A greedy density heuristic on 0/1 knapsack has no constant-factor
	// guarantee in general. Assert only the guarantee we can defend: it never
	// exceeds the optimum (checked above) and it is not catastrophically bad.
	if minRatio < 0.5 {
		t.Errorf("worst-case ratio %.4f fell below 0.5; the greedy heuristic is "+
			"weaker than previously measured", minRatio)
	}
}

// Widening the instance size shows how the gap moves with n at a fixed budget.
func TestOptimalityGapAcrossInstanceSizes(t *testing.T) {
	const smallBudg = 600
	opt := NewOptimizer(configs.KnapsackConfig{TokenBudget: smallBudg})

	t.Logf("budget=%d, 30 seeds per size", smallBudg)
	t.Logf("%5s %11s %11s %12s", "n", "worstRatio", "meanRatio", "exactHits")

	for _, n := range []int{5, 8, 12, 16, 20} {
		minRatio, sumRatio, hits := 2.0, 0.0, 0
		for seed := int64(1); seed <= 30; seed++ {
			items := genSmallInstance(seed*1000+int64(n), n)
			exactVal, _ := exactKnapsack(items, smallBudg)
			res := opt.Optimize(append([]models.KnapsackItem(nil), items...), nil)

			if res.TotalValue > exactVal+1e-9 {
				t.Fatalf("n=%d seed=%d: greedy %.6f > exact %.6f", n, seed, res.TotalValue, exactVal)
			}
			ratio := 1.0
			if exactVal > 0 {
				ratio = res.TotalValue / exactVal
			}
			if ratio < minRatio {
				minRatio = ratio
			}
			sumRatio += ratio
			if ratio > 1-1e-9 {
				hits++
			}
		}
		t.Logf("%5d %11.4f %11.4f %10d/30", n, minRatio, sumRatio/30, hits)
	}
}

// -----------------------------------------------------------------------------
// Side effects worth knowing about.
// -----------------------------------------------------------------------------

// Optimize mutates its argument: Phase 2 writes Density into the caller's
// backing array and Phase 3 sorts it in place. Documented, not endorsed.
func TestOptimizeMutatesCallerSlice(t *testing.T) {
	items := genCandidates(99, 50)
	before := append([]models.KnapsackItem(nil), items...)

	newDefaultOptimizer().Optimize(items, nil)

	reordered, densitiesWritten := 0, 0
	for i := range items {
		if items[i].ID != before[i].ID {
			reordered++
		}
		if before[i].Density == 0 && items[i].Density != 0 {
			densitiesWritten++
		}
	}
	t.Logf("Optimize mutated the caller's slice: %d/%d positions reordered, "+
		"%d Density fields written in place", reordered, len(items), densitiesWritten)

	if reordered == 0 {
		t.Errorf("expected the in-place sort to reorder the caller's slice; if " +
			"knapsack.go now copies, invert this test")
	}
}

// Empty and degenerate inputs must not panic and must report zero usage.
func TestEmptyAndDegenerateInputs(t *testing.T) {
	opt := newDefaultOptimizer()

	res := opt.Optimize(nil, nil)
	if res.TotalTokens != 0 || len(res.Selected) != 0 || res.Utilization != 0 {
		t.Errorf("empty input: got tokens=%d selected=%d util=%.4f",
			res.TotalTokens, len(res.Selected), res.Utilization)
	}

	// A single candidate larger than the whole budget must be dropped, not
	// truncated and not admitted.
	huge := filler(defaultBudget * 8)
	res = opt.Optimize([]models.KnapsackItem{
		{ID: "huge", Content: huge, Value: 1000, Weight: tokensOf(huge)},
	}, nil)
	t.Logf("single %d-token candidate against a %d-token budget: selected=%d tokens=%d",
		tokensOf(huge), defaultBudget, len(res.Selected), res.TotalTokens)
	if res.TotalTokens > defaultBudget {
		t.Errorf("oversized candidate admitted: %d tokens", res.TotalTokens)
	}
}
