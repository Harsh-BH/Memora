// Command probe re-derives every number the forgetting experiment's design
// rests on, from the sha256-verified corpus and the real ONNX embedder.
//
// It exists because the measurement that CHOSE that design is unreproducible by
// its own admission -- "the probe files were deleted after use" (gap 10). Every
// value the pre-registration labels PROVISIONAL-UNREPRODUCED is unusable in any
// write-up until this program reproduces it. The registered rule, from
// PREREGISTRATION.md 4.10:
//
//	If this script's value differs from the recorded value, BOTH are printed
//	side by side with the delta, and THIS SCRIPT'S VALUE IS AUTHORITATIVE. If
//	it cannot reproduce a number at all, that number is STRUCK from the record.
//	Deleting an unreproducible number is a successful outcome of this rule.
//
// Deterministic, and it takes no flags that change its output. Writes
// cma/eval/out/probe.json plus the frozen artifacts: the gold map CSV and the
// KU-strict ID list.
//
//	go run ./eval/cmd/probe            # from cma/, with LD_LIBRARY_PATH set
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/memora/cma/eval"
	"github.com/memora/cma/internal/llm"
)

// recorded is the PROVISIONAL-UNREPRODUCED table from the design pass, quoted
// here so every line of output can print measured-vs-recorded and the delta.
// These are the values under test; the measured column is what survives.
var recorded = map[string]float64{
	"d_stale_current_p50":           0.514,
	"d_current_nearest_nongold_p50": 0.217,
	"dense_top1_rate":               5.0 / 70.0,  // 7.1%
	"lexical_top1_rate":             15.0 / 70.0, // 21.4%
	"gold_pairs_within_eps_0.3":     6.0 / 70.0,  // 8.6%
	"false_neighbour_within_0.3":    60.0 / 70.0,
	"flat_cosine_prefers_current":   33.0 / 70.0, // 47.1%
	"ku_strict_n":                   49,
	"truncated_at_256_frac":         0.449,
}

type probeOut struct {
	GeneratedAt string             `json:"generated_at"`
	CorpusSHA   string             `json:"corpus_sha256"`
	MaxSeqLen   int                `json:"max_seq_len"`
	Dim         int                `json:"dim"`
	Instances   int                `json:"ku_permissive_instances"`
	Measured    map[string]float64 `json:"measured"`
	Recorded    map[string]float64 `json:"recorded_provisional"`
	Delta       map[string]float64 `json:"delta_measured_minus_recorded"`
	Percentiles map[string][]pct   `json:"percentiles"`
	Throughput  []throughput       `json:"embedder_throughput"`
	Notes       []string           `json:"notes"`
}

type pct struct {
	Q float64 `json:"q"`
	V float64 `json:"v"`
}

