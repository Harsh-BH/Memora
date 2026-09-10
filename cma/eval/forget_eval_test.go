package eval

// The forgetting experiment's harness.
//
// Reads cma/eval/PREREGISTRATION.md as its specification. Every threshold,
// grid, seed, order and gate below carries the section number it comes from,
// and none of them is a knob: they were published before any arm ran.
//
// TWO ENTRY POINTS, deliberately:
//
//   - TestForgettingHarnessSmoke runs the whole machinery over a handful of
//     instances on every `go test ./...`. It asserts the harness self-checks
//     (ORACLE = 1.0, ANTI-ORACLE = 0.0, FLOOR = 1.0, PF0-PF4) which hold at any
//     n, so the harness cannot rot silently. It reports NO arm result and its
//     numbers must never be quoted.
//   - TestForgettingExperiment is the real run over all 477 instances, gated
//     behind MEMORA_RUN_FORGET_EXPERIMENT=1 so that a routine `go test ./...`
//     never produces experiment numbers as a side effect. Producing the
//     confirmatory number must be a deliberate act.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/dig"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/vectorstore"
)

// --- registered constants (PREREGISTRATION.md 2, 4.6, 4.8, 4.9) ---

const (
	retrievalDepth = 200 // D: one retrieval pass per query, shared by every arm
	armCutoff      = 50  // K: R_X = [r in R : r not in A_X][:K]
	embedMaxSeqLen = 256 // 4.9
	embedDim       = 384 // 4.9; configs/config.yaml's 1536 is NOT read here
	randomReplicas = 20  // 2.1
	randomBaseSeed = 20260901
	rhoStar        = 0.0439 // 4.6: 72 stale gold / 1640 turns, the ORACLE arm's own rate
	dbscanEpsilon  = 0.3    // 4.6, the SHIPPED configs/config.yaml:52
	dbscanMinPts   = 3      // 4.6, the SHIPPED configs/config.yaml:53
)

var (
	gridD1    = []float64{0.20, 0.30, 0.40, 0.50} // 4.6, a DISTANCE grid
	gridD2    = []float64{0.10, 0.15, 0.20, 0.25} // 4.6, a SIMILARITY grid
	gridD3w   = []float64{0.05, 0.1, 0.3}         // 2.3
	gridD3tau = []float64{30, 90, 365}            // 2.3, days
)

// instRun is one instance's frozen state: its turns in 4.4 order, its gold map,
// and the single ranked list every arm filters.
type instRun struct {
	inst    *LMEInstance
	turns   []Turn
	gold    Gold            // supersession labels; empty for control instances
	goldAll map[string]bool // any gold turn, which is what the control's recall@5 needs
	ranked  []Ranked        // R, one retrieval pass at depth D
	ageDays map[string]float64
}

// --- the two entry points ---

func TestForgettingHarnessSmoke(t *testing.T) {
	h := setupHarness(t, "smoke")
	if h == nil {
		return
	}
	h.ku = firstN(h.ku, 6)
	h.control = firstN(h.control, 6)
	h.full = false
	h.run(t)
}

func TestForgettingExperiment(t *testing.T) {
	if os.Getenv("MEMORA_RUN_FORGET_EXPERIMENT") != "1" {
		t.Skip("set MEMORA_RUN_FORGET_EXPERIMENT=1 to run the full forgetting experiment. " +
			"It is gated so that a routine `go test ./...` cannot produce the confirmatory " +
			"number as a side effect -- see cma/eval/PREREGISTRATION.md 1.2.")
	}
	h := setupHarness(t, "full")
	if h == nil {
		return
	}
	h.full = true
	h.run(t)
}

// --- harness construction ---

type harness struct {
	ctx              context.Context
	embedder         *llm.LocalEmbedProvider
	store            *vectorstore.QdrantStore
	collection       string
	cache            *EmbedCache
	cachePath        string
	all              []*LMEInstance
	ku               []*LMEInstance
	control          []*LMEInstance
	abstain          []*LMEInstance
	full             bool
	exploratoryTests int // 4.14: the count is printed so a reader can correct for it
}

func setupHarness(t *testing.T, label string) *harness {
	t.Helper()
	root := filepath.Join("..", "third_party")
	lib := filepath.Join(root, "onnxruntime", "lib", "libonnxruntime.so")
	model := filepath.Join(root, "models", "all-MiniLM-L6-v2", "model_quint8_avx2.onnx")
	vocab := filepath.Join(root, "models", "all-MiniLM-L6-v2", "vocab.txt")
	for _, p := range []string{lib, model, vocab} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("third_party asset missing (%s): %v", p, err)
		}
	}
	if _, err := os.Stat(LongMemEvalPath()); err != nil {
		t.Skipf("LongMemEval not present at %s (see cma/eval/testdata/DATASETS.md): %v", LongMemEvalPath(), err)
	}
	if !qdrantReachable("localhost:6334") {
		t.Skipf("Qdrant not reachable at localhost:6334 -- see README for how to start it")
	}

	// V8: sha256 verification is inside LoadLongMemEval and fails the run.
	all, err := LoadLongMemEval(LongMemEvalPath())
	if err != nil {
		t.Fatalf("VOID (V8): %v", err)
	}
	h := &harness{
		ctx:     context.Background(),
		all:     all,
		ku:      Select(all, IsKUPermissive),
		control: Select(all, IsControlPrimary),
		abstain: Select(all, func(in *LMEInstance) bool { return IsControl(in) && in.IsAbstention() }),
	}
	// 4.2's registered set sizes. TestRegisteredCorpusCounts checks every
	// structural fact; these three are re-asserted here because an arm number
	// computed over the wrong set is worse than no number.
	if len(h.ku) != 70 || len(h.control) != 398 || len(h.abstain) != 9 {
		t.Fatalf("query sets = KU %d / falsifier %d / _abs %d; registered 70 / 398 / 9",
			len(h.ku), len(h.control), len(h.abstain))
	}

	h.embedder, err = llm.NewLocalEmbedProvider(lib, model, vocab, embedMaxSeqLen, embedDim)
	if err != nil {
		t.Fatalf("NewLocalEmbedProvider: %v", err)
	}
	t.Cleanup(func() { h.embedder.Close() })

	modelSHA, err := FileSHA256(model)
	if err != nil {
		t.Fatalf("hash model: %v", err)
	}
	h.cachePath = filepath.Join(DatasetDir(), fmt.Sprintf("embed_cache_minilm%d_%d.gob", embedDim, embedMaxSeqLen))
	h.cache = LoadEmbedCache(h.cachePath, modelSHA, embedMaxSeqLen, embedDim)

	// 4.9: collection cma_eval_forget at dim 384, cosine. The smoke run uses a
	// suffixed name so it can never collide with a real run's collection, and
	// BOTH are dropped in t.Cleanup -- Qdrant's soft nofile limit of 1024 jams
	// the container after roughly three leaked collections.
	h.collection = "cma_eval_forget"
	if label != "full" {
		h.collection = fmt.Sprintf("cma_eval_forget_%s_%d", label, time.Now().UnixNano())
	}
	h.store, err = vectorstore.NewQdrantStore(configs.QdrantConfig{
		Host: "localhost", GRPCPort: 6334, Collection: h.collection,
		VectorSize: embedDim, HnswM: 16, HnswEF: 100,
		// Required: without it an archive-then-search measures Qdrant indexing
		// lag rather than forgetting (4.15).
		WaitForIndex: true,
	})
	if err != nil {
		t.Fatalf("NewQdrantStore: %v", err)
	}
	t.Cleanup(func() { h.store.Close() })
	// Drop first, in case a previous run died before its own cleanup: the
	// production cma_episodes collection is never named here and is untouched.
	dropCollection(t, h.collection)
	if err := h.store.EnsureCollection(h.ctx); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() { dropCollection(t, h.collection) })
	return h
}

