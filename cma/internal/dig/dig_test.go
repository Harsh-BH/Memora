package dig

import (
	"context"
	"testing"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
)

// TestRerankNeverCallsLLM proves Rerank no longer attempts the (now
// unreachable) ScoreDIG call: NewReranker is given a nil llm.Provider, which
// would panic on any method call, and Rerank must still complete cleanly.
func TestRerankNeverCallsLLM(t *testing.T) {
	r := NewReranker(nil, configs.DIGConfig{MinScore: -0.5})

	candidates := []models.RetrievalResult{
		{
			Episode: &models.Episode{Content: "some memory", Timestamp: time.Now(), DecayFactor: 1.0},
			Score:   0.8,
		},
	}

	out, err := r.Rerank(context.Background(), "query", candidates)
	if err != nil {
		t.Fatalf("Rerank returned error: %v (should never touch the nil provider)", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 candidate to survive, got %d", len(out))
	}
}

// TestHeuristicScoreIgnoresSurprisal proves the dropped surprisal term no
// longer affects the score: two otherwise-identical episodes differing only
// in SurprisalValue must score identically.
func TestHeuristicScoreIgnoresSurprisal(t *testing.T) {
	r := NewReranker(nil, configs.DIGConfig{MinScore: -0.5})
	now := time.Now()

	low := models.RetrievalResult{
		Score:   0.5,
		Episode: &models.Episode{Timestamp: now, DecayFactor: 1.0, SurprisalValue: 0, ImportanceScore: 0.2},
	}
	high := models.RetrievalResult{
		Score:   0.5,
		Episode: &models.Episode{Timestamp: now, DecayFactor: 1.0, SurprisalValue: 9.9, ImportanceScore: 0.2},
	}

	scoreLow := r.heuristicScore(low)
	scoreHigh := r.heuristicScore(high)

	// Epsilon, not exact equality: the two calls straddle a nanosecond of
	// wall-clock time, so the recency term (time.Since(Timestamp)) alone
	// differs in the last bits between calls. A surprisal contribution
	// would separate these by ~0.2*log1p(9.9)/5 ≈ 0.0485, far above noise.
	const epsilon = 1e-6
	if diff := scoreHigh - scoreLow; diff < -epsilon || diff > epsilon {
		t.Fatalf("SurprisalValue still affects heuristicScore: %.9f (SurprisalValue=0) vs %.9f (SurprisalValue=9.9), diff=%.9f",
			scoreLow, scoreHigh, diff)
	}
	t.Logf("score with SurprisalValue=0: %.6f, with SurprisalValue=9.9: %.6f (equal, as expected)", scoreLow, scoreHigh)
}
