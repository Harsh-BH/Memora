package retrieval

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/graphstore"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/metrics"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/vectorstore"
)

// Service implements the concurrent hybrid retrieval path.
// This is the "Wake" mode read path — no writes occur here.
//
// Architecture:
//   - Routine A: Qdrant Top-K cosine similarity (episodic memory)
//   - Routine B: Neo4j 2-hop traversal (semantic memory)
//
// Both routines execute concurrently via goroutines, and results are
// merged and deduplicated before being passed to DIG reranking.
type Service struct {
	vectorDB    vectorstore.VectorStore
	graphDB     graphstore.GraphStore
	llmProvider llm.Provider
	cfg         configs.RetrievalConfig
	metrics     *metrics.Metrics
}

// NewService creates a new hybrid retrieval service.
func NewService(
	vectorDB vectorstore.VectorStore,
	graphDB graphstore.GraphStore,
	llmProvider llm.Provider,
	cfg configs.RetrievalConfig,
	m *metrics.Metrics,
) *Service {
	return &Service{
		vectorDB:    vectorDB,
		graphDB:     graphDB,
		llmProvider: llmProvider,
		cfg:         cfg,
		metrics:     m,
	}
}

// Retrieve executes concurrent hybrid retrieval and returns merged results.
func (s *Service) Retrieve(ctx context.Context, userID string, query string) ([]models.RetrievalResult, error) {
	start := time.Now()
	defer func() {
		s.metrics.RetrievalLatency.Observe(time.Since(start).Seconds())
	}()

	// Create a context with timeout for the retrieval.
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	// Generate query embedding for vector search.
	queryEmbedding, err := s.llmProvider.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query embedding: %w", err)
	}

	// Extract entities from query for graph traversal.
	entities := s.extractEntities(query)

	var (
		vectorResults []models.RetrievalResult
		graphResults  []models.RetrievalResult
		vectorErr     error
		graphErr      error
		wg            sync.WaitGroup
	)

	// Routine A: Qdrant Top-K cosine similarity search.
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.metrics.VectorSearchCount.Inc()
		vectorResults, vectorErr = s.vectorDB.Search(ctx, userID, queryEmbedding, s.cfg.VectorTopK)
		if vectorErr != nil {
			slog.Error("vector search failed", "error", vectorErr)
		}
	}()

	// Routine B: Neo4j 2-hop graph traversal.
	if len(entities) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.metrics.GraphSearchCount.Inc()
			graphResults, graphErr = s.graphDB.TraverseHops(ctx, userID, entities, s.cfg.GraphMaxHops)
			if graphErr != nil {
				slog.Error("graph search failed", "error", graphErr)
			}
		}()
	}

	wg.Wait()

	// Handle errors gracefully — partial results are acceptable.
	if vectorErr != nil && graphErr != nil {
		return nil, fmt.Errorf("both retrievals failed: vector=%w, graph=%v", vectorErr, graphErr)
	}

	// Merge and deduplicate results.
	merged := s.mergeResults(vectorResults, graphResults)

	slog.Info("retrieval completed",
		"user_id", userID,
		"vector_results", len(vectorResults),
		"graph_results", len(graphResults),
		"merged_results", len(merged),
	)

	return merged, nil
}

// mergeResults combines vector and graph results, deduplicating by content.
func (s *Service) mergeResults(vectorResults, graphResults []models.RetrievalResult) []models.RetrievalResult {
	seen := make(map[string]bool)
	var merged []models.RetrievalResult

	// Vector results first (higher priority for semantic matching).
	for _, r := range vectorResults {
		key := contentKey(r)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, r)
		}
	}

	// Graph results supplement with knowledge graph facts.
	for _, r := range graphResults {
		key := contentKey(r)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, r)
		}
	}

	return merged
}

// extractEntities performs simple entity extraction from the query.
// In production, this would use NER or the LLM for extraction.
func (s *Service) extractEntities(query string) []string {
	// Heuristic entity extraction: extract capitalized words and noun phrases.
	words := strings.Fields(query)
	var entities []string

	for _, word := range words {
		cleaned := strings.Trim(word, ".,!?;:'\"()[]")
		if len(cleaned) < 2 {
			continue
		}
		// Capitalize check: entities are likely proper nouns.
		if len(cleaned) > 0 && cleaned[0] >= 'A' && cleaned[0] <= 'Z' {
			entities = append(entities, cleaned)
		}
	}

	// Also include longer noun phrases (2-grams of capitalized words).
	for i := 0; i < len(words)-1; i++ {
		w1 := strings.Trim(words[i], ".,!?;:'\"()[]")
		w2 := strings.Trim(words[i+1], ".,!?;:'\"()[]")
		if len(w1) > 0 && len(w2) > 0 &&
			w1[0] >= 'A' && w1[0] <= 'Z' &&
			w2[0] >= 'A' && w2[0] <= 'Z' {
			entities = append(entities, w1+" "+w2)
		}
	}

	return entities
}

// contentKey generates a deduplication key from a retrieval result.
func contentKey(r models.RetrievalResult) string {
	if r.Episode != nil {
		return "ep:" + r.Episode.ID
	}
	if len(r.GraphFacts) > 0 {
		key := "gf:"
		for _, f := range r.GraphFacts {
			key += f.Subject + ":" + f.Predicate + ":" + f.Object + "|"
		}
		return key
	}
	return ""
}