func firstN[T any](xs []T, n int) []T {
	if len(xs) < n {
		return xs
	}
	return xs[:n]
}

// --- the run ---

func (h *harness) run(t *testing.T) {
	t.Logf("=== FORGETTING EXPERIMENT (%s) === collection %q dim %d, depth D=%d, cutoff K=%d",
		map[bool]string{true: "FULL RUN", false: "SMOKE -- machinery check only, NO result may be quoted"}[h.full],
		h.collection, embedDim, retrievalDepth, armCutoff)

	kuRuns := h.ingestAndRetrieve(t, h.ku, true)
	ctlRuns := h.ingestAndRetrieve(t, h.control, false)

	h.preflight(t, kuRuns)

	// --- theta* selection on the CONTROL set (4.6) ---
	// The control instances carry no KU item, no supersession label and no
	// outcome metric, so this selection cannot see SR@1.
	thetaD2 := h.selectTheta(t, ctlRuns, "D2", gridD2, func(r *instRun, th float64) map[string]bool {
		return ArchiveSet(NearestOlderIDF(r.turns, th))
	})
	thetaD1 := h.selectTheta(t, ctlRuns, "D1", gridD1, func(r *instRun, th float64) map[string]bool {
		return ArchiveSet(NearestOlderDense(r.turns, h.vecsOf(r), th))
	})

	// --- archive sets per arm (2) ---
	d2Pairs := make(map[string][]Pair, len(kuRuns))
	d1Pairs := make(map[string][]Pair, len(kuRuns))
	for _, r := range kuRuns {
		d2Pairs[r.inst.QuestionID] = NearestOlderIDF(r.turns, thetaD2)
		d1Pairs[r.inst.QuestionID] = NearestOlderDense(r.turns, h.vecsOf(r), thetaD1)
	}
	d2Sets := perInstance(kuRuns, func(r *instRun) map[string]bool {
		return ArchiveSet(d2Pairs[r.inst.QuestionID])
	})
	arms := []armSpec{
		{name: "RAW", role: "baseline", sets: perInstance(kuRuns, func(*instRun) map[string]bool { return nil })},
		{name: fmt.Sprintf("D2 (theta*=%.2f)", thetaD2), role: "CONFIRMATORY", sets: d2Sets, pairs: d2Pairs, primary: true},
		{name: fmt.Sprintf("D1 (theta=%.2f)", thetaD1), role: "EXPLORATORY", pairs: d1Pairs,
			sets: perInstance(kuRuns, func(r *instRun) map[string]bool {
				return ArchiveSet(d1Pairs[r.inst.QuestionID])
			})},
		{name: fmt.Sprintf("D0 (shipped eps=%.1f minPts=%d)", dbscanEpsilon, dbscanMinPts), role: "EXPLORATORY",
			sets: perInstance(kuRuns, func(r *instRun) map[string]bool {
				return ShippedConsolidatorArchive(r.turns, h.vecsOf(r), dbscanEpsilon, dbscanMinPts)
			})},
		{name: "ORACLE", role: "SELF-CHECK V2", sets: perInstance(kuRuns, func(r *instRun) map[string]bool {
			return OracleArchive(r.gold)
		})},
		{name: "ANTI-ORACLE", role: "SELF-CHECK V1", sets: perInstance(kuRuns, func(r *instRun) map[string]bool {
			return AntiOracleArchive(r.gold)
		})},
		{name: "FLOOR", role: "SELF-CHECK V7", sets: perInstance(kuRuns, func(r *instRun) map[string]bool {
			return FloorArchive(r.turns)
		})},
	}

	results := map[string]*armResult{}
	for _, a := range arms {
		res := h.scoreArm(kuRuns, a)
		results[a.name] = res
		h.reportArm(t, a, res, kuRuns)
		if a.role == "EXPLORATORY" {
			h.exploratoryTests++
		}
	}

	raw, prim := results["RAW"], results[arms[1].name]

	// --- the self-checks that VOID the run (5.2) ---
	h.checkVoids(t, results, arms, raw)

	// --- RANDOM-ARCHIVE, the mandatory rate-matched control (2.1) ---
	randMax, randMean := h.randomArchiveArm(t, kuRuns, d2Sets)

	// --- D3, the rival (2.3) ---
	d3Best, d3BestCell := h.recencyRivalArm(t, kuRuns)

	// --- the 398-query falsifier (3, 4.3) ---
	rawRecall, rawMRR := h.controlRecall(ctlRuns, nil)
	d2CtlSets := perInstance(ctlRuns, func(r *instRun) map[string]bool {
		return ArchiveSet(NearestOlderIDF(r.turns, thetaD2))
	})
	d2Recall, d2MRR := h.controlRecall(ctlRuns, d2CtlSets)
	t.Logf("--- CONTROL FALSIFIER, n=%d non-abstention queries (4.3) ---", len(ctlRuns))
	t.Logf("    RAW  recall@5 = %.4f   MRR = %.4f", rawRecall, rawMRR)
	t.Logf("    D2*  recall@5 = %.4f   MRR = %.4f   delta = %+.4f pp (F3 falsifies at worse than -2 pp)",
		d2Recall, d2MRR, 100*(d2Recall-rawRecall))

	// 5.2's third arithmetic self-check. ORACLE's archive set is defined only on
	// KU instances, so applying THAT MAP (not an empty one -- an empty map would
	// make this a tautology) to the control set must be an exact no-op. It is a
	// real check that a KU archive ID cannot leak onto a control instance.
	oracleArm, _ := findArm(arms, "ORACLE")
	oracleCtlRecall, _ := h.controlRecall(ctlRuns, oracleArm.sets)
	if oracleCtlRecall != rawRecall {
		t.Errorf("VOID: control recall@5 with ORACLE's KU archive map applied = %.6f, RAW = %.6f; "+
			"they must be exactly equal, so an archive set has leaked across query sets",
			oracleCtlRecall, rawRecall)
	}

	// 4.3: the 9 abstention queries are a separate, labelled EXPLORATORY row
	// with their own recall@5, carrying NO confirm/falsify weight. For those
	// queries retrieving the flagged turn is not the goal -- the task is to
	// abstain -- which is exactly why they are not in the falsifier's 398.
	absRuns := h.ingestAndRetrieve(t, h.abstain, false)
	absRawRecall, absRawMRR := h.controlRecall(absRuns, nil)
	absD2Recall, absD2MRR := h.controlRecall(absRuns, perInstance(absRuns, func(r *instRun) map[string]bool {
		return ArchiveSet(NearestOlderIDF(r.turns, thetaD2))
	}))
	h.exploratoryTests++
	t.Logf("--- ABSTENTION ROW [EXPLORATORY, no confirm/falsify weight], n=%d (4.3) ---", len(absRuns))
	t.Logf("    RAW recall@5 = %.4f MRR = %.4f  |  D2* recall@5 = %.4f MRR = %.4f. "+
		"For these queries the correct behaviour is to ABSTAIN, so a recall drop here is not "+
		"straightforwardly a cost.", absRawRecall, absRawMRR, absD2Recall, absD2MRR)

	// --- G1: the archived count, queried BACK from Qdrant (3.3, 4.12) ---
	// Never read off a clusterer log. This also applies the primary arm's
	// archive to the real collection, which is what PF4 then verifies against.
	h.applyAndVerifyArchive(t, kuRuns, d2Sets)

	// --- H1 (5.1) ---
	h.reportH1(t, raw, prim, rawRecall, d2Recall, randMax, randMean, d3Best, d3BestCell)

	if err := h.cache.Save(h.cachePath); err != nil {
		t.Logf("embedding cache not saved (%v); the next run re-embeds", err)
	}
	t.Logf("EXPLORATORY comparisons computed this run: %d (every non-confirmatory SR@1 or recall row "+
		"reported above). All are DESCRIPTIVE; no exploratory number may appear in a claim, and the "+
		"count is printed so a reader can apply their own multiplicity correction. The single "+
		"confirmatory test is D2* vs RAW on SR@1 over KU-permissive (1.2, 4.14).", h.exploratoryTests)
	t.Logf("HONEST RESULT LABEL: SR@1 over Qdrant top-%d cosine retrieval, all-MiniLM-L6-v2 384-dim "+
		"at %d wordpieces, with an offline archive filter. Measures whether forgetting puts the RIGHT "+
		"EVIDENCE in front of a reader -- NOT answer quality (there is no judge), NOT the knapsack "+
		"assembly or production read path, NOT the production write path, and NOT the repo's designed "+
		"fact-level supersession mechanism in conflict.go, which has never executed.",
		retrievalDepth, embedMaxSeqLen)
}

