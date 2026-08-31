package eval

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/dig"
	"github.com/memora/cma/internal/graphstore"
	"github.com/memora/cma/internal/ingest"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/retrieval"
	"github.com/memora/cma/internal/segmentation"
	"github.com/memora/cma/internal/vectorstore"
)

// neo4jReachable does a cheap TCP dial, matching qdrantReachable's
// skip-vs-run decision style.
func neo4jReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// TestHybridRetrievalEvalWithLiveGraph is the first run of Memora's FULL
// hybrid retrieval path with both stores live. Rounds 3-5 measured the
// Qdrant arm only; Neo4j was never stood up, which is also why every
// previous DIG measurement came out null (heuristicScore's graph-confidence
// term had nothing to score on -- GraphFacts was always empty).
//
// This test drives the REAL production path, retrieval.Service.Retrieve
// (internal/retrieval/service.go), not a reimplementation: the concurrent
// two-goroutine fan-out (Qdrant Search || Neo4j TraverseHops), the
// merge/dedup by contentKey, and the per-hop bi-temporal validity filter in
// graphstore/neo4j.go's TraverseHops all execute for real.
//
// GRAPH POPULATION -- read this before reading any number below.
// ExtractTriples is on the OpenAI path and there is no credential, so LLM
// triple extraction cannot run and remains UNMEASURED. The graph here is
// instead derived mechanically from the corpus text by the SAME production
// heuristic the query path uses to pick its traversal seeds:
// retrieval.Service.ExtractEntities (the capitalized-word + capitalized
// 2-gram rule at service.go's extractEntities). For each of the 100
// documents, that rule yields an ordered entity list; consecutive pairs
// become (subject, "co_occurs_in_document", object) triples inserted via
// the real graphstore.InsertTriple, with the document's episode ID as
// source_ep_id. Documents that mention the same entity therefore become
// connected through that shared node -- which is the whole "link documents
// that share entities" idea, built by code that has never seen a query.
//
// Why this is query-blind: the rule reads only hardCorpus, never
// hardQueries; it is the unmodified production function; and it is
// deterministic, so the graph is a pure function of the corpus. Nothing
// here was tuned to make any of the 110 queries answerable. Confidence is a
// flat 1.0 on every edge because the heuristic supplies no confidence
// signal -- inventing one would be a knob, so there isn't one.
//
// Scope: this measures hybrid retrieval over a graph populated by the
// production entity heuristic. LLM triple extraction, consolidation, and
// knapsack assembly remain unmeasured.
func TestHybridRetrievalEvalWithLiveGraph(t *testing.T) {
	root := filepath.Join("..", "third_party")
	lib := filepath.Join(root, "onnxruntime", "lib", "libonnxruntime.so")
	model := filepath.Join(root, "models", "all-MiniLM-L6-v2", "model_quint8_avx2.onnx")
	vocab := filepath.Join(root, "models", "all-MiniLM-L6-v2", "vocab.txt")
	for _, p := range []string{lib, model, vocab} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("third_party asset missing (%s): %v", p, err)
		}
	}
	if !qdrantReachable("localhost:6334") {
		t.Skip("Qdrant not reachable at localhost:6334")
	}
	if !neo4jReachable("localhost:7687") {
		t.Skip("Neo4j not reachable at localhost:7687 -- see docker-compose.yml")
	}

	// retrieval.Service logs one Info line per Retrieve; 110 of those bury
	// the report. Counts below are computed directly instead.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx := context.Background()

	embedder, err := llm.NewLocalEmbedProvider(lib, model, vocab, 128, 384)
	if err != nil {
		t.Fatalf("NewLocalEmbedProvider: %v", err)
	}
	defer embedder.Close()

	// Fresh Qdrant collection AND a fresh user_id per run: every Neo4j
	// query in graphstore filters on user_id, and InsertTriple MERGEs
	// nodes on {name, user_id}, so a timestamped user gives this run a
	// disjoint subgraph without needing a wipe.
	stamp := time.Now().UnixNano()
	userID := fmt.Sprintf("hybrid-eval-user-%d", stamp)
	collectionName := fmt.Sprintf("cma_hybrideval_minilm384_%d", stamp)

	vectorDB, err := vectorstore.NewQdrantStore(configs.QdrantConfig{
		Host: "localhost", GRPCPort: 6334,
		Collection: collectionName, VectorSize: 384, HnswM: 16, HnswEF: 100,
	})
	if err != nil {
		t.Fatalf("NewQdrantStore: %v", err)
	}
	defer vectorDB.Close()
	if err := vectorDB.EnsureCollection(ctx); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() { dropCollection(t, collectionName) })

	// Same credentials as configs/config.yaml's neo4j block.
	graphDB, err := graphstore.NewNeo4jStore(configs.Neo4jConfig{
		URI: "bolt://localhost:7687", Username: "neo4j",
		Password: "cmapassword", Database: "neo4j",
	})
	if err != nil {
		t.Fatalf("NewNeo4jStore: %v", err)
	}
	defer graphDB.Close(ctx)
	if err := graphDB.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	m := sharedMetrics()
	segCfg := configs.SegmentationConfig{MinEpisodeTokens: 50, MaxEpisodeTokens: 500}
	ingestSvc := ingest.NewService(segmentation.NewStructuralSegmenter(embedder, segCfg), vectorDB, m)

	// configs/config.yaml retrieval block, verbatim.
	retCfg := configs.RetrievalConfig{VectorTopK: 20, GraphMaxHops: 2, Timeout: 10 * time.Second}
	svc := retrieval.NewService(vectorDB, graphDB, embedder, retCfg, m)

	// --- Ingest: one document per call, one episode per document ---
	docEpisodeID := make([]string, len(hardCorpus))
	for i, doc := range hardCorpus {
		resp, err := ingestSvc.Ingest(ctx, userID, doc, "user")
		if err != nil {
			t.Fatalf("Ingest(doc %d): %v", i, err)
		}
		if resp.Segments != 1 {
			t.Fatalf("doc %d produced %d episodes, expected exactly 1", i, resp.Segments)
		}
		docEpisodeID[i] = resp.EpisodeIDs[0]
	}
	if err := waitForCount(ctx, vectorDB, userID, len(hardCorpus), 15*time.Second); err != nil {
		t.Fatalf("points never became visible after ingest: %v", err)
	}

	// --- Populate the graph with the production entity heuristic ---
	graphStart := time.Now()
	nodeSet := map[string]bool{}
	entPerDoc := make([]int, len(hardCorpus))
	edgeCount := 0
	for i, doc := range hardCorpus {
		ents := dedupeStable(svc.ExtractEntities(doc))
		entPerDoc[i] = len(ents)
		for _, e := range ents {
			nodeSet[e] = true
		}
		for j := 0; j+1 < len(ents); j++ {
			tr := models.Triple{
				Subject:    ents[j],
				Predicate:  "co_occurs_in_document",
				Object:     ents[j+1],
				Confidence: 1.0,
			}
			if err := graphDB.InsertTriple(ctx, userID, tr, docEpisodeID[i]); err != nil {
				t.Fatalf("InsertTriple(doc %d, pair %d): %v", i, j, err)
			}
			edgeCount++
		}
	}
	stats, err := graphDB.GetStats(ctx, userID)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	t.Logf("GRAPH POPULATION: %d documents -> %d distinct entity nodes, %d RELATES_TO edges inserted "+
		"(neo4j GetStats reports %v), %.1fs; entities/doc min=%d max=%d",
		len(hardCorpus), len(nodeSet), edgeCount, stats, time.Since(graphStart).Seconds(),
		minOf(entPerDoc), maxIntOf(entPerDoc))

	reranker := dig.NewReranker(nil, configs.DIGConfig{MinScore: -0.5})
	ks := []int{1, 3, 5, 10}
	maxK := ks[len(ks)-1]

	type curve struct {
		hitsAtK map[int]int
		rrSum   float64
		n       int
	}
	cosCurve := curve{hitsAtK: map[int]int{}}
	hybCurve := curve{hitsAtK: map[int]int{}}
	hybDigCurve := curve{hitsAtK: map[int]int{}}

	var (
		queriesWithSeeds     int
		queriesWithGraphHits int
		rawGraphFacts        int
		graphFactsSurviving  int
		traverseErrs         int
		hybridSameAsCosine   int
		digSameAsHybrid      int
		mergedSizes          []int
		cosNoAnswer          []float64
		hybNoAnswer          []float64
		graphInTop10         int
		seedExamples         []string
		maxSurvivorsPerQuery int
		graphMergePos        []int
		graphDigScores       []float64
		vecDigMin            = 1e9
	)

	for qi, q := range hardQueries {
		// --- what the production path will use as graph seeds ---
		seeds := svc.ExtractEntities(q.text)
		if len(seeds) > 0 {
			queriesWithSeeds++
		}
		if qi < 3 {
			seedExamples = append(seedExamples, fmt.Sprintf("%q -> seeds %v", q.text, seeds))
		}

		// --- graph arm, measured on its own (never measured before) ---
		if len(seeds) > 0 {
			gctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			gres, gerr := graphDB.TraverseHops(gctx, userID, seeds, retCfg.GraphMaxHops)
			cancel()
			if gerr != nil {
				traverseErrs++
			} else if len(gres) > 0 {
				queriesWithGraphHits++
				rawGraphFacts += len(gres)
			}
		}

		// --- cosine control, same run, same collection ---
		qVec, err := embedder.Embed(ctx, q.text)
		if err != nil {
			t.Fatalf("embed query %d: %v", qi, err)
		}
		cosPool, err := vectorDB.Search(ctx, userID, qVec, retCfg.VectorTopK)
		if err != nil {
			t.Fatalf("vector search query %d: %v", qi, err)
		}
		cosTop := cosPool
		if len(cosTop) > maxK {
			cosTop = cosTop[:maxK]
		}

		// --- THE REAL HYBRID PATH ---
		merged, err := svc.Retrieve(ctx, userID, q.text)
		if err != nil {
			t.Fatalf("hybrid Retrieve query %d: %v", qi, err)
		}
		mergedSizes = append(mergedSizes, len(merged))
		survivors := 0
		for pos, r := range merged {
			if len(r.GraphFacts) > 0 {
				graphFactsSurviving++
				survivors++
				graphMergePos = append(graphMergePos, pos)
			}
		}
		if survivors > maxSurvivorsPerQuery {
			maxSurvivorsPerQuery = survivors
		}
		hybTop := merged
		if len(hybTop) > maxK {
			hybTop = hybTop[:maxK]
		}
		for _, r := range hybTop {
			if len(r.GraphFacts) > 0 {
				graphInTop10++
				break
			}
		}
		if sameOrder(cosTop, hybTop) {
			hybridSameAsCosine++
		}

		// --- hybrid + heuristic DIG, now with a live graph ---
		digCands, err := reranker.Rerank(ctx, q.text, merged)
		if err != nil {
			t.Fatalf("dig rerank query %d: %v", qi, err)
		}
		digRes := make([]models.RetrievalResult, 0, len(digCands))
		for _, c := range digCands {
			// Evidence for WHY the graph arm cannot reorder anything: compare
			// the DIG score a graph-sourced candidate earns against the worst
			// score any vector-sourced candidate earns.
			if len(c.Result.GraphFacts) > 0 {
				graphDigScores = append(graphDigScores, c.DIGScore)
			} else if c.DIGScore < vecDigMin {
				vecDigMin = c.DIGScore
			}
			digRes = append(digRes, c.Result)
		}
		if len(digRes) > maxK {
			digRes = digRes[:maxK]
		}
		if sameOrder(hybTop, digRes) {
			digSameAsHybrid++
		}

		if q.expected < 0 {
			if len(cosTop) > 0 {
				cosNoAnswer = append(cosNoAnswer, cosTop[0].Score)
			}
			if len(hybTop) > 0 {
				hybNoAnswer = append(hybNoAnswer, hybTop[0].Score)
			}
			continue
		}

		cosRank := rankOf(cosTop, q.expected, docEpisodeID)
		hybRank := rankOf(hybTop, q.expected, docEpisodeID)
		digRank := rankOf(digRes, q.expected, docEpisodeID)

		cosCurve.n++
		hybCurve.n++
		hybDigCurve.n++
		for _, k := range ks {
			if cosRank != 0 && cosRank <= k {
				cosCurve.hitsAtK[k]++
			}
			if hybRank != 0 && hybRank <= k {
				hybCurve.hitsAtK[k]++
			}
			if digRank != 0 && digRank <= k {
				hybDigCurve.hitsAtK[k]++
			}
		}
		if cosRank != 0 {
			cosCurve.rrSum += 1.0 / float64(cosRank)
		}
		if hybRank != 0 {
			hybCurve.rrSum += 1.0 / float64(hybRank)
		}
		if digRank != 0 {
			hybDigCurve.rrSum += 1.0 / float64(digRank)
		}
	}

	report := func(name string, c curve) {
		t.Logf("--- %s: %d answerable queries ---", name, c.n)
		for _, k := range ks {
			t.Logf("  recall@%-2d = %d/%d = %.4f", k, c.hitsAtK[k], c.n, float64(c.hitsAtK[k])/float64(c.n))
		}
		t.Logf("  MRR (cutoff %d) = %.4f", maxK, c.rrSum/float64(c.n))
	}
	t.Logf("=== HYBRID EVAL, BOTH STORES LIVE: %d docs / %d clusters / %d answerable / %d unanswerable ===",
		len(hardCorpus), len(hardCorpus)/5, cosCurve.n, len(cosNoAnswer))
	report("COSINE alone (Qdrant top-20, same run, control)", cosCurve)
	report("HYBRID (retrieval.Service.Retrieve: Qdrant || Neo4j 2-hop, merged)", hybCurve)
	report("HYBRID + heuristic DIG rerank (graph-confidence term now non-empty)", hybDigCurve)

	sort.Ints(mergedSizes)
	t.Logf("GRAPH ARM: %d/%d queries produced >=1 capitalized seed for TraverseHops; "+
		"%d of those returned >=1 fact; %d raw facts returned in total; %d TraverseHops errors/timeouts",
		queriesWithSeeds, len(hardQueries), queriesWithGraphHits, rawGraphFacts, traverseErrs)
	t.Logf("GRAPH ARM seed examples: %v", seedExamples)
	t.Logf("MERGE: %d graph-fact results survived mergeResults across all %d queries "+
		"(raw facts offered: %d); merged list size min=%d median=%d max=%d",
		graphFactsSurviving, len(hardQueries), rawGraphFacts,
		mergedSizes[0], mergedSizes[len(mergedSizes)/2], mergedSizes[len(mergedSizes)-1])
	sort.Ints(graphMergePos)
	gpMin, gpMax := -1, -1
	if len(graphMergePos) > 0 {
		gpMin, gpMax = graphMergePos[0], graphMergePos[len(graphMergePos)-1]
	}
	t.Logf("MERGE COLLAPSE: at most %d graph-fact result survived mergeResults on ANY single query "+
		"(contentKey, service.go:189-191, keys every graph result as \"ep:\" + an EMPTY Episode.ID, "+
		"because TraverseHops at neo4j.go:208 builds an Episode with Content but no ID -- so all of a "+
		"query's graph facts collide on one dedup key); surviving graph results landed at merged-list "+
		"positions %d..%d (0-based)", maxSurvivorsPerQuery, gpMin, gpMax)
	t.Logf("DIG SCORES: graph-sourced candidates scored %.4f..%.4f; the WORST vector-sourced candidate "+
		"scored %.4f -- graph results sort below every document, so the graph-confidence term "+
		"(dig.go:110) cannot promote anything into the top-%d",
		minFloatOf(graphDigScores), maxOf(graphDigScores), vecDigMin, maxK)
	t.Logf("TOP-10 IMPACT: a graph-sourced result appeared in the top-%d on %d/%d queries",
		maxK, graphInTop10, len(hardQueries))
	t.Logf("HYBRID vs COSINE: identical top-%d episode-ID order on %d/%d queries (answerable + unanswerable)",
		maxK, hybridSameAsCosine, len(hardQueries))
	t.Logf("DIG WITH A LIVE GRAPH: hybrid+DIG returned the identical top-%d order as un-reranked hybrid "+
		"on %d/%d queries", maxK, digSameAsHybrid, len(hardQueries))

	avg := func(xs []float64) float64 {
		if len(xs) == 0 {
			return 0
		}
		var s float64
		for _, x := range xs {
			s += x
		}
		return s / float64(len(xs))
	}
	t.Logf("--- unanswerable queries (%d): top-1 score still returned ---", len(cosNoAnswer))
	t.Logf("  cosine: mean top-1 = %.4f, max = %.4f", avg(cosNoAnswer), maxOf(cosNoAnswer))
	t.Logf("  hybrid: mean top-1 = %.4f, max = %.4f", avg(hybNoAnswer), maxOf(hybNoAnswer))

	t.Logf("HONEST RESULT LABEL: recall@k / MRR for Memora's full hybrid retrieval path " +
		"(retrieval.Service.Retrieve -- concurrent Qdrant cosine + Neo4j 2-hop traversal, merged and " +
		"DIG-reranked) over a Neo4j graph populated by the production ExtractEntities heuristic run on " +
		"the corpus text. LLM triple extraction (llm.ExtractTriples, OpenAI-only) remains UNMEASURED, " +
		"as do consolidation and knapsack assembly.")
}

// dedupeStable removes repeats while preserving first-seen order.
func dedupeStable(xs []string) []string {
	seen := make(map[string]bool, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func minOf(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs {
		if x < m {
			m = x
		}
	}
	return m
}

func maxIntOf(xs []int) int {
	var m int
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

func minFloatOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs {
		if x < m {
			m = x
		}
	}
	return m
}
