package vectorstore

import (
	"context"

	"github.com/memora/cma/internal/models"
)

// VectorStore defines the interface for the episodic memory vector database.
// In CMA, this represents the hippocampal fast-write store.
type VectorStore interface {
	// EnsureCollection creates the vector collection with HNSW index if it does not exist.
	EnsureCollection(ctx context.Context) error

	// Upsert inserts or updates episodic fragments in the vector store.
	Upsert(ctx context.Context, episodes []models.Episode) error

	// Search performs cosine similarity search filtered by user_id.
	// Returns up to topK results.
	Search(ctx context.Context, userID string, queryVector []float32, topK int) ([]models.RetrievalResult, error)

	// GetUnconsolidated retrieves episodes that have not yet been consolidated, for a given user.
	GetUnconsolidated(ctx context.Context, userID string, limit int) ([]models.Episode, error)

	// MarkConsolidated updates the consolidation_status of the given episode IDs to "consolidated".
	MarkConsolidated(ctx context.Context, ids []string) error

	// UpdateDecay updates the decay_factor for the given episode IDs.
	UpdateDecay(ctx context.Context, ids []string, decayFactor float64) error

	// DeleteByIDs removes episodes by their IDs.
	DeleteByIDs(ctx context.Context, ids []string) error

	// CountUnconsolidated returns the number of unconsolidated episodes for a user.
	CountUnconsolidated(ctx context.Context, userID string) (int, error)

	// Close releases resources.
	Close() error
}