// --- ingest + one retrieval pass ---

func (h *harness) ingestAndRetrieve(t *testing.T, instances []*LMEInstance, isKU bool) []*instRun {
	t.Helper()
	start := time.Now()
	runs := make([]*instRun, 0, len(instances))
	embedded := 0

	for _, in := range instances {
		turns, err := in.Turns()
		if err != nil {
			t.Fatalf("%s: Turns: %v", in.QuestionID, err)
		}
		qTime, err := in.QuestionTime()
		if err != nil {
			t.Fatalf("%s: question_date: %v", in.QuestionID, err)
		}

		// 4.9: ingest in 4.4 order. clustering.go:82-84 flips a border point's
		// noise label on first touch, so unsorted ingestion makes D0
		// unreproducible and invites re-rolling until a clustering looks good.
		eps := make([]models.Episode, 0, len(turns))
		age := make(map[string]float64, len(turns))
		for _, tn := range turns {
			vec, ok := h.cache.Get(tn.Content)
			if !ok {
				// Batch size 1 throughout, and asserted as such: this quantized
				// model derives its int8 activation scale from whole-batch
				// statistics, so the same text embeds to ~0.99 cosine of itself
				// depending on batch composition -- the same order as the score
				// gaps that decide ranking (local_embed.go:100-110).
				vec, err = h.embedder.Embed(h.ctx, tn.Content)
				if err != nil {
					t.Fatalf("embed %s: %v", tn.ID, err)
				}
				h.cache.Put(tn.Content, vec)
				embedded++
			}
			if len(vec) != embedDim {
				t.Fatalf("embedding for %s has dim %d, want %d", tn.ID, len(vec), embedDim)
			}
			age[tn.ID] = qTime.Sub(tn.Timestamp()).Hours() / 24.0
			eps = append(eps, models.Episode{
				ID: tn.ID, UserID: in.QuestionID, Content: tn.Content,
				Embedding: vec, Timestamp: tn.Timestamp(), EventID: in.QuestionID,
				MemoryType: models.MemoryEpisodic,
				// PF0: explicit, never the zero value. Episodes built directly
				// bypass models.NewEpisode (models.go:64 is the only writer of
				// DecayFactor = 1.0), and dig.go:104's `score *= DecayFactor`
				// annihilates every score at 0.0.
				ConsolidationStatus: models.StatusPending,
				DecayFactor:         1.0,
				ImportanceScore:     0.0,
				AssociatedEntities:  []string{},
			})
		}
		if err := h.store.Upsert(h.ctx, eps); err != nil {
			t.Fatalf("%s: Upsert: %v", in.QuestionID, err)
		}

		qVec, ok := h.cache.Get(in.Question)
		if !ok {
			qVec, err = h.embedder.Embed(h.ctx, in.Question)
			if err != nil {
				t.Fatalf("embed question %s: %v", in.QuestionID, err)
			}
			h.cache.Put(in.Question, qVec)
			embedded++
		}
		hits, err := h.store.Search(h.ctx, in.QuestionID, qVec, retrievalDepth)
		if err != nil {
			t.Fatalf("%s: Search: %v", in.QuestionID, err)
		}
		ranked := make([]Ranked, 0, len(hits))
		for _, hit := range hits {
			ranked = append(ranked, Ranked{ID: hit.Episode.ID, Score: hit.Score})
		}
		if len(ranked) != len(turns) {
			t.Fatalf("%s: retrieval returned %d of %d turns at depth %d. 3.1's arithmetic "+
				"(20-24 turns against K=50) requires every turn to survive into R_X; "+
				"anything less means the index is incomplete",
				in.QuestionID, len(ranked), len(turns), retrievalDepth)
		}

		r := &instRun{inst: in, turns: turns, ranked: ranked, ageDays: age,
			goldAll: GoldIDs(turns)}
		if isKU {
			r.gold = GoldOf(turns)
		}
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool { // 4.8: ascending question_id byte order
		return runs[i].inst.QuestionID < runs[j].inst.QuestionID
	})
	t.Logf("INGEST+RETRIEVE: %d instances, %d turns, %d newly embedded (rest cached), %s",
		len(runs), countTurns(runs), embedded, time.Since(start).Round(time.Millisecond))
	return runs
}

