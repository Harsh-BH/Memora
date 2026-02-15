package graphstore

import (
	"context"

	"github.com/memora/cma/internal/models"
)

// GraphStore defines the interface for the semantic memory knowledge graph.
// In CMA, this represents the neocortical slow-learning store.
// CONSTRAINT: Only the consolidation engine (Sleep worker) may write to this store.
type GraphStore interface {
	// EnsureSchema creates constraints and indexes in the graph database.
	EnsureSchema(ctx context.Context) error

	// InsertTriple creates a new semantic triple with bi-temporal metadata.
	// Only called during consolidation (Sleep cycle).
	InsertTriple(ctx context.Context, userID string, triple models.Triple, sourceEpID string) error

	// QueryBySubject retrieves all relationships for a given subject entity.
	QueryBySubject(ctx context.Context, userID string, subject string) ([]models.GraphRelationship, error)

	// TraverseHops performs a multi-hop graph traversal starting from seed entities.
	// maxHops controls the depth of traversal (typically 2).
	TraverseHops(ctx context.Context, userID string, seedEntities []string, maxHops int) ([]models.RetrievalResult, error)

	// FindConflicts checks if a new triple conflicts with existing facts.
	FindConflicts(ctx context.Context, userID string, triple models.Triple) ([]models.ConflictRecord, error)

	// ResolveConflict applies temporal decay to an old relationship and optionally
	// closes its validity window.
	ResolveConflict(ctx context.Context, conflict models.ConflictRecord, decayRate float64) error

	// GetStats retrieves statistics about the knowledge graph.
	GetStats(ctx context.Context, userID string) (map[string]interface{}, error)

	// Close releases database resources.
	Close(ctx context.Context) error
}
