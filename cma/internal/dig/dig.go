package dig

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
)

// Reranker implements Document Information Gain (DIG) reranking.
//
// Mathematical basis (from the CMA paper):
//
//	DIG(d|x) = log P(y|x,d) - log P(y|x)
//
// - If DIG > 0: document actively helps generate the correct answer.
// - If DIG ≈ 0: document is irrelevant.
// - If DIG < 0: document is a distractor (hallucination inducer).
//
// CMA filters out all candidates with DIG ≤ 0.
type Reranker struct {
	llmProvider     llm.Provider
	minScore        float64
	fallbackEnabled bool
}

// NewReranker creates a new DIG reranker.
func NewReranker(provider llm.Provider, cfg configs.DIGConfig) *Reranker {
	return &Reranker{
		llmProvider:     provider,
		minScore:        cfg.MinScore,
		fallbackEnabled: cfg.FallbackEnabled,
	}
}

// Rerank scores and filters retrieval results using Document Information Gain.
// Returns only candidates with DIG > 0, sorted by DIG score descending.
func (r *Reranker) Rerank(ctx context.Context, query string, candidates []models.RetrievalResult) ([]models.DIGCandidate, error) {
	scored := make([]models.DIGCandidate, 0, len(candidates))

	for _, candidate := range candidates {
		content := extractContent(candidate)
		if content == "" {
			continue
		}

		var digScore float64
		var err error

		// Try cross-encoder DIG scoring via LLM.
		digScore, err = r.llmProvider.ScoreDIG(ctx, query, content)
		if err != nil {
			if !r.fallbackEnabled {
				continue
			}
			// Fallback: heuristic scoring based on cosine similarity, recency, and surprisal.
			digScore = r.heuristicScore(candidate)
		}

		scored = append(scored, models.DIGCandidate{
			Result:   candidate,
			DIGScore: digScore,
			Content:  content,
		})
	}

	// Filter: remove DIG ≤ 0 candidates (distractors).
	filtered := make([]models.DIGCandidate, 0, len(scored))
	for _, c := range scored {
		if c.DIGScore > r.minScore {
			filtered = append(filtered, c)
		}
	}

	// Sort by DIG score descending.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].DIGScore > filtered[j].DIGScore
	})

	return filtered, nil
}

// heuristicScore computes a fallback DIG approximation when the LLM cross-encoder
// is unavailable. Uses a combination of:
//   - Cosine similarity score (from vector search)
//   - Recency decay (exponential decay based on age)
//   - Surprisal value (high-surprise events are more salient)
func (r *Reranker) heuristicScore(result models.RetrievalResult) float64 {
	score := result.Score // cosine similarity baseline

	if result.Episode != nil {
		// Recency boost: exponential decay with half-life of 24 hours.
		age := time.Since(result.Episode.Timestamp).Hours()
		recencyBoost := math.Exp(-age / 24.0)
		score += 0.3 * recencyBoost

		// Surprisal boost: high-surprise events are memory landmarks.
		if result.Episode.SurprisalValue > 0 {
			surprisalBoost := math.Log1p(result.Episode.SurprisalValue) / 5.0
			score += 0.2 * surprisalBoost
		}

		// Importance score contribution.
		score += 0.1 * result.Episode.ImportanceScore

		// Decay factor penalty.
		score *= result.Episode.DecayFactor
	}

	// Graph facts get a baseline positive score.
	if len(result.GraphFacts) > 0 {
		for _, fact := range result.GraphFacts {
			score += 0.15 * fact.Confidence
		}
	}

	return score
}

// extractContent retrieves the textual content from a RetrievalResult.
func extractContent(result models.RetrievalResult) string {
	if result.Episode != nil && result.Episode.Content != "" {
		return result.Episode.Content
	}

	// For graph-only results, construct content from triples.
	if len(result.GraphFacts) > 0 {
		content := ""
		for _, fact := range result.GraphFacts {
			content += fact.Subject + " " + fact.Predicate + " " + fact.Object + ". "
		}
		return content
	}

	return ""
}