func countTurns(runs []*instRun) int {
	n := 0
	for _, r := range runs {
		n += len(r.turns)
	}
	return n
}

func (h *harness) vecsOf(r *instRun) map[string][]float32 {
	out := make(map[string][]float32, len(r.turns))
	for _, tn := range r.turns {
		if v, ok := h.cache.Get(tn.Content); ok {
			out[tn.ID] = v
		}
	}
	return out
}

// --- pre-flight asserts PF0-PF4 (4.11) ---

func (h *harness) preflight(t *testing.T, runs []*instRun) {
	t.Helper()
	if len(runs) == 0 {
		t.Fatal("no instances to pre-flight")
	}

	// PF0: a sample round-trips through Qdrant reading back DecayFactor 1.0.
	// The plan's original assert ("rerank order == cosine order") is degenerate:
	// at the zero-value DecayFactor every score is exactly 0.0, min_score -0.5
	// keeps them all, and dig.go:75's sort.Slice is not stable, so it passes or
	// fails on a coin flip.
	sampled := 0
	for _, r := range runs {
		got, err := h.store.GetRecent(h.ctx, r.inst.QuestionID, len(r.turns))
		if err != nil {
			t.Fatalf("PF0 GetRecent: %v", err)
		}
		for _, ep := range got {
			if ep.DecayFactor != 1.0 {
				t.Fatalf("PF0 FAILED: episode %s read back DecayFactor %v, want 1.0. "+
					"dig.go:104 multiplies by this, so 0.0 annihilates every score", ep.ID, ep.DecayFactor)
			}
			if ep.ConsolidationStatus != models.StatusPending {
				t.Fatalf("PF0 FAILED: episode %s read back status %q, want %q",
					ep.ID, ep.ConsolidationStatus, models.StatusPending)
			}
			sampled++
			if sampled >= 20 {
				break
			}
		}
		if sampled >= 20 {
			break
		}
	}
	t.Logf("PF0 PASS: %d points round-tripped with DecayFactor 1.0 and status pending", sampled)

	// PF1: a check that cannot pass for the wrong reason. Distinct cosine
	// scores make sort.Slice's instability irrelevant; identical timestamps
	// make dig.go:98's recency term a shared constant.
	ts := time.Date(2023, 5, 1, 12, 0, 0, 0, time.UTC)
	var cands []models.RetrievalResult
	for _, s := range []float64{0.9, 0.7, 0.5, 0.3, 0.1} {
		cands = append(cands, models.RetrievalResult{
			Score: s, Source: "vector",
			// Content must be non-empty: dig.go:57 skips a candidate whose
			// extractContent is "", which silently drops it from the output
			// rather than scoring it -- found by this assert failing.
			Episode: &models.Episode{ID: fmt.Sprintf("pf1-%.1f", s), Content: fmt.Sprintf("candidate %.1f", s),
				Timestamp: ts, ImportanceScore: 0, DecayFactor: 1.0},
		})
	}
	reranked, err := dig.NewReranker(nil, configs.DIGConfig{MinScore: -0.5}).Rerank(h.ctx, "q", cands)
	if err != nil {
		t.Fatalf("PF1 Rerank: %v", err)
	}
	if len(reranked) != len(cands) {
		t.Fatalf("PF1 FAILED: rerank returned %d of %d candidates", len(reranked), len(cands))
	}
	for i := 1; i < len(reranked); i++ {
		if reranked[i-1].Result.Score < reranked[i].Result.Score {
			t.Fatalf("PF1 FAILED: rerank did not return strictly descending cosine order")
		}
	}
	t.Logf("PF1 PASS: DIG's reranker collapses to cosine order on distinct scores with a constant " +
		"recency term -- which is why the reranker is OFF in every arm")

	// PF2: the DecayFactor trap recorded as a runnable check rather than
	// tripped over.
	zeroDecay, err := dig.NewReranker(nil, configs.DIGConfig{MinScore: -1e9}).Rerank(h.ctx, "q",
		[]models.RetrievalResult{{Score: 0.9, Episode: &models.Episode{ID: "pf2", Content: "pf2 candidate",
			Timestamp: ts, DecayFactor: 0.0}}})
	if err != nil {
		t.Fatalf("PF2 Rerank: %v", err)
	}
	if len(zeroDecay) != 1 || zeroDecay[0].DIGScore != 0.0 {
		t.Fatalf("PF2 FAILED: a candidate with DecayFactor 0.0 scored %v, want exactly 0.0 "+
			"(dig.go:104 annihilates the whole score)", zeroDecay)
	}
	t.Logf("PF2 PASS: DecayFactor 0.0 annihilates the DIG score to exactly 0.0, as dig.go:104 dictates")

	// PF3: normalisation. v and 2v must score identically under cosine.
	h.preflightNormalisation(t)

	// PF4 runs at the END of the run, in applyAndVerifyArchive: it needs a real
	// archive applied, and applying one early would corrupt the shared
	// retrieval pass that every arm filters.
}

