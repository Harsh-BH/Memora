package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/memora/cma/internal/metrics"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/segmentation"
	"github.com/memora/cma/internal/vectorstore"
)

// Service orchestrates the ingest pipeline:
// raw input → surprisal segmentation → embedding → Qdrant upsert.
//
// This is the append-only episodic write path (hippocampal encoding).
// All writes here are to the vector store only — graph writes are
// strictly reserved for the consolidation (Sleep) cycle.
type Service struct {
	segmenter  *segmentation.SurprisalEngine
	vectorDB   vectorstore.VectorStore
	metrics    *metrics.Metrics
}

// NewService creates a new ingest pipeline service.
func NewService(
	segmenter *segmentation.SurprisalEngine,
	vectorDB vectorstore.VectorStore,
	m *metrics.Metrics,
) *Service {
	return &Service{
		segmenter: segmenter,
		vectorDB:  vectorDB,
		metrics:   m,
	}
}

// Ingest processes raw text input through the CMA ingest pipeline.
// Returns the generated episode IDs.
func (s *Service) Ingest(ctx context.Context, userID string, content string, role string) (*models.IngestResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.IngestLatency.Observe(time.Since(start).Seconds())
	}()

	slog.Info("ingest started",
		"user_id", userID,
		"content_length", len(content),
		"role", role,
	)

	// Step 1: Surprisal-based segmentation.
	// This produces episodic fragments at event boundaries where
	// S > μ + γσ (Bayesian Surprise threshold).
	episodes, err := s.segmenter.Segment(ctx, userID, content)
	if err != nil {
		return nil, fmt.Errorf("segmentation: %w", err)
	}

	if len(episodes) == 0 {
		return &models.IngestResponse{
			EpisodeIDs: []string{},
			Segments:   0,
			Message:    "no episodes generated",
		}, nil
	}

	// Step 2: Enrich episodes with metadata.
	for i := range episodes {
		episodes[i].UserID = userID
		if episodes[i].Metadata == nil {
			episodes[i].Metadata = make(map[string]any)
		}
		episodes[i].Metadata["role"] = role
		episodes[i].Metadata["ingested_at"] = time.Now().UTC().Format(time.RFC3339)
	}

	s.metrics.SegmentsBoundary.Add(float64(len(episodes) - 1)) // boundaries = segments - 1

	// Step 3: Append-only vector commit to Qdrant.
	if err := s.vectorDB.Upsert(ctx, episodes); err != nil {
		return nil, fmt.Errorf("vector upsert: %w", err)
	}

	// Collect episode IDs.
	ids := make([]string, 0, len(episodes))
	for _, ep := range episodes {
		ids = append(ids, ep.ID)
	}

	s.metrics.EpisodesIngested.Add(float64(len(episodes)))

	slog.Info("ingest completed",
		"user_id", userID,
		"episodes", len(episodes),
		"ids", ids,
	)

	return &models.IngestResponse{
		EpisodeIDs: ids,
		Segments:   len(episodes),
		Message:    fmt.Sprintf("ingested %d episodic fragments", len(episodes)),
	}, nil
}
