package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/dig"
	"github.com/memora/cma/internal/ingest"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/segmentation"
	"github.com/memora/cma/internal/vectorstore"
)

const hardEvalUserID = "hard-eval-user"

// TestHardRetrievalEvalVsBM25 is the round-4 replacement for round 3's
// TestRetrievalEvalRecallAndMRR, whose 20 topically disjoint documents
// produced a meaningless 100% recall@k -- an easy discrimination task, not
// a measurement. hardCorpus/hardQueries (hard_corpus.go) were written
// entirely, ground truth included, before this test was ever run against
// them; nothing here was adjusted after seeing a result. If the numbers
// are bad, they are reported bad.
//
// Reports the full recall@1/3/5/10 + MRR curve for THREE retrieval methods
// head to head, over the identical corpus and queries:
//   - BM25: a from-scratch Okapi lexical baseline (bm25.go), no embedding,
//     no vector store, no network
//   - cosine: Qdrant top-k cosine search over local all-MiniLM-L6-v2
//     384-dim embeddings (via the real, fixed StructuralSegmenter +
//     ingest.Service pipeline)
//   - cosine+DIG: the same cosine candidates (pool of digPoolSize, wider
//     than the top-10 cutoff so reranking has room to actually reorder),
//     reranked by dig.Reranker.Rerank -- which is the HEURISTIC scorer
//     (cosine + recency + importance + decay-factor + graph-confidence),
//     since the true logprob-based DIG is permanently unreachable (see
//     dig.go's doc comment, round 3). Label this arm "cosine+heuristic
//     DIG", never "the logprob DIG the paper describes."
//
// Predicted BEFORE running (see round-5 report): in this eval, cosine+DIG
// should be statistically indistinguishable from plain cosine. Every term
// heuristicScore adds beyond the cosine baseline is either constant or
// zero here: all 100 episodes are ingested within the same few seconds so
// the recency term is ~identical across candidates; ImportanceScore is 0
// for every episode (surprisal is gone, see structural.go); DecayFactor is
// 1.0 for every episode (nothing has been consolidated); and GraphFacts is
// always empty (no Neo4j). Adding the same near-constant to every
// candidate's score cannot change their relative order. This is a
// prediction from reading the code, stated before the run below, not a
// post-hoc excuse for a null result.
//
// Scope, same constraint as rounds 3-4: vector-only. Neo4j is not stood
// up. Report/word ANY number from this test as "recall@k / MRR over
// Qdrant top-k cosine search" (plus, for the third arm, "with heuristic
// DIG reranking") -- never "Memora's retrieval quality," never implying
// the graph arm or knapsack assembly were measured. Neither path in this
// test touches that code.
func TestHardRetrievalEvalVsBM25(t *testing.T) {
	root := filepath.Join("..", "third_party")
	lib := filepath.Join(root, "onnxruntime", "lib", "libonnxruntime.so")
	model := filepath.Join(root, "models", "all-MiniLM-L6-v2", "model_quint8_avx2.onnx")
	vocab := filepath.Join(root, "models", "all-MiniLM-L6-v2", "vocab.txt")
	for _, p := range []string{lib, model, vocab} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("third_party asset missing (%s): %v", p, err)
		}
	}
	qdrantAddr := "localhost:6334"
	if !qdrantReachable(qdrantAddr) {
		t.Skipf("Qdrant not reachable at %s -- see README for how to start it", qdrantAddr)
	}

	ctx := context.Background()

	embedder, err := llm.NewLocalEmbedProvider(lib, model, vocab, 128, 384)
	if err != nil {
		t.Fatalf("NewLocalEmbedProvider: %v", err)
	}
	defer embedder.Close()

	collectionName := fmt.Sprintf("cma_hardeval_minilm384_%d", time.Now().UnixNano())
	qdrantCfg := configs.QdrantConfig{
		Host: "localhost", GRPCPort: 6334,
		Collection: collectionName, VectorSize: 384, HnswM: 16, HnswEF: 100,
	}
	vectorDB, err := vectorstore.NewQdrantStore(qdrantCfg)
	if err != nil {
		t.Fatalf("NewQdrantStore: %v", err)
	}
	defer vectorDB.Close()
	if err := vectorDB.EnsureCollection(ctx); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	segCfg := configs.SegmentationConfig{MinEpisodeTokens: 50, MaxEpisodeTokens: 500}
	segmenter := segmentation.NewStructuralSegmenter(embedder, segCfg)
	m := sharedMetrics()
	ingestSvc := ingest.NewService(segmenter, vectorDB, m)

	// --- Ingest: one document per call, so each document lands as exactly
	// one episode (all are well under the 500-token max) ---
	docEpisodeID := make([]string, len(hardCorpus))
	for i, doc := range hardCorpus {
		resp, err := ingestSvc.Ingest(ctx, hardEvalUserID, doc, "user")
		if err != nil {
			t.Fatalf("Ingest(doc %d): %v", i, err)
		}
		if resp.Segments != 1 {
			t.Fatalf("doc %d produced %d episodes, expected exactly 1 (adjust corpus or this assumption)",
				i, resp.Segments)
		}
		docEpisodeID[i] = resp.EpisodeIDs[0]
	}
	t.Logf("INGEST: %d documents -> %d episodes in Qdrant collection %q",
		len(hardCorpus), len(hardCorpus), collectionName)

	if err := waitForCount(ctx, vectorDB, hardEvalUserID, len(hardCorpus), 15*time.Second); err != nil {
		t.Fatalf("points never became visible after ingest: %v", err)
	}

	bm25 := NewBM25(hardCorpus)
	// nil llm.Provider: Rerank never calls it (round-3 finding, dig_test.go
	// TestRerankNeverCallsLLM) -- the logprob path is permanently
	// unreachable, so heuristicScore always runs instead.
	reranker := dig.NewReranker(nil, configs.DIGConfig{MinScore: -0.5})

	ks := []int{1, 3, 5, 10}
	maxK := ks[len(ks)-1]
	const digPoolSize = 30 // wider than maxK so reranking has room to reorder

	type curve struct {
		hitsAtK map[int]int
		rrSum   float64
		n       int
	}
	vecCurve := curve{hitsAtK: map[int]int{}}
	bmCurve := curve{hitsAtK: map[int]int{}}
	digCurve := curve{hitsAtK: map[int]int{}}
	var vecNoAnswerScores, bmNoAnswerScores []float64
	var misses []string
	identicalOrder := 0 // queries where cosine+DIG returned the same top-maxK IDs in the same order as raw cosine

	for qi, q := range hardQueries {
		// --- embedding path: fetch a wider pool once, reuse for both
		// raw-cosine (its own top maxK) and cosine+DIG (reranks the pool) ---
		qVec, err := embedder.Embed(ctx, q.text)
		if err != nil {
			t.Fatalf("embed query %d: %v", qi, err)
		}
		pool, err := vectorDB.Search(ctx, hardEvalUserID, qVec, digPoolSize)
		if err != nil {
			t.Fatalf("vector search query %d: %v", qi, err)
		}
		vecResults := pool
		if len(vecResults) > maxK {
			vecResults = vecResults[:maxK]
		}
		vecRank := rankOf(vecResults, q.expected, docEpisodeID)

		digCandidates, err := reranker.Rerank(ctx, q.text, pool)
		if err != nil {
			t.Fatalf("dig rerank query %d: %v", qi, err)
		}
		digResults := make([]models.RetrievalResult, 0, len(digCandidates))
		for _, c := range digCandidates {
			digResults = append(digResults, c.Result)
		}
		if len(digResults) > maxK {
			digResults = digResults[:maxK]
		}
		digRank := rankOf(digResults, q.expected, docEpisodeID)

		if sameOrder(vecResults, digResults) {
			identicalOrder++
		}

		// --- BM25 path ---
		bmScores := bm25.ScoreAll(q.text)
		bmRankedAll := rankedIndices(bmScores)
		bmRanked := bmRankedAll
		if len(bmRanked) > maxK {
			bmRanked = bmRanked[:maxK]
		}
		bmRank := 0
		for pos, docIdx := range bmRanked {
			if q.expected >= 0 && docIdx == q.expected {
				bmRank = pos + 1
				break
			}
		}

		if q.expected < 0 {
			// Unanswerable query: record top-1 confidence each method still
			// hands back, reported separately -- never folded into recall/MRR.
			if len(vecResults) > 0 {
				vecNoAnswerScores = append(vecNoAnswerScores, vecResults[0].Score)
			}
			if len(bmRankedAll) > 0 {
				bmNoAnswerScores = append(bmNoAnswerScores, bmScores[bmRankedAll[0]])
			}
			continue
		}

		vecCurve.n++
		bmCurve.n++
		digCurve.n++
		for _, k := range ks {
			if vecRank != 0 && vecRank <= k {
				vecCurve.hitsAtK[k]++
			}
			if bmRank != 0 && bmRank <= k {
				bmCurve.hitsAtK[k]++
			}
			if digRank != 0 && digRank <= k {
				digCurve.hitsAtK[k]++
			}
		}
		if vecRank != 0 {
			vecCurve.rrSum += 1.0 / float64(vecRank)
		}
		if bmRank != 0 {
			bmCurve.rrSum += 1.0 / float64(bmRank)
		}
		if digRank != 0 {
			digCurve.rrSum += 1.0 / float64(digRank)
		}
		if vecRank == 0 || vecRank > 3 {
			misses = append(misses, fmt.Sprintf("  vec rank=%d dig rank=%d bm25 rank=%d: %q (expected doc %d, cluster %d)",
				vecRank, digRank, bmRank, q.text, q.expected, hardClusterOf(q.expected)))
		}
	}

	report := func(name string, c curve) {
		t.Logf("--- %s: %d answerable queries ---", name, c.n)
		sort.Ints(ks)
		for _, k := range ks {
			t.Logf("  recall@%-2d = %d/%d = %.4f", k, c.hitsAtK[k], c.n, float64(c.hitsAtK[k])/float64(c.n))
		}
		t.Logf("  MRR (cutoff %d) = %.4f", maxK, c.rrSum/float64(c.n))
	}
	t.Logf("=== CORPUS: %d documents in %d clusters of 5, %d answerable queries, %d unanswerable ===",
		len(hardCorpus), len(hardCorpus)/5, vecCurve.n, len(vecNoAnswerScores))
	report("BM25 (Okapi, k1=1.5 b=0.75, lexical only)", bmCurve)
	report("COSINE (Qdrant top-k, all-MiniLM-L6-v2 384-dim)", vecCurve)
	report("COSINE+HEURISTIC DIG (cosine + recency + importance + decay + graph-confidence rerank)", digCurve)
	t.Logf("DIG PREDICTION CHECK: cosine+DIG returned the identical top-%d ID order as raw cosine on %d/%d "+
		"queries, answerable and unanswerable together (predicted: all of them, since every non-cosine "+
		"DIG term is constant or zero in this eval -- see this test's doc comment)",
		maxK, identicalOrder, len(hardQueries))

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
	t.Logf("--- unanswerable queries (%d): top-1 score still returned (not a hit/miss metric, just observed confidence) ---",
		len(vecNoAnswerScores))
	t.Logf("  embedding: mean top-1 cosine = %.4f, max = %.4f", avg(vecNoAnswerScores), maxOf(vecNoAnswerScores))
	t.Logf("  BM25:      mean top-1 score  = %.4f, max = %.4f (0 means no query term ever appeared in any document)",
		avg(bmNoAnswerScores), maxOf(bmNoAnswerScores))

	if len(misses) > 0 {
		t.Logf("--- %d answerable queries where the embedding path ranked the correct doc outside top 3 (or missed) ---", len(misses))
		for _, miss := range misses {
			t.Log(miss)
		}
	}

	t.Logf("HONEST RESULT LABEL: recall@k / MRR over Qdrant top-k cosine search, an Okapi BM25 "+
		"baseline, and cosine reranked by DIG's HEURISTIC scorer (not the logprob DIG the paper "+
		"describes -- that path is permanently unreachable), all-MiniLM-L6-v2 384-dim local "+
		"embeddings, %d clustered documents / %d answerable queries. Does NOT measure the Neo4j "+
		"graph arm or knapsack assembly.",
		len(hardCorpus), vecCurve.n)
}

// rankOf returns the 1-based position of the expected document's episode in
// results, or 0 if it is not present (or expected < 0, i.e. unanswerable).
func rankOf(results []models.RetrievalResult, expected int, docEpisodeID []string) int {
	if expected < 0 {
		return 0
	}
	for pos, r := range results {
		if r.Episode != nil && r.Episode.ID == docEpisodeID[expected] {
			return pos + 1
		}
	}
	return 0
}

// sameOrder reports whether two result lists name the same episode IDs in
// the same order -- used to check the round-5 prediction that cosine+DIG
// cannot reorder cosine's ranking in this eval.
func sameOrder(a, b []models.RetrievalResult) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		idA, idB := "", ""
		if a[i].Episode != nil {
			idA = a[i].Episode.ID
		}
		if b[i].Episode != nil {
			idB = b[i].Episode.ID
		}
		if idA != idB {
			return false
		}
	}
	return true
}

func maxOf(xs []float64) float64 {
	var m float64
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}