func (h *harness) preflightNormalisation(t *testing.T) {
	t.Helper()
	const uid = "pf3-normalisation"
	vec := make([]float32, embedDim)
	doubled := make([]float32, embedDim)
	for i := range vec {
		vec[i] = float32(i%7) + 1
		doubled[i] = 2 * vec[i]
	}
	eps := []models.Episode{
		{ID: "00000000-0000-4000-8000-0000000000f3", UserID: uid, Content: "v", Embedding: vec,
			Timestamp: time.Unix(1700000000, 0), ConsolidationStatus: models.StatusPending, DecayFactor: 1.0},
		{ID: "00000000-0000-4000-8000-0000000000f4", UserID: uid, Content: "2v", Embedding: doubled,
			Timestamp: time.Unix(1700000000, 0), ConsolidationStatus: models.StatusPending, DecayFactor: 1.0},
	}
	if err := h.store.Upsert(h.ctx, eps); err != nil {
		t.Fatalf("PF3 Upsert: %v", err)
	}
	hits, err := h.store.Search(h.ctx, uid, vec, 10)
	if err != nil {
		t.Fatalf("PF3 Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("PF3 FAILED: search returned %d points, want 2", len(hits))
	}
	if math.Abs(hits[0].Score-hits[1].Score) > 1e-5 {
		t.Fatalf("PF3 FAILED: v and 2v scored %.8f and %.8f; Distance_Cosine must be scale-invariant",
			hits[0].Score, hits[1].Score)
	}
	if err := h.store.DeleteByIDs(h.ctx, []string{eps[0].ID, eps[1].ID}); err != nil {
		t.Fatalf("PF3 cleanup: %v", err)
	}
	t.Logf("PF3 PASS: v and 2v score identically (%.6f); cosine is scale-invariant as assumed", hits[0].Score)
}

// --- theta* selection (4.6) ---

func (h *harness) selectTheta(t *testing.T, ctl []*instRun, arm string, grid []float64,
	detect func(*instRun, float64) map[string]bool) float64 {
	t.Helper()
	total := countTurns(ctl)
	best, bestRate, bestGap := grid[0], 0.0, math.Inf(1)
	t.Logf("--- %s theta* selection: archive rate over the %d CONTROL instances, target rho*=%.4f (4.6) ---",
		arm, len(ctl), rhoStar)
	for _, th := range grid {
		archived := 0
		for _, r := range ctl {
			archived += len(detect(r, th))
		}
		rate := float64(archived) / float64(total)
		gap := math.Abs(rate - rhoStar)
		t.Logf("    theta=%.2f  archived %d / %d turns = %.4f   |rate - rho*| = %.4f", th, archived, total, rate, gap)
		// Ties go to the LOWER archive rate: conservative, fewer archives, less
		// collateral risk on the falsifier (4.6).
		if gap < bestGap || (gap == bestGap && rate < bestRate) {
			best, bestRate, bestGap = th, rate, gap
		}
	}
	if best == grid[0] || best == grid[len(grid)-1] {
		t.Logf("    NOTE: theta* = %.2f is a GRID ENDPOINT, i.e. the target rate lies outside the "+
			"registered grid. The endpoint remains theta*. The grid is NOT extended -- extending it "+
			"after seeing rates is the forking path the registration exists to close (4.6).", best)
	}
	t.Logf("    theta*(%s) = %.2f at control archive rate %.4f", arm, best, bestRate)
	return best
}

// --- arms and scoring ---

type armSpec struct {
	name    string
	role    string
	sets    map[string]map[string]bool // question_id -> archive set
	pairs   map[string][]Pair          // question_id -> detector decisions; nil for arms that make none
	primary bool
}

type armResult struct {
	outcomes  []int // per instance, in question_id order
	mean      float64
	defined   int
	undefined int
	archived  int
	precision float64 // |A n gold-stale| / |A|
	recall    float64 // |A n gold-stale| / |gold-stale|

	// pairCatch is a DIFFERENT number from recall, and 3.1 requires they never
	// be presented as two corroborating findings: it is the fraction of
	// instances where the detector anchored on a CURRENT gold turn and selected
	// that turn's own STALE gold as its nearest-older. That is the mechanism
	// under test. archive recall counts a stale gold archived for any reason,
	// including as some unrelated turn's nearest neighbour.
	pairCatch    float64
	pairCatchN   int
	pairInstance int
	hasPairs     bool
	sameSess     int // pairs whose anchor and target share a session
	crossSess    int
}

func perInstance(runs []*instRun, f func(*instRun) map[string]bool) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(runs))
	for _, r := range runs {
		out[r.inst.QuestionID] = f(r)
	}
	return out
}

