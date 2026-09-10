package eval

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/graphstore"
	"github.com/memora/cma/internal/ingest"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/retrieval"
	"github.com/memora/cma/internal/segmentation"
	"github.com/memora/cma/internal/vectorstore"
)

// TestGraphAttachFeasibilityProbe measures, WITHOUT changing any production
// code, what the proposed "corroboration" merge could do if it were built:
// attach each graph fact to the vector result whose episode ID equals the
// fact's source_ep_id, then let a saturating boost reorder the top-k.
//
// It does NOT reimplement retrieval. It uses the production ingest path, the
// production entity heuristic (retrieval.Service.ExtractEntities) to build the
// graph, the production Qdrant Search, and the EXACT Cypher from
// graphstore.TraverseHops -- with the one line that fix would add:
// record.Get("source_ep_id"). Everything else is arithmetic over those rows.
//
// Purpose: turn "the fix will help" from PROJECTED into MEASURED-or-not.
// Delete this file once the real fix lands and hybrid_eval_test.go covers it.
//
// MEASUREMENT SCAFFOLDING, committed 2026-09-01 so that deletion leaves a
// record. This file was untracked while it was producing the numbers the graph
// arm design rests on; deleting an untracked file destroys its own evidence.
// PREREGISTRATION.md 5.3 therefore requires it be committed BEFORE the Track B
// diff removes it. It is not part of the forgetting experiment and gates
// nothing in it.
func TestGraphAttachFeasibilityProbe(t *testing.T) {
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
		t.Skip("Neo4j not reachable at localhost:7687")
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx := context.Background()
	embedder, err := llm.NewLocalEmbedProvider(lib, model, vocab, 128, 384)
	if err != nil {
		t.Fatalf("NewLocalEmbedProvider: %v", err)
	}
	defer embedder.Close()

	stamp := time.Now().UnixNano()
	userID := fmt.Sprintf("attach-probe-user-%d", stamp)
	collectionName := fmt.Sprintf("cma_attachprobe_minilm384_%d", stamp)

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

	driver, err := neo4j.NewDriverWithContext("bolt://localhost:7687", neo4j.BasicAuth("neo4j", "cmapassword", ""))
	if err != nil {
		t.Fatalf("neo4j driver: %v", err)
	}
	defer driver.Close(ctx)

	m := sharedMetrics()
	segCfg := configs.SegmentationConfig{MinEpisodeTokens: 50, MaxEpisodeTokens: 500}
	ingestSvc := ingest.NewService(segmentation.NewStructuralSegmenter(embedder, segCfg), vectorDB, m)
	retCfg := configs.RetrievalConfig{VectorTopK: 20, GraphMaxHops: 2, Timeout: 10 * time.Second}
	svc := retrieval.NewService(vectorDB, graphDB, embedder, retCfg, m)

	docEpisodeID := make([]string, len(hardCorpus))
	epToDoc := map[string]int{}
	for i, doc := range hardCorpus {
		resp, err := ingestSvc.Ingest(ctx, userID, doc, "user")
		if err != nil {
			t.Fatalf("Ingest(doc %d): %v", i, err)
		}
		docEpisodeID[i] = resp.EpisodeIDs[0]
		epToDoc[resp.EpisodeIDs[0]] = i
	}
	if err := waitForCount(ctx, vectorDB, userID, len(hardCorpus), 15*time.Second); err != nil {
		t.Fatalf("points never became visible: %v", err)
	}
	for i, doc := range hardCorpus {
		ents := dedupeStable(svc.ExtractEntities(doc))
		for j := 0; j+1 < len(ents); j++ {
			tr := models.Triple{Subject: ents[j], Predicate: "co_occurs_in_document", Object: ents[j+1], Confidence: 1.0}
			if err := graphDB.InsertTriple(ctx, userID, tr, docEpisodeID[i]); err != nil {
				t.Fatalf("InsertTriple: %v", err)
			}
		}
	}

	// The EXACT TraverseHops Cypher (graphstore/neo4j.go:161-173) plus the
	// source_ep_id read the fix would add.
	cypher := fmt.Sprintf(`
		MATCH path = (s:Entity {user_id: $user_id})-[r:RELATES_TO*1..%d]-(target:Entity)
		WHERE s.name IN $seeds
		  AND ALL(rel IN relationships(path) WHERE rel.valid_to IS NULL OR rel.valid_to > datetime())
		UNWIND relationships(path) AS rel
		WITH DISTINCT rel, startNode(rel) AS src, endNode(rel) AS dst
		RETURN rel.id AS id, src.name AS from_name, dst.name AS to_name,
		       rel.predicate AS predicate, rel.confidence AS confidence,
		       rel.source_ep_id AS source_ep_id
		ORDER BY rel.confidence DESC
		LIMIT 50
	`, retCfg.GraphMaxHops)

	// --- collect per-query evidence once, then score it under several policies ---
	type qdata struct {
		eps      []string  // vector top-20 episode IDs, cosine order
		cos      []float64 // their cosine scores
		gold     string
		factsAll map[string]int // epID -> distinct attached facts, all seeds
		factsFlt map[string]int // epID -> distinct attached facts, stopword-filtered seeds
	}
	// Sentence-initial function words that ExtractEntities emits as "entities"
	// because they are capitalized. "The" is the highest-degree node in this
	// graph (degree 12, reaching 16 distinct source episodes at 2 hops).
	stop := map[string]bool{"What": true, "Why": true, "How": true, "Which": true, "When": true,
		"Where": true, "Who": true, "The": true, "A": true, "An": true, "In": true, "Is": true,
		"Does": true, "Do": true, "Can": true, "If": true, "This": true, "That": true, "It": true}

	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j", AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	traverse := func(seeds []string, allowed map[string]int) (map[string]int, int, int) {
		facts := map[string]map[string]bool{}
		raw, orphan := 0, 0
		if len(seeds) == 0 {
			return map[string]int{}, 0, 0
		}
		res, err := session.Run(ctx, cypher, map[string]any{"user_id": userID, "seeds": seeds})
		if err != nil {
			t.Fatalf("traverse: %v", err)
		}
		for res.Next(ctx) {
			rec := res.Record()
			src, _ := rec.Get("source_ep_id")
			from, _ := rec.Get("from_name")
			pred, _ := rec.Get("predicate")
			to, _ := rec.Get("to_name")
			ep := fmt.Sprintf("%v", src)
			raw++
			if _, ok := allowed[ep]; !ok {
				orphan++
				continue
			}
			if facts[ep] == nil {
				facts[ep] = map[string]bool{}
			}
			facts[ep][fmt.Sprintf("%v|%v|%v", from, pred, to)] = true
		}
		if err := res.Err(); err != nil {
			t.Fatalf("traverse iterate: %v", err)
		}
		out := make(map[string]int, len(facts))
		for ep, set := range facts {
			out[ep] = len(set)
		}
		return out, raw, orphan
	}

	data := make([]qdata, 0, len(hardQueries))
	var rawFacts, orphanFacts, attachedAll, goldCorr, nonGoldCorr, queriesWithFacts int
	var rawFlt, orphanFlt, attachedFlt, goldCorrFlt, nonGoldCorrFlt int
	var gapTop12 []float64

	for qi, q := range hardQueries {
		qVec, err := embedder.Embed(ctx, q.text)
		if err != nil {
			t.Fatalf("embed q%d: %v", qi, err)
		}
		pool, err := vectorDB.Search(ctx, userID, qVec, retCfg.VectorTopK)
		if err != nil {
			t.Fatalf("search q%d: %v", qi, err)
		}
		d := qdata{}
		allowed := map[string]int{}
		for i, r := range pool {
			ep := ""
			if r.Episode != nil {
				ep = r.Episode.ID
			}
			d.eps = append(d.eps, ep)
			d.cos = append(d.cos, r.Score)
			allowed[ep] = i
		}
		if len(d.cos) >= 2 {
			gapTop12 = append(gapTop12, d.cos[0]-d.cos[1])
		}
		if q.expected >= 0 {
			d.gold = docEpisodeID[q.expected]
		}

		seeds := svc.ExtractEntities(q.text)
		var filtered []string
		for _, s := range seeds {
			if !stop[s] {
				filtered = append(filtered, s)
			}
		}
		var raw, orph int
		d.factsAll, raw, orph = traverse(seeds, allowed)
		rawFacts += raw
		orphanFacts += orph
		if raw > 0 {
			queriesWithFacts++
		}
		d.factsFlt, raw, orph = traverse(filtered, allowed)
		rawFlt += raw
		orphanFlt += orph

		for ep, n := range d.factsAll {
			attachedAll += n
			if ep == d.gold {
				goldCorr++
			} else {
				nonGoldCorr++
			}
		}
		for ep, n := range d.factsFlt {
			attachedFlt += n
			if ep == d.gold {
				goldCorrFlt++
			} else {
				nonGoldCorrFlt++
			}
		}
		data = append(data, d)
	}

	// baseline (no boost) recall@1 / MRR straight off the cosine order
	score := func(w float64, filtered bool) (int, float64, int) {
		var hits1, changed int
		var mrr float64
		var answerable int
		for _, d := range data {
			type sc struct {
				ep  string
				s   float64
				pos int
			}
			facts := d.factsAll
			if filtered {
				facts = d.factsFlt
			}
			list := make([]sc, 0, len(d.eps))
			for i, ep := range d.eps {
				s := d.cos[i]
				if n := facts[ep]; n > 0 {
					conf := float64(n) // every edge has confidence 1.0 in this graph
					s += w * conf / (conf + 1.0)
				}
				list = append(list, sc{ep, s, i})
			}
			sort.SliceStable(list, func(a, b int) bool { return list[a].s > list[b].s })
			for i := 0; i < len(list) && i < 10; i++ {
				if list[i].pos != i {
					changed++
					break
				}
			}
			if d.gold == "" {
				continue
			}
			answerable++
			for i, x := range list {
				if x.ep == d.gold {
					if i == 0 {
						hits1++
					}
					mrr += 1 / float64(i+1)
					break
				}
			}
		}
		return hits1, mrr / float64(answerable), changed
	}

	sort.Float64s(gapTop12)
	t.Logf("FACT SUPPLY: %d raw facts over %d/%d queries (same Cypher as TraverseHops, LIMIT 50)",
		rawFacts, queriesWithFacts, len(hardQueries))
	t.Logf("ATTACH RATE (all seeds): %d/%d facts had source_ep_id INSIDE the vector top-%d; %d orphans dropped",
		rawFacts-orphanFacts, rawFacts, retCfg.VectorTopK, orphanFacts)
	t.Logf("CORROBORATION SPREAD (all seeds): %d distinct attached facts; %d gold-doc corroborations vs %d non-gold",
		attachedAll, goldCorr, nonGoldCorr)
	t.Logf("ATTACH RATE (stopword-filtered seeds): %d/%d attachable, %d orphans; %d distinct attached facts; "+
		"%d gold vs %d non-gold corroborations",
		rawFlt-orphanFlt, rawFlt, orphanFlt, attachedFlt, goldCorrFlt, nonGoldCorrFlt)
	if len(gapTop12) > 0 {
		t.Logf("COSINE GAP top1-top2: min=%.4f p50=%.4f max=%.4f", gapTop12[0], gapTop12[len(gapTop12)/2], gapTop12[len(gapTop12)-1])
	}
	b1, bm, _ := score(0, false)
	t.Logf("BASELINE (w=0, cosine order): recall@1=%d/100 MRR=%.4f", b1, bm)
	for _, w := range []float64{0.15, 0.05, 0.02, 0.005} {
		for _, f := range []bool{false, true} {
			h, mrr, ch := score(w, f)
			label := "all seeds"
			if f {
				label = "stopword-filtered"
			}
			t.Logf("BOOST w=%.3f (%-17s): recall@1=%d/100 MRR=%.4f top-10 order changed on %d/%d queries",
				w, label, h, mrr, ch, len(hardQueries))
		}
	}
}
