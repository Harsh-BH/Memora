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
	"github.com/memora/cma/internal/ingest"
	"github.com/memora/cma/internal/llm"
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
// Reports the full recall@1/3/5/10 + MRR curve for TWO retrieval methods
// head to head, over the identical corpus and queries:
//   - the embedding path: Qdrant top-k cosine search over local
//     all-MiniLM-L6-v2 384-dim embeddings (via the real, fixed
//     StructuralSegmenter + ingest.Service pipeline)
//   - a from-scratch Okapi BM25 lexical baseline (bm25.go), no embedding,
//     no vector store, no network
//
// Scope, same constraint as round 3: vector-only. Neo4j is not stood up.
// Report/word ANY number from this test as "recall@k / MRR over Qdrant
// top-k cosine search" for the embedding path -- never "Memora's retrieval
// quality," never implying the graph arm, DIG reranking, or knapsack
// assembly were measured. Neither path in this test touches that code.
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

	ks := []int{1, 3, 5, 10}
	maxK := ks[len(ks)-1]

	type curve struct {
		hitsAtK map[int]int
		rrSum   float64
		n       int
	}
	vecCurve := curve{hitsAtK: map[int]int{}}
	bmCurve := curve{hitsAtK: map[int]int{}}
	var vecNoAnswerScores, bmNoAnswerScores []float64
	var misses []string

	for qi, q := range hardQueries {
		// --- embedding path ---
		qVec, err := embedder.Embed(ctx, q.text)
		if err != nil {
			t.Fatalf("embed query %d: %v", qi, err)
		}
		vecResults, err := vectorDB.Search(ctx, hardEvalUserID, qVec, maxK)
		if err != nil {
			t.Fatalf("vector search query %d: %v", qi, err)
		}
		vecRank := 0
		for pos, r := range vecResults {
			if r.Episode != nil && q.expected >= 0 && r.Episode.ID == docEpisodeID[q.expected] {
				vecRank = pos + 1
				break
			}
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
		for _, k := range ks {
			if vecRank != 0 && vecRank <= k {
				vecCurve.hitsAtK[k]++
			}
			if bmRank != 0 && bmRank <= k {
				bmCurve.hitsAtK[k]++
			}
		}
		if vecRank != 0 {
			vecCurve.rrSum += 1.0 / float64(vecRank)
		}
		if bmRank != 0 {
			bmCurve.rrSum += 1.0 / float64(bmRank)
		}
		if vecRank == 0 || vecRank > 3 {
			misses = append(misses, fmt.Sprintf("  vec rank=%d bm25 rank=%d: %q (expected doc %d, cluster %d)",
				vecRank, bmRank, q.text, q.expected, hardClusterOf(q.expected)))
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
	report("EMBEDDING (Qdrant top-k cosine, all-MiniLM-L6-v2 384-dim)", vecCurve)
	report("BM25 (Okapi, k1=1.5 b=0.75, lexical only)", bmCurve)

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

	t.Logf("HONEST RESULT LABEL: recall@k / MRR over Qdrant top-k cosine search vs. an Okapi BM25 "+
		"baseline, all-MiniLM-L6-v2 384-dim local embeddings, %d clustered documents / %d answerable "+
		"queries. Does NOT measure the Neo4j graph arm, DIG reranking, or knapsack assembly.",
		len(hardCorpus), vecCurve.n)
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