func (h *harness) scoreArm(runs []*instRun, a armSpec) *armResult {
	res := &armResult{outcomes: make([]int, 0, len(runs)), hasPairs: a.pairs != nil}
	var hitStale, totalStale int
	for _, r := range runs {
		set := a.sets[r.inst.QuestionID]
		res.outcomes = append(res.outcomes, SRAt1(Filter(r.ranked, set, armCutoff), r.gold))
		res.archived += len(set)
		totalStale += len(r.gold.Stale)
		for id := range set {
			if r.gold.Stale[id] {
				hitStale++
			}
		}
		if !res.hasPairs {
			continue
		}
		sessOf := make(map[string]int, len(r.turns))
		for _, tn := range r.turns {
			sessOf[tn.ID] = tn.SessionIdx
		}
		caught := false
		for _, p := range a.pairs[r.inst.QuestionID] {
			if sessOf[p.Anchor] == sessOf[p.Target] {
				res.sameSess++
			} else {
				res.crossSess++
			}
			// The mechanism: anchored on a current gold, selected its own
			// stale gold as the nearest strictly-older turn.
			if r.gold.Current[p.Anchor] && r.gold.Stale[p.Target] {
				caught = true
			}
		}
		if len(r.gold.Stale) > 0 {
			res.pairInstance++
			if caught {
				res.pairCatchN++
			}
		}
	}
	res.mean, res.defined, res.undefined = MeanDefined(res.outcomes)
	if res.archived > 0 {
		res.precision = float64(hitStale) / float64(res.archived)
	}
	if totalStale > 0 {
		res.recall = float64(hitStale) / float64(totalStale)
	}
	if res.pairInstance > 0 {
		res.pairCatch = float64(res.pairCatchN) / float64(res.pairInstance)
	}
	return res
}

func (h *harness) reportArm(t *testing.T, a armSpec, res *armResult, runs []*instRun) {
	t.Helper()
	t.Logf("--- ARM %s [%s] ---", a.name, a.role)
	t.Logf("    SR@1 = %.4f over %d defined queries (%d undefined, evidence-found rate %.4f)",
		res.mean, res.defined, res.undefined, float64(res.defined)/float64(len(runs)))
	t.Logf("    archived %d turns; archive precision %.4f, archive recall %.4f",
		res.archived, res.precision, res.recall)
	if res.hasPairs {
		t.Logf("    pair-catch %d/%d = %.4f (anchored on a CURRENT gold, selected its own STALE gold). "+
			"3.1: this and delta-SR@1 are arithmetically LINKED, never two corroborating findings. "+
			"Pairs: same-session %d / cross-session %d",
			res.pairCatchN, res.pairInstance, res.pairCatch, res.sameSess, res.crossSess)
	}
	if res.archived > 0 {
		h.attribution(t, a, res, runs)
	}
}

// attribution is the mandatory 2x2 of 3.2. Per 3.1's arithmetic, in the primary
// row R_X holds every unarchived turn, so SR@1 cannot see a non-gold archive:
// the "own stale gold NOT archived" column must be identical to RAW unless the
// detector archived one of that query's CURRENT gold turns. A difference there
// is a harness bug (V9), not a finding.
func (h *harness) attribution(t *testing.T, a armSpec, res *armResult, runs []*instRun) {
	t.Helper()
	var inHit, inMiss, outHit, outMiss int
	for i, r := range runs {
		set := a.sets[r.inst.QuestionID]
		staleArchived := false
		for id := range r.gold.Stale {
			if set[id] {
				staleArchived = true
			}
		}
		switch {
		case staleArchived && res.outcomes[i] == SRCurrent:
			inHit++
		case staleArchived:
			inMiss++
		case res.outcomes[i] == SRCurrent:
			outHit++
		default:
			outMiss++
		}
	}
	t.Logf("    attribution 2x2 (3.2):  own stale gold IN A -> SR@1=1: %d, SR@1=0: %d  |  "+
		"NOT in A -> SR@1=1: %d, SR@1=0: %d", inHit, inMiss, outHit, outMiss)
}

// --- the void conditions (5.2) ---

// findArm looks an arm up by name prefix, so a theta-bearing name like
// "D2 (theta*=0.25)" stays addressable without hard-coding the theta.
func findArm(arms []armSpec, prefix string) (armSpec, int) {
	for i, a := range arms {
		if strings.HasPrefix(a.name, prefix) {
			return a, i
		}
	}
	return armSpec{}, -1
}

func (h *harness) checkVoids(t *testing.T, results map[string]*armResult, arms []armSpec, raw *armResult) {
	t.Helper()
	find := func(prefix string) (armSpec, *armResult) {
		a, i := findArm(arms, prefix)
		if i < 0 {
			t.Fatalf("arm %s not found", prefix)
		}
		return a, results[a.name]
	}

	// V3: RAW's evidence-found rate. 3.1 proves it must be 1.0 (20-24 turns
	// against K=50); anything less means the index or query set is broken.
	evidence := float64(raw.defined) / float64(raw.defined+raw.undefined)
	if evidence < 0.95 {
		t.Errorf("VOID (V3): RAW evidence-found rate = %.4f, below 0.95. 3.1's arithmetic says "+
			"it must be 1.0, so the index or the query set is broken", evidence)
	}

	_, oracle := find("ORACLE")
	if oracle.mean != 1.0 {
		t.Errorf("VOID (V2): SR@1(ORACLE) = %.6f, must be exactly 1.0 -- the gold map or the "+
			"archive filter is wrong", oracle.mean)
	}
	_, anti := find("ANTI-ORACLE")
	if anti.mean != 0.0 {
		t.Errorf("VOID (V1): SR@1(ANTI-ORACLE) = %.6f, must be exactly 0.0 -- the harness reads "+
			"the gold map in the wrong orientation", anti.mean)
	}
	// 2.2's vacuity guard: SR@1 is undefined when no gold survives, and a mean
	// over an empty set would report 0.0 for the wrong reason.
	if anti.defined < raw.defined/2 {
		t.Errorf("VOID (V1): ANTI-ORACLE defined-query count %d < 0.5 x RAW's %d -- the check is "+
			"vacuous over a near-empty set", anti.defined, raw.defined)
	}
	_, floor := find("FLOOR")
	if floor.mean != 1.0 {
		t.Errorf("VOID (V7): SR@1(FLOOR) = %.6f, must be exactly 1.0 -- archiving does not actually "+
			"remove points from retrieval, so the mechanism under test is inert", floor.mean)
	}
	t.Logf("SELF-CHECKS: ORACLE=%.4f (V2 wants 1.0), ANTI-ORACLE=%.4f over %d defined queries "+
		"(V1 wants 0.0 and >= %d defined), FLOOR=%.4f (V7 wants 1.0), RAW evidence-found=%.4f (V3 wants >= 0.95)",
		oracle.mean, anti.mean, anti.defined, raw.defined/2, floor.mean, evidence)

	// V4, the premise gate. Only meaningful at full n.
	if !h.full {
		t.Logf("SMOKE RUN: V4 (RAW SR@1 in [0.35, 0.65]) and every H1 clause are NOT evaluated. "+
			"RAW SR@1 here is %.4f over %d queries and is a machinery check, not a result.",
			raw.mean, raw.defined)
		return
	}
	if raw.mean < 0.35 || raw.mean > 0.65 {
		t.Errorf("VOID (V4, the premise gate): SR@1(RAW) = %.4f is outside [0.35, 0.65]. "+
			"Above 0.65 there is not enough headroom for a +10pp claim to mean what was registered; "+
			"below 0.35 the harness and the probe disagree about the corpus by more than 12pp and "+
			"that must be reconciled and reported BEFORE any arm is compared. Either way SR@1(RAW) "+
			"is a finding in its own right.", raw.mean)
	}
}

