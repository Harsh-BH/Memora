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
//
// The true DIG score (llm.Provider.ScoreDIG, a two-call logprob delta) is no
// longer reachable: it requires a logprobs-capable chat completions API, and
// neither available provider offers one here (no OPENAI_API_KEY is set, and
// Claude's Messages API exposes no logprobs surface at all -- a different,
// permanent wall, not a missing key). Calling it would only ever hit the
// existing error-fallback below, so heuristicScore is promoted to primary
// instead of leaving a doomed call in the hot path.
func (r *Reranker) Rerank(ctx context.Context, query string, candidates []models.RetrievalResult) ([]models.DIGCandidate, error) {
	scored := make([]models.DIGCandidate, 0, len(candidates))

	for _, candidate := range candidates {
		content := extractContent(candidate)
		if content == "" {
			continue
		}

		scored = append(scored, models.DIGCandidate{
			Result:   candidate,
			DIGScore: r.heuristicScore(candidate),
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

// heuristicScore approximates DIG without an LLM call. Uses a combination of:
//   - Cosine similarity score (from vector search)
//   - Recency decay (exponential decay based on age)
//   - Importance and decay-factor weighting from consolidation
//
// No longer includes a surprisal term: the segmenter that produced
// SurprisalValue was dead machinery (see segmentation/structural.go) and
// every episode's value is now a neutral 0, so that term always weighted on
// a constant. Dropped rather than left in place scoring nothing.
func (r *Reranker) heuristicScore(result models.RetrievalResult) float64 {
	score := result.Score // cosine similarity baseline

	if result.Episode != nil {
		// Recency boost: exponential decay with half-life of 24 hours.
		age := time.Since(result.Episode.Timestamp).Hours()
		recencyBoost := math.Exp(-age / 24.0)
		score += 0.3 * recencyBoost

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
