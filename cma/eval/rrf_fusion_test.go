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

// TestHardRetrievalEvalRRFFusion answers a question TestHardRetrievalEvalVsBM25
// leaves open: BM25 and dense cosine are measured separately on the same
// hard_corpus.go / hardQueries harness (round 4, sha 9c461d0/7e63a9c) but
// never combined. This test fuses their FULL rankings over all 100 corpus
// documents with standard Reciprocal Rank Fusion (Cormack, Clarke & Buettcher
// 2009), RRF(d) = sum over each ranker of 1/(k + rank), k=60, no learned
// weights, no per-field tuning -- and reports the same recall@1/3/5/10 + MRR
// curve so it is directly comparable to the other two arms.
//
// This is exploratory, not a regression test: dense cosine already scores
// 98/100 recall@1 and 100/100 at @3-10 on this corpus (near the ceiling a
// 100-document corpus can show), so there is very little headroom for fusion
// to add anything, and BM25's 8 recall@1 misses are exactly the case where a
// generic rank-based combination could pull the correct doc down instead of
// up if BM25 confidently mis-ranks it. Reported honestly either way.
func TestHardRetrievalEvalRRFFusion(t *testing.T) {
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

	collectionName := fmt.Sprintf("cma_rrfeval_minilm384_%d", time.Now().UnixNano())
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

	docEpisodeID := make([]string, len(hardCorpus))
	for i, doc := range hardCorpus {
		resp, err := ingestSvc.Ingest(ctx, hardEvalUserID, doc, "user")
		if err != nil {
			t.Fatalf("Ingest(doc %d): %v", i, err)
		}
		if resp.Segments != 1 {
			t.Fatalf("doc %d produced %d episodes, expected exactly 1", i, resp.Segments)
		}
		docEpisodeID[i] = resp.EpisodeIDs[0]
	}
	if err := waitForCount(ctx, vectorDB, hardEvalUserID, len(hardCorpus), 15*time.Second); err != nil {
		t.Fatalf("points never became visible after ingest: %v", err)
	}

	epToIdx := make(map[string]int, len(docEpisodeID))
	for i, id := range docEpisodeID {
		epToIdx[id] = i
	}

	bm25 := NewBM25(hardCorpus)
	const rrfK = 60.0 // standard RRF constant (Cormack et al. 2009)
	n := len(hardCorpus)

	ks := []int{1, 3, 5, 10}
	maxK := ks[len(ks)-1]
	hitsAtK := map[int]int{}
	var rrSum float64
	var answered int

	for _, q := range hardQueries {
		qVec, err := embedder.Embed(ctx, q.text)
		if err != nil {
			t.Fatalf("embed query %q: %v", q.text, err)
		}
		// Full ranking from each arm (n = whole corpus), so RRF sees every
		// document's rank on both sides rather than a truncated pool.
		vecPool, err := vectorDB.Search(ctx, hardEvalUserID, qVec, n)
		if err != nil {
			t.Fatalf("vector search %q: %v", q.text, err)
		}
		vecRankOfDoc := make(map[int]int, n) // 1-based
		for pos, r := range vecPool {
			if r.Episode == nil {
				continue
			}
			if idx, ok := epToIdx[r.Episode.ID]; ok {
				vecRankOfDoc[idx] = pos + 1
			}
		}

		bmScores := bm25.ScoreAll(q.text)
		bmRanked := rankedIndices(bmScores)
		bmRankOfDoc := make(map[int]int, n)
		for pos, idx := range bmRanked {
			bmRankOfDoc[idx] = pos + 1
		}

		rrf := make([]float64, n)
		for docIdx := 0; docIdx < n; docIdx++ {
			if r, ok := vecRankOfDoc[docIdx]; ok {
				rrf[docIdx] += 1.0 / (rrfK + float64(r))
			}
			if r, ok := bmRankOfDoc[docIdx]; ok {
				rrf[docIdx] += 1.0 / (rrfK + float64(r))
			}
		}
		fusedOrder := make([]int, n)
		for i := range fusedOrder {
			fusedOrder[i] = i
		}
		sort.SliceStable(fusedOrder, func(i, j int) bool {
			return rrf[fusedOrder[i]] > rrf[fusedOrder[j]]
		})

		if q.expected < 0 {
			continue // unanswerable queries: not part of recall/MRR, matches hard_eval_test.go
		}
		answered++
		rank := 0
		for pos, docIdx := range fusedOrder {
			if docIdx == q.expected {
				rank = pos + 1
				break
			}
		}
		for _, k := range ks {
			if rank != 0 && rank <= k {
				hitsAtK[k]++
			}
		}
		if rank != 0 {
			rrSum += 1.0 / float64(rank)
		}
	}

	t.Logf("=== RRF FUSION (BM25 rank ⊕ cosine rank, k=%.0f) over %d answerable queries ===", rrfK, answered)
	sort.Ints(ks)
	for _, k := range ks {
		t.Logf("  recall@%-2d = %d/%d = %.4f", k, hitsAtK[k], answered, float64(hitsAtK[k])/float64(answered))
	}
	t.Logf("  MRR (cutoff %d) = %.4f", maxK, rrSum/float64(answered))
	t.Logf("COMPARE (same corpus/queries, TestHardRetrievalEvalVsBM25): BM25 recall@1=92/100 MRR=0.9517 | " +
		"cosine recall@1=98/100 MRR=0.9900")
}
