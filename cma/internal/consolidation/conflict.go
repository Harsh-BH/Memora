package consolidation

import (
	"context"
	"fmt"
	"log/slog"


	"github.com/memora/cma/internal/graphstore"
	"github.com/memora/cma/internal/models"
)

// ConflictResolver implements the CMA conflict resolution logic.
//
// When the consolidation engine discovers a new fact that contradicts
// an existing fact in the knowledge graph:
//
//  1. If the newer fact conflicts → decay old fact confidence, update
//     valid_to, insert new fact with provenance.
//  2. If no conflict → insert new fact directly.
//
// All conflict resolution respects bi-temporal modeling:
//   - valid_from / valid_to: when the fact is true in the world
//   - transaction_time: when the system recorded the change
//   - source_ep_id: provenance back to the originating episode
type ConflictResolver struct {
	graphDB   graphstore.GraphStore
	decayRate float64
}

// NewConflictResolver creates a new conflict resolver.
func NewConflictResolver(graphDB graphstore.GraphStore, decayRate float64) *ConflictResolver {
	if decayRate <= 0 || decayRate >= 1 {
		decayRate = 0.95
	}
	return &ConflictResolver{
		graphDB:   graphDB,
		decayRate: decayRate,
	}
}

// ResolveAndInsert checks for conflicts and either resolves them or inserts new facts.
// This is the core conflict resolution logic from the CMA paper Section 4.4.1 Step 4-5.
func (cr *ConflictResolver) ResolveAndInsert(ctx context.Context, userID string, triples []models.Triple, sourceEpID string) (int, int, error) {
	conflictsDetected := 0
	triplesInserted := 0

	for _, triple := range triples {
		// Step 3: Check for conflicts in Graph.
		conflicts, err := cr.graphDB.FindConflicts(ctx, userID, triple)
		if err != nil {
			slog.Error("conflict detection failed",
				"user_id", userID,
				"subject", triple.Subject,
				"error", err,
			)
			continue
		}

		if len(conflicts) > 0 {
			// Step 4: Resolve conflict — decay old fact, insert new.
			conflictsDetected += len(conflicts)

			for _, conflict := range conflicts {
				slog.Info("conflict detected",
					"user_id", userID,
					"existing", fmt.Sprintf("%s %s %s", conflict.ExistingTriple.Subject, conflict.ExistingTriple.Predicate, conflict.ExistingTriple.Object),
					"new", fmt.Sprintf("%s %s %s", triple.Subject, triple.Predicate, triple.Object),
				)

				// Apply temporal decay to old fact.
				if err := cr.graphDB.ResolveConflict(ctx, conflict, cr.decayRate); err != nil {
					slog.Error("conflict resolution failed",
						"user_id", userID,
						"conflict_rel_id", conflict.ExistingRelID,
						"error", err,
					)
					continue
				}
			}

			// Insert the new (winning) fact.
			if err := cr.graphDB.InsertTriple(ctx, userID, triple, sourceEpID); err != nil {
				slog.Error("insert new fact after conflict failed",
					"user_id", userID,
					"error", err,
				)
				continue
			}
			triplesInserted++

		} else {
			// Step 5: No conflict — insert new fact directly.
			if err := cr.graphDB.InsertTriple(ctx, userID, triple, sourceEpID); err != nil {
				slog.Error("insert triple failed",
					"user_id", userID,
					"error", err,
				)
				continue
			}
			triplesInserted++
		}
	}

	return conflictsDetected, triplesInserted, nil
}