// --- RANDOM-ARCHIVE (2.1) ---

func (h *harness) randomArchiveArm(t *testing.T, runs []*instRun, d2Sets map[string]map[string]bool) (max, mean float64) {
	t.Helper()
	t.Logf("--- ARM RANDOM-ARCHIVE [mandatory H1 clause], %d replicates, per-instance exact rate match ---",
		randomReplicas)
	max = math.Inf(-1)
	var sum float64
	for r := 0; r < randomReplicas; r++ {
		rng := rand.New(rand.NewSource(randomBaseSeed + int64(r)))
		sets := make(map[string]map[string]bool, len(runs))
		for _, run := range runs { // runs are already in ascending question_id order (4.8)
			sets[run.inst.QuestionID] = RandomArchive(run.turns, len(d2Sets[run.inst.QuestionID]), rng)
		}
		res := h.scoreArm(runs, armSpec{name: "RANDOM", sets: sets})
		sum += res.mean
		if res.mean > max {
			max = res.mean
		}
		t.Logf("    seed %d: SR@1 = %.4f (%d archived)", randomBaseSeed+r, res.mean, res.archived)
	}
	mean = sum / float64(randomReplicas)
	t.Logf("    RANDOM-ARCHIVE over %d replicates: mean %.4f, MAX %.4f. D2* must beat the MAX "+
		"(a one-sided permutation-style threshold at p ~ 1/21 = 0.048, exact).", randomReplicas, mean, max)
	t.Logf("    REGISTERED EXPECTATION (3.1): in the primary row R_X holds every unarchived turn, so " +
		"SR@1 cannot see a non-gold archive and RANDOM's expected delta is about zero. A near-null " +
		"RANDOM-ARCHIVE is a PASS, not a failed control.")
	return max, mean
}

// --- D3, the rival (2.3) ---

func (h *harness) recencyRivalArm(t *testing.T, runs []*instRun) (best float64, cell string) {
	t.Helper()
	t.Logf("--- ARM D3 (global recency prior) [EXPLORATORY rival], all 9 cells reported ---")
	t.Logf("    D3 is NOT a novel rival: force_recent_turns: 3 (configs/config.yaml:42) is already " +
		"a recency prior in production.")
	best = math.Inf(-1)
	for _, w := range gridD3w {
		for _, tau := range gridD3tau {
			outcomes := make([]int, 0, len(runs))
			for _, r := range runs {
				outcomes = append(outcomes, SRAt1(Filter(RecencyRescore(r.ranked, r.ageDays, w, tau), nil, armCutoff), r.gold))
			}
			m, defined, _ := MeanDefined(outcomes)
			h.exploratoryTests++
			t.Logf("    w=%.2f tau=%3.0fd: SR@1 = %.4f over %d defined", w, tau, m, defined)
			if m > best {
				best, cell = m, fmt.Sprintf("w=%.2f tau=%.0fd", w, tau)
			}
		}
	}
	t.Logf("    D3 best cell: %s at SR@1 = %.4f (D3 selects its own best cell on the outcome "+
		"metric -- it is the rival, and handicapping ourselves against it is the point)", cell, best)
	return best, cell
}

// --- control falsifier (3, 4.3) ---

func (h *harness) controlRecall(runs []*instRun, sets map[string]map[string]bool) (recall5, mrr float64) {
	if len(runs) == 0 {
		return 0, 0
	}
	var hits, rrSum float64
	for _, r := range runs {
		filtered := Filter(r.ranked, sets[r.inst.QuestionID], armCutoff)
		if RecallAt(filtered, r.goldAll, 5) {
			hits++
		}
		rrSum += ReciprocalRank(filtered, r.goldAll)
	}
	n := float64(len(runs))
	return hits / n, rrSum / n
}

// --- G1 + PF4: apply the primary archive for real, then verify (4.11, 4.12) ---

