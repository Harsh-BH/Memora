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
	//
	// BUG 3 of 3, found 2026-08-31 by cma/eval/hybrid_eval_test.go: graph
	// results are APPENDED after every vector result, unconditionally and
	// unsorted. With the shipped config (configs/config.yaml vector_top_k: 20)
	// the vector arm alone fills positions 0-19, so a graph result can never
	// land above position 20 -- i.e. never inside a top-10 cutoff.
	//
	// MEASURED: across 110 eval queries the surviving graph result landed at
	// merged position 20 every single time, and appeared in the top-10 on
	// 0/110 queries. Hybrid retrieval returned the identical top-10 episode
	// order as plain cosine on 110/110 queries.
	//
	// This is also the complete explanation for the long-standing "DIG is a
	// null" result. dig.heuristicScore only adds its graph-confidence term
	// (dig.go:108-112) to results that CARRY GraphFacts, and a vector result
	// never does -- merge drops the graph result instead of attaching its
	// facts to the matching document. So graph evidence cannot corroborate a
	// document, only sit below it.
	//
	// FIX is a design decision, not a typo: attach GraphFacts to the vector
	// result with the same source episode ID (roughly 10 lines: build an
	// episode-ID -> index map over merged, append facts on hit, append the
	// result on miss), which is what would let DIG's graph term actually
	// score something. Alternatively interleave both arms by score. Left
	// alone deliberately -- picking fusion semantics needs its own round.
	for _, r := range graphResults {
		key := contentKey(r)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, r)
		}
	}

	return merged
}

// ExtractEntities exposes the production entity heuristic below, unchanged, so
// an eval harness can populate a graph using the exact same rule the query path
// uses to pick its traversal seeds. Wrapper only -- no behavior of its own.
func (s *Service) ExtractEntities(text string) []string { return s.extractEntities(text) }

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
//
// BUG 2 of 3, found 2026-08-31 by cma/eval/hybrid_eval_test.go (the first
// run of this path with Neo4j actually populated): every graph result
// collapses onto the single key "ep:".
//
// graphstore.TraverseHops (neo4j.go:208-210) builds each graph result with
// Episode: &models.Episode{Content: factStr} -- non-nil, but with no ID set.
// So the r.Episode != nil branch below fires for EVERY graph result and
// returns "ep:" + "" == "ep:". They therefore all collide on one dedup slot
// in mergeResults, and at most ONE of a query's graph facts survives.
//
// MEASURED, not inferred: over 110 eval queries, TraverseHops returned 438
// facts and exactly 33 survived mergeResults -- one apiece for the 33
// queries whose seeds matched anything.
//
// It also makes the len(r.GraphFacts) > 0 branch below DEAD CODE for graph
// results. A reader reasonably assumes graph results take the "gf:" path;
// they never do, because Episode is always non-nil for them. That branch is
// only reachable for a hand-constructed result with GraphFacts and a nil
// Episode, which nothing in this codebase produces.
//
// FIX: test len(r.GraphFacts) > 0 BEFORE r.Episode != nil, or key graph
// results on their source episode ID once BUG 1 supplies one. Two lines.
// Not fixed here: doing so changes what reaches DIG and the knapsack, and
// that deserves its own measured round rather than a drive-by edit.
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