type throughput struct {
	MaxSeqLen int     `json:"max_seq_len"`
	Texts     int     `json:"texts"`
	Seconds   float64 `json:"seconds"`
	TextsPerS float64 `json:"texts_per_second"`
	Projected float64 `json:"projected_minutes_for_10960_turns"`
	TruncFrac float64 `json:"fraction_of_turns_truncated"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "probe: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	outDir := filepath.Join(root, "eval", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// V8 applies here too: a probe over the wrong corpus is worse than none.
	all, err := eval.LoadLongMemEval(eval.LongMemEvalPath())
	if err != nil {
		return err
	}
	ku := eval.Select(all, eval.IsKUPermissive)
	fmt.Printf("corpus: %d instances, KU-permissive n=%d\n", len(all), len(ku))

	third := filepath.Join(root, "..", "cma", "third_party")
	if _, err := os.Stat(third); err != nil {
		third = filepath.Join(root, "third_party")
	}
	lib := filepath.Join(third, "onnxruntime", "lib", "libonnxruntime.so")
	model := filepath.Join(third, "models", "all-MiniLM-L6-v2", "model_quint8_avx2.onnx")
	vocab := filepath.Join(third, "models", "all-MiniLM-L6-v2", "vocab.txt")

	const maxSeq, dim = 256, 384
	embedder, err := llm.NewLocalEmbedProvider(lib, model, vocab, maxSeq, dim)
	if err != nil {
		return fmt.Errorf("embedder: %w", err)
	}
	defer embedder.Close()

	modelSHA, err := eval.FileSHA256(model)
	if err != nil {
		return err
	}
	cachePath := filepath.Join(eval.DatasetDir(), fmt.Sprintf("embed_cache_minilm%d_%d.gob", dim, maxSeq))
	cache := eval.LoadEmbedCache(cachePath, modelSHA, maxSeq, dim)
	ctx := context.Background()
	embed := func(text string) ([]float32, error) {
		if v, ok := cache.Get(text); ok {
			return v, nil
		}
		v, err := embedder.Embed(ctx, text) // batch size 1 throughout (4.9)
		if err != nil {
			return nil, err
		}
		cache.Put(text, v)
		return v, nil
	}

	out := probeOut{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		CorpusSHA:   eval.SHA256LongMemEval,
		MaxSeqLen:   maxSeq, Dim: dim, Instances: len(ku),
		Measured: map[string]float64{}, Recorded: map[string]float64{},
		Delta: map[string]float64{}, Percentiles: map[string][]pct{},
	}

	// --- the frozen gold map, written before any statistic uses it ---
	var (
		dStaleCurrent   []float64 // one per (stale, current) gold pair
		dCurrentNonGold []float64 // one per instance: nearest non-gold to the current gold
		denseTop1Ranks  []float64
		lexTop1Ranks    []float64
		denseTop1       int
		lexTop1         int
		pairWithinEps   int
		falseNeighbour  int
		prefersCurrent  int
		answerOffsets   []float64
	)
	goldCSV, err := os.Create(filepath.Join(outDir, "gold_map.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(goldCSV)
	_ = w.Write([]string{"question_id", "turn_id", "label", "session_idx", "turn_idx", "role", "content_sha256"})

	for _, in := range ku {
		turns, err := in.Turns()
		if err != nil {
			return err
		}
		gold := eval.GoldOf(turns)
		for _, t := range turns {
			label := ""
			switch {
			case gold.Current[t.ID]:
				label = "current"
			case gold.Stale[t.ID]:
				label = "stale"
			default:
				continue
			}
			_ = w.Write([]string{in.QuestionID, t.ID, label,
				fmt.Sprint(t.SessionIdx), fmt.Sprint(t.TurnIdx), t.Role, eval.TextKey(t.Content)})
		}

		vecs := make(map[string][]float32, len(turns))
		for _, t := range turns {
			if vecs[t.ID], err = embed(t.Content); err != nil {
				return err
			}
		}

		var cur eval.Turn
		var stales []eval.Turn
		for _, t := range turns {
			switch {
			case gold.Current[t.ID]:
				cur = t
			case gold.Stale[t.ID]:
				stales = append(stales, t)
			}
		}
		if cur.ID == "" || len(stales) == 0 {
			continue
		}

		// d(stale gold, current gold): one value per PAIR. Two instances carry
		// two stale golds, so this is 72 values over 70 instances -- stated
		// because the recorded table reports 70 and the difference is real.
		nearestStale, nearestStaleD := stales[0], math.Inf(1)
		for _, s := range stales {
			d := eval.CosineDistance(vecs[s.ID], vecs[cur.ID])
			dStaleCurrent = append(dStaleCurrent, d)
			if d < nearestStaleD {
				nearestStale, nearestStaleD = s, d
			}
		}
		if nearestStaleD <= 0.3 {
			pairWithinEps++
		}

		// d(current gold, nearest NON-gold turn in the same instance), and
		// whether any non-gold turn is inside 0.30 of the current gold.
		nonGoldBest := math.Inf(1)
		for _, t := range turns {
			if t.ID == cur.ID || gold.IsGold(t.ID) {
				continue
			}
			if d := eval.CosineDistance(vecs[t.ID], vecs[cur.ID]); d < nonGoldBest {
				nonGoldBest = d
			}
		}
		if !math.IsInf(nonGoldBest, 1) {
			dCurrentNonGold = append(dCurrentNonGold, nonGoldBest)
			if nonGoldBest <= 0.30 {
				falseNeighbour++
			}
		}

		// Rank of the nearest stale twin among all other turns, by dense
		// distance and by IDF-weighted Jaccard. Rank 1 means the stale twin IS
		// the current turn's nearest neighbour.
		denseRank := rankOfStale(turns, cur, gold, func(a, b eval.Turn) float64 {
			return eval.CosineDistance(vecs[a.ID], vecs[b.ID])
		})
		sim := eval.NewIDFSimilarity(turns)
		lexRank := rankOfStale(turns, cur, gold, func(a, b eval.Turn) float64 {
			return -sim(a, b) // negated: rankOfStale orders ascending
		})
		if denseRank > 0 {
			denseTop1Ranks = append(denseTop1Ranks, float64(denseRank))
			if denseRank == 1 {
				denseTop1++
			}
		}
		if lexRank > 0 {
			lexTop1Ranks = append(lexTop1Ranks, float64(lexRank))
			if lexRank == 1 {
				lexTop1++
			}
		}

		// The flat-cosine current-preference proxy: is the QUESTION closer to
		// the current gold than to its nearest stale twin? This is the
		// headroom claim the whole thesis rests on.
		qVec, err := embed(in.Question)
		if err != nil {
			return err
		}
		if eval.CosineDistance(qVec, vecs[cur.ID]) < eval.CosineDistance(qVec, vecs[nearestStale.ID]) {
			prefersCurrent++
		}

		// Where inside its gold turn does an answer token first appear?
		if off := answerOffset(in, cur.Content); off >= 0 {
			answerOffsets = append(answerOffsets, float64(off))
		}
	}
	w.Flush()
	goldCSV.Close()
	if err := w.Error(); err != nil {
		return err
	}

	n := float64(len(ku))
	record := func(key string, measured float64) {
		out.Measured[key] = measured
		if rec, ok := recorded[key]; ok {
			out.Recorded[key] = rec
			out.Delta[key] = measured - rec
			status := "REPRODUCED"
			if math.Abs(measured-rec) > 0.005*math.Max(1, math.Abs(rec)) {
				status = "DIFFERS -- the measured value is authoritative (4.10)"
			}
			fmt.Printf("  %-32s measured %-10.4f recorded %-10.4f delta %+.4f  %s\n",
				key, measured, rec, measured-rec, status)
		} else {
			fmt.Printf("  %-32s measured %-10.4f (no recorded value)\n", key, measured)
		}
	}

	fmt.Printf("\n=== SECTION 0 RE-DERIVATION (%d KU-permissive instances, %d gold pairs) ===\n",
		len(ku), len(dStaleCurrent))
	sort.Float64s(dStaleCurrent)
	sort.Float64s(dCurrentNonGold)
	out.Percentiles["d_stale_current"] = pcts(dStaleCurrent, 0.10, 0.25, 0.50, 0.75, 0.90)
	out.Percentiles["d_current_nearest_nongold"] = pcts(dCurrentNonGold, 0.10, 0.50, 0.90)
	out.Percentiles["dense_stale_rank"] = pcts(sorted(denseTop1Ranks), 0.50, 0.90)
	out.Percentiles["lexical_stale_rank"] = pcts(sorted(lexTop1Ranks), 0.50, 0.90)
	out.Percentiles["answer_char_offset_in_gold_turn"] = pcts(sorted(answerOffsets), 0.50, 0.90, 1.0)

	record("d_stale_current_p50", percentile(dStaleCurrent, 0.50))
	record("d_current_nearest_nongold_p50", percentile(dCurrentNonGold, 0.50))
	record("dense_top1_rate", float64(denseTop1)/n)
	record("lexical_top1_rate", float64(lexTop1)/n)
	record("gold_pairs_within_eps_0.3", float64(pairWithinEps)/n)
	record("false_neighbour_within_0.3", float64(falseNeighbour)/n)
	record("flat_cosine_prefers_current", float64(prefersCurrent)/n)

	fmt.Printf("\nTHE ANTI-CORRELATION CLAIM: d(stale gold, current gold) p50 = %.4f versus "+
		"d(current gold, nearest NON-gold turn) p50 = %.4f. A superseded fact sits %s its "+
		"replacement than an unrelated turn in the same conversation does.\n",
		percentile(dStaleCurrent, 0.50), percentile(dCurrentNonGold, 0.50),
		map[bool]string{true: "FURTHER from", false: "CLOSER to"}[percentile(dStaleCurrent, 0.50) > percentile(dCurrentNonGold, 0.50)])

	// --- KU-strict, the rule the registration demoted for being underspecified ---
	strictIDs, numericN, lexicalN := kuStrict(ku)
	record("ku_strict_n", float64(len(strictIDs)))
	fmt.Printf("  ku_strict split: %d numeric + %d lexical (recorded: 32 + 17)\n", numericN, lexicalN)
	if err := os.WriteFile(filepath.Join(outDir, "ku_strict_ids.txt"),
		[]byte(strings.Join(strictIDs, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	out.Notes = append(out.Notes, "KU-strict is EXPLORATORY and its rule is underspecified in the "+
		"design pass ('numeric-token containment ... else answer-tokens-minus-question-tokens overlap "+
		"strictly greater and >= 0.50'). The reading implemented here is documented in kuStrict(). "+
		"The PRIMARY set is KU-permissive n=70 and does not change whatever this number is (1.2).")

	// --- embedder throughput and the real wordpiece truncation rate ---
	fmt.Printf("\n=== EMBEDDER THROUGHPUT (300 real turns, batch size 1, CPU) ===\n")
	sample := sampleTurns(all, 300)
	for _, seq := range []int{128, 256} {
		tp, err := measureThroughput(lib, model, vocab, seq, dim, sample, all)
		if err != nil {
			return err
		}
		out.Throughput = append(out.Throughput, tp)
		fmt.Printf("  maxSeqLen %3d: %.1f texts/s -> %.1f min for 10,960 turns; %.1f%% of all turns truncated\n",
			tp.MaxSeqLen, tp.TextsPerS, tp.Projected, 100*tp.TruncFrac)
		if seq == 256 {
			record("truncated_at_256_frac", tp.TruncFrac)
		}
	}
	out.Notes = append(out.Notes, "Truncation is measured with the model's own WordPiece tokenizer "+
		"(llm.CountTokens), not the design pass's character-count proxy. Truncation is uniform across "+
		"arms -- a validity limit, not a confound, because every arm reads the same vectors (4.9).")

	if err := cache.Save(cachePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: embedding cache not saved: %v\n", err)
	}

	jsonPath := filepath.Join(outDir, "probe.json")
	blob, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, append(blob, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("\nwrote %s\n      %s\n      %s\n", jsonPath,
		filepath.Join(outDir, "gold_map.csv"), filepath.Join(outDir, "ku_strict_ids.txt"))
	fmt.Printf("\nREGISTERED RULE (4.10): where measured and recorded differ, the MEASURED value is\n" +
		"authoritative and the recorded one is struck. A struck number is a successful outcome of\n" +
		"this rule, not a failure of the experiment.\n")
	return nil
}

// rankOfStale returns the 1-based rank of the closest STALE gold turn among all
// turns other than cur, ordered by ascending dist. 0 if there is no stale gold.
func rankOfStale(turns []eval.Turn, cur eval.Turn, gold eval.Gold, dist func(a, b eval.Turn) float64) int {
	type scored struct {
		t eval.Turn
		d float64
	}
	var xs []scored
	for _, t := range turns {
		if t.ID == cur.ID {
			continue
		}
		xs = append(xs, scored{t, dist(cur, t)})
	}
	sort.SliceStable(xs, func(i, j int) bool { return xs[i].d < xs[j].d })
	for i, x := range xs {
		if gold.Stale[x.t.ID] {
			return i + 1
		}
	}
	return 0
}

// answerOffset returns the character offset of the first answer token inside
// the gold turn, or -1 if no answer token appears in it.
func answerOffset(in *eval.LMEInstance, content string) int {
	var answer string
	if err := json.Unmarshal(in.Answer, &answer); err != nil {
		answer = strings.Trim(string(in.Answer), `"`)
	}
	lower := strings.ToLower(content)
	best := -1
	for _, tok := range strings.Fields(strings.ToLower(answer)) {
		tok = strings.Trim(tok, ".,;:!?()'\"")
		if len(tok) < 3 {
			continue
		}
		if i := strings.Index(lower, tok); i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	return best
}