func (h *harness) applyAndVerifyArchive(t *testing.T, runs []*instRun, sets map[string]map[string]bool) {
	t.Helper()
	// Ordering matters and is registered (4.13): upsert-all -> detect ->
	// archive -> search, never archive -> upsert. QdrantStore.Upsert rewrites
	// the WHOLE payload map and has no archive keys, so re-upserting after
	// archiving would silently reset consolidation_status. Nothing re-upserts
	// after this point.
	want := 0
	for _, r := range runs {
		ids := make([]string, 0, len(sets[r.inst.QuestionID]))
		for id := range sets[r.inst.QuestionID] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if err := h.store.ArchiveByIDs(h.ctx, ids); err != nil {
			t.Fatalf("ArchiveByIDs(%s): %v", r.inst.QuestionID, err)
		}
		want += len(ids)
	}

	// G1: the archived count comes BACK from Qdrant over raw HTTP, never off a
	// clusterer log (3.3, 4.12). Raw HTTP is also an independent path from the
	// gRPC client that wrote it.
	got := h.countArchivedHTTP(t)
	t.Logf("G1: Qdrant reports %d points with consolidation_status=archived; the detector claimed %d", got, want)
	if got != want {
		t.Errorf("G1 FAILED: Qdrant holds %d archived points, the detector archived %d", got, want)
	}
	if want == 0 {
		t.Errorf("VOID (V6): the primary arm archived nothing at theta*. Reported as a FINDING " +
			"about D2 at theta*, not as a null about H1 (5.1).")
		return
	}

	// PF4: the assert that licenses the entire offline simulation. Real Search
	// with the new MustNot must return EXACTLY the offline-computed R_X.
	checked := 0
	for _, r := range runs {
		if checked >= 20 {
			break
		}
		qVec, ok := h.cache.Get(r.inst.Question)
		if !ok {
			continue
		}
		hits, err := h.store.Search(h.ctx, r.inst.QuestionID, qVec, retrievalDepth)
		if err != nil {
			t.Fatalf("PF4 Search: %v", err)
		}
		live := make([]Ranked, 0, len(hits))
		for _, hit := range hits {
			live = append(live, Ranked{ID: hit.Episode.ID, Score: hit.Score})
		}
		offline := Filter(r.ranked, sets[r.inst.QuestionID], retrievalDepth)
		if len(live) != len(offline) {
			t.Fatalf("VOID (V5, PF4): %s live Search returned %d points, offline R_X has %d. "+
				"The offline simulation is not licensed.", r.inst.QuestionID, len(live), len(offline))
		}
		for i := range live {
			if live[i].ID != offline[i].ID {
				t.Fatalf("VOID (V5, PF4): %s position %d is %s live and %s offline. "+
					"The offline simulation is not licensed.", r.inst.QuestionID, i, live[i].ID, offline[i].ID)
			}
		}
		checked++
	}
	t.Logf("PF4 PASS: real Search + MustNot returned exactly the offline R_X on %d sampled queries. "+
		"This is what licenses every arm above being computed offline.", checked)
}

// countArchivedHTTP is the raw Qdrant points/count of 4.12: no new interface
// method, and an independent read path from the gRPC client that did the write.
func (h *harness) countArchivedHTTP(t *testing.T) int {
	t.Helper()
	body := strings.NewReader(`{"exact":true,"filter":{"must":[{"key":"consolidation_status",` +
		`"match":{"value":"archived"}}]}}`)
	url := fmt.Sprintf("http://%s/collections/%s/points/count", qdrantHTTPAddr, h.collection)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		t.Fatalf("G1 build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("G1 count request: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("G1 decode: %v", err)
	}
	return out.Result.Count
}

// --- H1 (5.1) ---

func (h *harness) reportH1(t *testing.T, raw, prim *armResult, rawRecall, d2Recall, randMax, randMean, d3Best float64, d3Cell string) {
	t.Helper()
	b, c, skipped := Discordant(prim.outcomes, raw.outcomes)
	p := McNemarExact(b, c)
	delta := prim.mean - raw.mean
	// n is the PAIRED count -- queries where both arms are defined -- not one
	// arm's defined count. McNemar is a paired test and the equivalence bound
	// must use the same denominator the discordant pairs came from.
	paired := len(prim.outcomes) - skipped
	half := WilsonHalfWidth(b, c, paired)

	t.Logf("=== H1, THE PRIMARY CELL (1.1, 1.2) ===")
	t.Logf("    SR@1(RAW) = %.4f   SR@1(D2*) = %.4f   delta = %+.2f pp", raw.mean, prim.mean, 100*delta)
	t.Logf("    exact two-sided McNemar: b=%d c=%d (raw discordant counts), %d queries skipped as "+
		"undefined, p = %.6f. ONE confirmatory test; no correction because there is one test (4.14).",
		b, c, skipped, p)
	t.Logf("    equivalence half-width = %.4f over %d paired queries (registered bound +/- 0.10, "+
		"which needs b+c <= 12 at n=70)", half, paired)

	if !h.full {
		t.Logf("    SMOKE RUN: the clauses below are NOT evaluated and these numbers are not results.")
		return
	}

	fail := func(id, msg string, args ...any) {
		t.Errorf("H1 FALSIFIED at %s: "+msg+"  (A falsified H1 is a RESULT, not a failure -- 5.1.)",
			append([]any{id}, args...)...)
	}
	if delta < 0.10 {
		fail("F1 (effect)", "SR@1(D2*) - SR@1(RAW) = %+.2f pp, below the registered +10 pp", 100*delta)
	}
	if p >= 0.05 {
		fail("F2 (significance)", "McNemar p = %.4f >= 0.05. Report the +/-%.1f pp equivalence "+
			"interval, never a bare p > 0.05.", p, 100*half)
	}
	if rawRecall-d2Recall > 0.02 {
		fail("F3 (cost)", "control recall@5 dropped %.2f pp on the 398, more than the registered 2 pp. "+
			"This is 'forgetting buys freshness with recall', which is NOT the thesis's claim.",
			100*(rawRecall-d2Recall))
	}
	if prim.mean <= randMax {
		fail("F4 (mechanism)", "SR@1(D2*) = %.4f <= max over %d RANDOM-ARCHIVE replicates = %.4f "+
			"(mean %.4f). A positive result cannot be distinguished from 'archiving anything helps'.",
			prim.mean, randomReplicas, randMax, randMean)
	}
	rivalBar := math.Max(d3Best, raw.mean+0.10)
	if prim.mean <= rivalBar {
		fail("F5 (rival)", "SR@1(D2*) = %.4f <= max(D3 best %.4f at %s, RAW+10pp %.4f) = %.4f",
			prim.mean, d3Best, d3Cell, raw.mean+0.10, rivalBar)
	}
	if d3Best < raw.mean {
		t.Logf("    NOTE, registered in 2.3: D3's best cell (%.4f) fell BELOW RAW (%.4f). The "+
			"rival-beating clause was satisfied vacuously and carries no evidential weight.",
			d3Best, raw.mean)
	}
	t.Logf("    SR@1 must never be quoted without control recall@5 beside it: it is structurally "+
		"blind to collateral damage in the primary row (3.1, 6.8). RAW %.4f -> D2* %.4f.",
		rawRecall, d2Recall)
}
