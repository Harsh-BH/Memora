// Package eval is the retrieval-quality harness RAG_PROJECT_DECISION.md
// called for on 2026-08-20 and that nobody had built. It measures ONE
// thing: recall@k and MRR for Qdrant top-k cosine search over a local
// embedding model. It does NOT exercise the Neo4j graph half, DIG
// reranking, or knapsack assembly -- see the doc comment on
// TestRetrievalEvalRecallAndMRR for why, and report/word any number this
// produces as "recall@k over Qdrant top-k cosine", never as "Memora's
// retrieval quality" or anything implying the graph arm was measured.
package eval

import (
	"context"
	"fmt"
	"net"
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

const evalUserID = "eval-user"

// qdrantReachable does a cheap TCP dial rather than a full gRPC handshake,
// just to decide skip-vs-run.
func qdrantReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// TestRetrievalEvalRecallAndMRR is the whole point of this round: a small,
// honest, real measurement of retrieval quality, in place of the eval
// harness RAG_PROJECT_DECISION.md asked for and nobody built.
//
// Scope, deliberately narrow:
//   - Embeddings: local all-MiniLM-L6-v2 ONNX, 384-dim (see
//     internal/llm/local_embed.go). NOT the production 1536-dim model.
//   - Retrieval: vectorstore.VectorStore.Search directly (Qdrant top-k
//     cosine only). Neo4j is not stood up this round (RAM budget), so the
//     graph half of hybrid retrieval, DIG reranking, and knapsack context
//     assembly are UNMEASURED here -- this test calls none of that code.
//   - Ingest: goes through the real, fixed StructuralSegmenter and
//     ingest.Service, so it also doubles as the first genuine end-to-end
//     exercise of the ingest bugfix (see the corpus/episode counts logged
//     below -- multi-token real content, not the old 1-token garbage).
//
// Skips (does not fail) if the third_party model assets or a local Qdrant
// aren't available, so a normal `go test ./...` stays green without infra.
func TestRetrievalEvalRecallAndMRR(t *testing.T) {
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

	// A collection distinct from production's 384-vs-1536-dim "cma_episodes"
	// -- named and timestamped so repeated runs don't collide or accumulate.
	collectionName := fmt.Sprintf("cma_eval_minilm384_%d", time.Now().UnixNano())
	qdrantCfg := configs.QdrantConfig{
		Host:       "localhost",
		GRPCPort:   6334,
		Collection: collectionName,
		VectorSize: 384,
		HnswM:      16,
		HnswEF:     100,
	}
	vectorDB, err := vectorstore.NewQdrantStore(qdrantCfg)
	if err != nil {
		t.Fatalf("NewQdrantStore: %v", err)
	}
	defer vectorDB.Close()
	if err := vectorDB.EnsureCollection(ctx); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() { dropCollection(t, collectionName) })

	segCfg := configs.SegmentationConfig{MinEpisodeTokens: 50, MaxEpisodeTokens: 500}
	segmenter := segmentation.NewStructuralSegmenter(embedder, segCfg)
	m := sharedMetrics()
	ingestSvc := ingest.NewService(segmenter, vectorDB, m)

	// --- Ingest the corpus through the real, fixed pipeline ---
	docEpisodeIDs := make([][]string, len(corpus))
	totalEpisodes := 0
	// Ingest wall clock is logged because PREREGISTRATION.md 4.15 makes it the
	// rollback trigger for Upsert's Wait:true flag (>2x regression => Wait
	// becomes a constructor option). Keeping the number in the test output means
	// that rule stays checkable instead of living in one commit message.
	ingestStart := time.Now()
	for i, doc := range corpus {
		resp, err := ingestSvc.Ingest(ctx, evalUserID, doc, "user")
		if err != nil {
			t.Fatalf("Ingest(doc %d): %v", i, err)
		}
		if resp.Segments == 0 {
			t.Fatalf("doc %d produced 0 episodes -- this is exactly the old ingest bug", i)
		}
		docEpisodeIDs[i] = resp.EpisodeIDs
		totalEpisodes += resp.Segments
	}
	ingestWall := time.Since(ingestStart)
	t.Logf("INGEST: %d source documents -> %d episodes in Qdrant collection %q, wall clock %s",
		len(corpus), totalEpisodes, collectionName, ingestWall.Round(time.Millisecond))

	// Upsert's gRPC call does not set wait:true (internal/vectorstore/qdrant.go),
	// so a freshly-upserted point is not guaranteed immediately visible to a
	// filtered Scroll/Search on brand-new collection -- observed directly
	// while writing this test: GetRecent returned 0 rows called immediately
	// after ingest, then returned all 20 correctly seconds later in a
	// separate process. Poll rather than assume synchronous visibility.
	if err := waitForCount(ctx, vectorDB, evalUserID, totalEpisodes, 10*time.Second); err != nil {
		t.Fatalf("points never became visible after ingest: %v", err)
	}

	// Evidence the ingest fix produced real content, not 1-token garbage:
	// pull one episode back and log its length.
	sample, err := vectorDB.GetRecent(ctx, evalUserID, 1)
	if err != nil {
		t.Fatalf("GetRecent sample: %v", err)
	}
	if len(sample) != 1 {
		t.Fatalf("expected 1 sample episode, got %d", len(sample))
	}
	t.Logf("SAMPLE PAYLOAD: episode %s, %d chars, %d tokens, content=%q",
		sample[0].ID, len(sample[0].Content), sample[0].TokenCount, truncate(sample[0].Content, 120))
	if sample[0].TokenCount <= 1 {
		t.Fatalf("sample episode has TokenCount=%d -- looks like the old 1-token bug, not the fix",
			sample[0].TokenCount)
	}

	// expectedIDs[i] = the set of episode IDs that count as a correct hit
	// for queries[i].expected.
	expectedIDs := make([]map[string]bool, len(queries))
	for i, q := range queries {
		set := make(map[string]bool)
		for _, id := range docEpisodeIDs[q.expected] {
			set[id] = true
		}
		expectedIDs[i] = set
	}

	// --- Run every query, at k = 1, 3, 5, 10 ---
	ks := []int{1, 3, 5, 10}
	maxK := ks[len(ks)-1]
	hitsAtK := make(map[int]int)
	var reciprocalRanks []float64
	var misses []string

	for qi, q := range queries {
		qVec, err := embedder.Embed(ctx, q.text)
		if err != nil {
			t.Fatalf("embed query %d: %v", qi, err)
		}
		results, err := vectorDB.Search(ctx, evalUserID, qVec, maxK)
		if err != nil {
			t.Fatalf("search query %d: %v", qi, err)
		}

		rank := 0 // 0 = not found in top maxK
		for pos, r := range results {
			if r.Episode != nil && expectedIDs[qi][r.Episode.ID] {
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
			reciprocalRanks = append(reciprocalRanks, 1.0/float64(rank))
		} else {
			reciprocalRanks = append(reciprocalRanks, 0)
			misses = append(misses, fmt.Sprintf("  MISS: %q (expected doc %d)", q.text, q.expected))
		}
	}

	n := len(queries)
	t.Logf("--- RECALL@K AND MRR: %d queries over %d episodes from %d documents ---", n, totalEpisodes, len(corpus))
	sort.Ints(ks)
	for _, k := range ks {
		recall := float64(hitsAtK[k]) / float64(n)
		t.Logf("recall@%-2d = %d/%d = %.4f", k, hitsAtK[k], n, recall)
	}
	var mrrSum float64
	for _, rr := range reciprocalRanks {
		mrrSum += rr
	}
	mrr := mrrSum / float64(n)
	t.Logf("MRR (cutoff %d) = %.4f", maxK, mrr)
	if len(misses) > 0 {
		t.Logf("--- %d misses (rank > %d or not found) ---", len(misses), maxK)
		for _, miss := range misses {
			t.Log(miss)
		}
	}

	t.Logf("HONEST RESULT LABEL: recall@k / MRR over Qdrant top-k cosine search, "+
		"all-MiniLM-L6-v2 384-dim local embeddings, %d docs / %d episodes / %d queries. "+
		"Does NOT measure the Neo4j graph arm, DIG reranking, or knapsack assembly -- "+
		"none of that code ran in this test.",
		len(corpus), totalEpisodes, n)
}

// waitForCount polls GetRecent until at least want episodes are visible for
// userID, or timeout elapses. See its call site for why this is needed.
func waitForCount(ctx context.Context, vectorDB vectorstore.VectorStore, userID string, want int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		eps, err := vectorDB.GetRecent(ctx, userID, want+5)
		if err != nil {
			return err
		}
		if len(eps) >= want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s: got %d of %d expected episodes", timeout, len(eps), want)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
