package consolidation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/metrics"

	"github.com/memora/cma/internal/vectorstore"
)

const (
	// TaskTypeConsolidate is the Asynq task type for consolidation jobs.
	TaskTypeConsolidate = "consolidation:process"
)

// Worker implements the CMA consolidation engine (the "Sleep" cycle).
//
// This is the DIFFERENTIATING feature of the CMA architecture.
// It is the ONLY component allowed to write to the knowledge graph.
//
// Pipeline (from the CMA paper Section 4.4.1):
//  1. Trigger: 15 min inactivity OR >10 unconsolidated episodes
//  2. Clustering: DBSCAN over episode embeddings
//  3. Abstraction: LLM generates "Gist" per cluster
//  4. Integration: Check Neo4j for conflicts, resolve if found
//  5. Graph Update: Insert new semantic triples
//  6. Forgetting: Mark episodes as consolidated, apply decay
type Worker struct {
	vectorDB   vectorstore.VectorStore
	llmProvider llm.Provider
	clusterer  *DBSCAN
	resolver   *ConflictResolver
	cfg        configs.ConsolidationConfig
	metrics    *metrics.Metrics
}

// NewWorker creates a new consolidation worker.
func NewWorker(
	vectorDB vectorstore.VectorStore,
	llmProvider llm.Provider,
	clusterer *DBSCAN,
	resolver *ConflictResolver,
	cfg configs.ConsolidationConfig,
	m *metrics.Metrics,
) *Worker {
	return &Worker{
		vectorDB:    vectorDB,
		llmProvider: llmProvider,
		clusterer:   clusterer,
		resolver:    resolver,
		cfg:         cfg,
		metrics:     m,
	}
}

// ConsolidationPayload is serialized into the Asynq task payload.
type ConsolidationPayload struct {
	UserID string `json:"user_id"`
}

// NewConsolidateTask creates a new Asynq consolidation task for a user.
func NewConsolidateTask(userID string) (*asynq.Task, error) {
	payload, err := json.Marshal(ConsolidationPayload{UserID: userID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeConsolidate, payload, asynq.MaxRetry(3), asynq.Timeout(5*time.Minute)), nil
}

// ProcessTask is the Asynq task handler for consolidation jobs.
// This implements the full Sleep cycle pipeline.
func (w *Worker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	start := time.Now()
	defer func() {
		w.metrics.ConsolidationLatency.Observe(time.Since(start).Seconds())
		w.metrics.ConsolidationRuns.Inc()
	}()

	var payload ConsolidationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal consolidation payload: %w", err)
	}

	userID := payload.UserID
	slog.Info("consolidation started", "user_id", userID)

	// Step 1: Fetch unconsolidated episodes from Qdrant.
	episodes, err := w.vectorDB.GetUnconsolidated(ctx, userID, 100)
	if err != nil {
		return fmt.Errorf("fetch unconsolidated: %w", err)
	}

	if len(episodes) == 0 {
		slog.Info("no unconsolidated episodes", "user_id", userID)
		return nil
	}

	slog.Info("episodes fetched", "user_id", userID, "count", len(episodes))

	// Step 2: Clustering — DBSCAN over episode embeddings.
	clusters := w.clusterer.Cluster(episodes)
	w.metrics.ClustersFormed.Observe(float64(len(clusters)))

	slog.Info("clustering completed", "user_id", userID, "clusters", len(clusters))

	totalConflicts := 0
	totalTriples := 0
	var consolidatedIDs []string

	for _, cluster := range clusters {
		if len(cluster.Episodes) == 0 {
			continue
		}

		// Step 3: Abstraction — LLM generates gist for each cluster.
		gist, err := w.llmProvider.Synthesize(ctx, cluster.Episodes)
		if err != nil {
			slog.Error("synthesis failed", "user_id", userID, "cluster_id", cluster.ID, "error", err)
			continue
		}

		// Step 3b: Extract atomic triples from the gist.
		triples, err := w.llmProvider.ExtractTriples(ctx, gist)
		if err != nil {
			slog.Error("triple extraction failed", "user_id", userID, "cluster_id", cluster.ID, "error", err)
			continue
		}

		w.metrics.TriplesExtracted.Add(float64(len(triples)))

		// Steps 4-5: Conflict resolution and graph insertion.
		sourceEpID := cluster.Episodes[0].ID // use first episode as provenance
		conflicts, inserted, err := w.resolver.ResolveAndInsert(ctx, userID, triples, sourceEpID)
		if err != nil {
			slog.Error("resolve and insert failed", "user_id", userID, "error", err)
			continue
		}

		totalConflicts += conflicts
		totalTriples += inserted

		// Collect episode IDs for consolidation marking.
		for _, ep := range cluster.Episodes {
			consolidatedIDs = append(consolidatedIDs, ep.ID)
		}
	}

	w.metrics.ConflictsDetected.Add(float64(totalConflicts))
	w.metrics.ConflictsResolved.Add(float64(totalConflicts))

	// Step 6: Forgetting — mark episodes as consolidated.
	if len(consolidatedIDs) > 0 {
		if err := w.vectorDB.MarkConsolidated(ctx, consolidatedIDs); err != nil {
			slog.Error("mark consolidated failed", "user_id", userID, "error", err)
			return fmt.Errorf("mark consolidated: %w", err)
		}

		// Apply decay to consolidated episodes.
		if err := w.vectorDB.UpdateDecay(ctx, consolidatedIDs, w.cfg.DecayRate); err != nil {
			slog.Error("update decay failed", "user_id", userID, "error", err)
		}

		w.metrics.EpisodesConsolidated.Add(float64(len(consolidatedIDs)))
	}

	slog.Info("consolidation completed",
		"user_id", userID,
		"clusters", len(clusters),
		"triples_inserted", totalTriples,
		"conflicts_resolved", totalConflicts,
		"episodes_consolidated", len(consolidatedIDs),
		"latency_ms", time.Since(start).Milliseconds(),
	)

	return nil
}

// ShouldConsolidate checks if a user needs consolidation based on CMA triggers:
//   - >N unconsolidated episodes
//   - Inactivity timeout exceeded
func (w *Worker) ShouldConsolidate(ctx context.Context, userID string, lastActivity time.Time) (bool, string) {
	// Check inactivity timeout (15 min default).
	if time.Since(lastActivity) > w.cfg.InactivityTimeout {
		return true, "inactivity_timeout"
	}

	// Check unconsolidated episode count (>10 default).
	count, err := w.vectorDB.CountUnconsolidated(ctx, userID)
	if err != nil {
		slog.Error("count unconsolidated", "user_id", userID, "error", err)
		return false, ""
	}

	if count >= w.cfg.MaxUnconsolidated {
		return true, "max_unconsolidated"
	}

	return false, ""
}

// RegisterHandler registers the consolidation task handler with the Asynq server mux.
func (w *Worker) RegisterHandler(mux *asynq.ServeMux) {
	mux.HandleFunc(TaskTypeConsolidate, w.ProcessTask)
}
