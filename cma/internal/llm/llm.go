package llm

import (
	"context"

	"github.com/memora/cma/internal/models"
)

// Provider defines the interface for LLM operations required by the CMA system.
// This abstraction allows pluggability of different LLM backends (OpenAI, Anthropic, local models).
type Provider interface {
	// Embed generates a dense vector embedding for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// GetTokenProbabilities returns per-token log probabilities for the input text.
	// Used by the surprisal segmentation engine.
	GetTokenProbabilities(ctx context.Context, text string) ([]TokenProb, error)

	// ExtractTriples extracts atomic (Subject, Predicate, Object) triples from a text cluster.
	// Used during consolidation (Sleep cycle).
	ExtractTriples(ctx context.Context, content string) ([]models.Triple, error)

	// Synthesize generates a gist/summary proposition from a cluster of episodes.
	// Used during consolidation to create semantic abstractions.
	Synthesize(ctx context.Context, episodes []models.Episode) (string, error)

	// ScoreDIG computes the Document Information Gain for a candidate document.
	// DIG(d|x) = log P(y|x,d) - log P(y|x)
	ScoreDIG(ctx context.Context, query string, document string) (float64, error)

	// Generate produces a completion given a prompt (for general-purpose use).
	Generate(ctx context.Context, prompt string) (string, error)

	// CountTokens returns the approximate token count for the given text.
	CountTokens(text string) int
}

// TokenProb holds a token and its log probability.
type TokenProb struct {
	Token  string  `json:"token"`
	LogProb float64 `json:"log_prob"`
	Offset int     `json:"offset"`
}