// kuStrict implements the design pass's strict answer-content audit. Its rule
// was never committed as code or as an ID list, and has several unfrozen
// sub-choices; this is one reading, stated explicitly so the number it produces
// is at least reproducible:
//
//	numeric clause -- the answer's digit tokens all appear in the NEWER answer
//	  session's gold turns and not all of them appear in the older's;
//	lexical clause -- otherwise, with A = answer tokens minus question tokens,
//	  overlap(newer) > overlap(older) and overlap(newer) >= 0.50.
//
// PREREGISTRATION.md 1.2 demoted KU-strict to EXPLORATORY precisely because of
// this ambiguity. Whatever number comes out, the primary set does not change.
func kuStrict(ku []*eval.LMEInstance) (ids []string, numeric, lexical int) {
	for _, in := range ku {
		turns, err := in.Turns()
		if err != nil {
			continue
		}
		gold := eval.GoldOf(turns)
		newerToks, olderToks := map[string]bool{}, map[string]bool{}
		for _, t := range turns {
			switch {
			case gold.Current[t.ID]:
				addTokens(newerToks, t.Content)
			case gold.Stale[t.ID]:
				addTokens(olderToks, t.Content)
			}
		}
		var answer string
		if err := json.Unmarshal(in.Answer, &answer); err != nil {
			answer = strings.Trim(string(in.Answer), `"`)
		}
		ansToks := map[string]bool{}
		addTokens(ansToks, answer)
		qToks := map[string]bool{}
		addTokens(qToks, in.Question)

		var nums []string
		for tok := range ansToks {
			if isNumeric(tok) {
				nums = append(nums, tok)
			}
		}
		if len(nums) > 0 {
			inNewer, inOlder := true, true
			for _, tok := range nums {
				inNewer = inNewer && newerToks[tok]
				inOlder = inOlder && olderToks[tok]
			}
			if inNewer && !inOlder {
				ids = append(ids, in.QuestionID)
				numeric++
				continue
			}
		}
		distinctive := map[string]bool{}
		for tok := range ansToks {
			if !qToks[tok] {
				distinctive[tok] = true
			}
		}
		if len(distinctive) == 0 {
			continue
		}
		ovNewer := overlap(distinctive, newerToks)
		ovOlder := overlap(distinctive, olderToks)
		if ovNewer > ovOlder && ovNewer >= 0.50 {
			ids = append(ids, in.QuestionID)
			lexical++
		}
	}
	sort.Strings(ids)
	return ids, numeric, lexical
}

func addTokens(dst map[string]bool, text string) {
	for _, tok := range eval.TokenizeSimple(text) {
		dst[tok] = true
	}
}

func isNumeric(tok string) bool {
	for _, r := range tok {
		if r < '0' || r > '9' {
			return false
		}
	}
	return tok != ""
}

func overlap(want, have map[string]bool) float64 {
	if len(want) == 0 {
		return 0
	}
	hit := 0
	for tok := range want {
		if have[tok] {
			hit++
		}
	}
	return float64(hit) / float64(len(want))
}

// --- throughput ---

func sampleTurns(all []*eval.LMEInstance, n int) []string {
	var out []string
	for _, in := range all {
		turns, err := in.Turns()
		if err != nil {
			continue
		}
		for _, t := range turns {
			out = append(out, t.Content)
			if len(out) == n {
				return out
			}
		}
	}
	return out
}

func measureThroughput(lib, model, vocab string, seq, dim int, sample []string, all []*eval.LMEInstance) (throughput, error) {
	p, err := llm.NewLocalEmbedProvider(lib, model, vocab, seq, dim)
	if err != nil {
		return throughput{}, err
	}
	defer p.Close()
	ctx := context.Background()
	start := time.Now()
	for _, text := range sample {
		if _, err := p.Embed(ctx, text); err != nil { // batch size 1
			return throughput{}, err
		}
	}
	elapsed := time.Since(start).Seconds()

	// The real WordPiece truncation rate, not a character proxy.
	total, truncated := 0, 0
	for _, in := range all {
		turns, err := in.Turns()
		if err != nil {
			continue
		}
		for _, t := range turns {
			total++
			if p.CountTokens(t.Content) > seq {
				truncated++
			}
		}
	}
	rate := float64(len(sample)) / elapsed
	return throughput{
		MaxSeqLen: seq, Texts: len(sample), Seconds: elapsed, TextsPerS: rate,
		Projected: 10960 / rate / 60, TruncFrac: float64(truncated) / float64(total),
	}, nil
}

// --- small helpers ---

func sorted(xs []float64) []float64 {
	out := append([]float64(nil), xs...)
	sort.Float64s(out)
	return out
}

// percentile uses nearest-rank on an already-sorted slice.
func percentile(sortedXs []float64, q float64) float64 {
	if len(sortedXs) == 0 {
		return math.NaN()
	}
	i := int(math.Ceil(q*float64(len(sortedXs)))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(sortedXs) {
		i = len(sortedXs) - 1
	}
	return sortedXs[i]
}

func pcts(sortedXs []float64, qs ...float64) []pct {
	out := make([]pct, 0, len(qs))
	for _, q := range qs {
		out = append(out, pct{Q: q, V: percentile(sortedXs, q)})
	}
	return out
}

// moduleRoot walks up from the working directory to the directory holding
// go.mod, so the probe writes to cma/eval/out/ no matter where it is run from.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %s", dir)
		}
		dir = parent
	}
}
