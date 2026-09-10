package eval

// The detectors and arm arithmetic for the forgetting experiment.
//
// Every rule here is fixed by cma/eval/PREREGISTRATION.md, cited by section.
// Nothing in this file has a free knob: thresholds arrive as arguments from a
// registered grid, the total order is 4.4's, and the tie-breaks are named.
//
// This file is deliberately free of Qdrant and of the embedder. It is pure
// arithmetic over frozen turns and frozen vectors, so forget_test.go can pin
// all of it without a container and cmd/probe can re-derive the section-0
// numbers from the same code the harness runs.

import (
	"math"
	"math/rand"
	"sort"

	"github.com/memora/cma/internal/consolidation"
	"github.com/memora/cma/internal/models"
)

// Pair is one detector decision: at anchor turn t, the nearest strictly-older
// turn was u*, at this score. Archiving u* is the action; keeping the anchor is
// what makes pair-catch measurable separately from archive recall.
type Pair struct {
	Anchor string  // turn t
	Target string  // u*, the turn archived
	Score  float64 // simIDF for D2 (higher = closer), cosine distance for D1 (lower = closer)
}

// ArchiveSet collapses detector pairs to the set of turn IDs to archive. A turn
// selected by several anchors appears once: A is a set (4.5, "no cascade").
func ArchiveSet(pairs []Pair) map[string]bool {
	out := make(map[string]bool, len(pairs))
	for _, p := range pairs {
		out[p.Target] = true
	}
	return out
}

// --- D2, the PRIMARY detector: IDF-weighted Jaccard nearest-older (4.5) ---

// idfJaccard is simIDF from 4.5:
//
//	simIDF(a,b) = sum of idf over the token-set INTERSECTION
//	            / sum of idf over the token-set UNION
//
// Token SETS, not multisets. idf is bm25.go's, with its +1 smoothing, which
// keeps every value non-negative so the ratio is well defined.
func idfJaccard(a, b map[string]bool, idf func(string) float64) float64 {
	var inter, union float64
	for tok := range a {
		w := idf(tok)
		union += w
		if b[tok] {
			inter += w
		}
	}
	for tok := range b {
		if !a[tok] {
			union += idf(tok)
		}
	}
	if union == 0 {
		return 0
	}
	return inter / union
}

func tokenSet(text string) map[string]bool {
	out := map[string]bool{}
	for _, tok := range tokenizeSimple(text) {
		out[tok] = true
	}
	return out
}

// TokenizeSimple exposes bm25.go's tokenizer -- lowercase, split on any
// non-letter non-digit rune -- because 4.5 names it as D2's tokenizer and
// cmd/probe must use the same one, not a lookalike.
func TokenizeSimple(text string) []string { return tokenizeSimple(text) }

// NewIDFSimilarity builds simIDF over one instance's turns and returns it as a
// scorer. The BM25 index and the token sets are built ONCE here rather than per
// pair, and both the D2 detector and cmd/probe call this, so the probe cannot
// drift into measuring a slightly different similarity than the arm uses.
//
// The IDF corpus is exactly the turns passed in: PER-INSTANCE, per 4.7. The
// primary arm's memory is per-instance (user_id = question_id), so the detector
// must see exactly what a per-user deployment would see; a pooled IDF would be
// information from other users' memories that no such system has. Registered
// limitation: idf over ~24 documents is statistically thin, so the
// discrimination is weak, and that is a validity limit of D2 stated up front.
func NewIDFSimilarity(turns []Turn) func(a, b Turn) float64 {
	docs := make([]string, len(turns))
	sets := make(map[string]map[string]bool, len(turns))
	for i, t := range turns {
		docs[i] = t.Content
		sets[t.ID] = tokenSet(t.Content)
	}
	idf := NewBM25(docs).idf
	return func(a, b Turn) float64 { return idfJaccard(sets[a.ID], sets[b.ID], idf) }
}

// NearestOlderIDF is arm D2. For each turn t, u* is the strictly-older turn
// with the highest simIDF; u* is archived iff simIDF(t,u*) >= theta.
//
// Registered details, none of them adjustable here:
//   - IDF corpus is PER-INSTANCE (4.7); see NewIDFSimilarity.
//   - Sign convention (4.5): D2 thresholds a SIMILARITY with >=.
//   - Tie-break: the u earliest in the 4.4 order. turns must arrive in that
//     order (Turns returns it), and the strict > below keeps the first seen.
//   - Same-session neighbours ARE allowed and there is no role filter. Both
//     would be corpus-specific tuning that would not transfer, and the
//     same-session/cross-session split of A is reported as a diagnostic instead.
//   - One pass, no cascade: all scores are against the full unarchived corpus,
//     so the result does not depend on evaluation order.
func NearestOlderIDF(turns []Turn, theta float64) []Pair {
	if len(turns) < 2 {
		return nil
	}
	sim := NewIDFSimilarity(turns)
	var pairs []Pair
	for i := 1; i < len(turns); i++ { // i == 0 has no strictly-older turn
		best, bestScore := -1, math.Inf(-1)
		for j := 0; j < i; j++ {
			if s := sim(turns[i], turns[j]); s > bestScore {
				best, bestScore = j, s
			}
		}
		if best >= 0 && bestScore >= theta {
			pairs = append(pairs, Pair{Anchor: turns[i].ID, Target: turns[best].ID, Score: bestScore})
		}
	}
	return pairs
}

// --- D1: dense nearest-older (4.5) ---

// NearestOlderDense is arm D1. Identical to D2 but over the frozen 384-d
// vectors, with d(t,u) = 1 - cos: u* = argmin d, archived iff d(t,u*) <= theta.
//
// Sign convention (4.5), stated so nobody flips it: D1 thresholds a DISTANCE
// with <=, D2 a similarity with >=.
func NearestOlderDense(turns []Turn, vecs map[string][]float32, theta float64) []Pair {
	if len(turns) < 2 {
		return nil
	}
	var pairs []Pair
	for i := 1; i < len(turns); i++ {
		best, bestDist := -1, math.Inf(1)
		for j := 0; j < i; j++ {
			if d := CosineDistance(vecs[turns[i].ID], vecs[turns[j].ID]); d < bestDist {
				best, bestDist = j, d
			}
		}
		if best >= 0 && bestDist <= theta {
			pairs = append(pairs, Pair{Anchor: turns[i].ID, Target: turns[best].ID, Score: bestDist})
		}
	}
	return pairs
}

// CosineDistance is 1 - cosine similarity, matching
// consolidation.cosineDistance's conventions exactly (including returning 1.0
// for a length mismatch or a zero vector) so D1 and D0 measure the same space.
func CosineDistance(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 1.0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 1.0
	}
	return 1.0 - dot/(math.Sqrt(na)*math.Sqrt(nb))
}

// --- D0: the shipped consolidator, at the shipped config (4.6) ---

// ShippedConsolidatorArchive is arm D0: run consolidation.NewDBSCAN at the
// SHIPPED dbscan_epsilon 0.3 / dbscan_min_points 3 (configs/config.yaml:52-53),
// keep clusters with at least 2 members, and archive all but the newest member
// under the 4.4 order.
//
// This arm tests the configuration the repo actually ships, which is why the
// epsilon and minPts arrive from the caller rather than being tuned here.
//
// Two guards that are not optional:
//   - len(turns) >= 2, because clustering.go:107-109 returns every noise point
//     as its own singleton cluster, so a 1-turn instance yields a "cluster".
//   - epsilon is NOT to be raised toward 1.0: computeCentroid sizes its
//     accumulator from episodes[0] (clustering.go:166) then indexes it with a
//     longer episode's range. Verified index-out-of-range at eps 1.0 / minPts 1.
func ShippedConsolidatorArchive(turns []Turn, vecs map[string][]float32, epsilon float64, minPoints int) map[string]bool {
	out := map[string]bool{}
	if len(turns) < 2 {
		return out
	}
	pos := make(map[string]int, len(turns)) // 4.4 rank, for "the newest member"
	eps := make([]models.Episode, 0, len(turns))
	for i, t := range turns {
		pos[t.ID] = i
		eps = append(eps, models.Episode{
			ID: t.ID, UserID: t.Instance.QuestionID, Content: t.Content,
			Embedding: vecs[t.ID], Timestamp: t.Timestamp(),
			MemoryType: models.MemoryEpisodic, ConsolidationStatus: models.StatusPending,
			// Explicit, never the zero value: an Episode built directly bypasses
			// models.NewEpisode, whose line 64 is the only writer of
			// DecayFactor = 1.0. Left at 0.0, dig.go:104's `score *= DecayFactor`
			// annihilates every score. See PF0/PF2 in 4.11.
			DecayFactor: 1.0, ImportanceScore: 0.0,
		})
	}
	for _, c := range consolidation.NewDBSCAN(epsilon, minPoints).Cluster(eps) {
		if len(c.Episodes) < 2 {
			continue // singleton, i.e. a noise point: nothing is superseded
		}
		newest := c.Episodes[0].ID
		for _, e := range c.Episodes[1:] {
			if pos[e.ID] > pos[newest] {
				newest = e.ID
			}
		}
		for _, e := range c.Episodes {
			if e.ID != newest {
				out[e.ID] = true
			}
		}
	}
	return out
}

// --- ORACLE, ANTI-ORACLE, FLOOR: the three harness self-checks (2) ---

// OracleArchive archives exactly the labelled stale-gold turns. SR@1 = 1.0 here
// is ARITHMETIC, never a result (V2 voids the run if it is not 1.0).
func OracleArchive(g Gold) map[string]bool { return copySet(g.Stale) }

// AntiOracleArchive archives exactly the labelled CURRENT-gold turns, so the
// first surviving gold must be a stale one and SR@1 must be 0.0 exactly (V1).
//
// It is the ONLY check that the harness reads the gold map in the right
// orientation: ORACLE reaches 1.0 identically if the stale and current columns
// were swapped, so ORACLE alone proves nothing about orientation (2.2).
func AntiOracleArchive(g Gold) map[string]bool { return copySet(g.Current) }

// FloorArchive archives every turn not in the newest session -- the degenerate
// exploit that reaches SR@1 = 1.0 by destroying memory. Its value is as check
// V7: if FLOOR is not exactly 1.0, archiving does not actually remove points
// from retrieval and the mechanism under test is inert.
func FloorArchive(turns []Turn) map[string]bool {
	out := map[string]bool{}
	if len(turns) == 0 {
		return out
	}
	newest := turns[len(turns)-1].SessionIdx // turns are in 4.4 order
	for _, t := range turns {
		if t.SessionIdx != newest {
			out[t.ID] = true
		}
	}
	return out
}

func copySet(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		if v {
			out[k] = true
		}
	}
	return out
}

// --- RANDOM-ARCHIVE: the mandatory rate-matched control (2.1) ---

// RandomArchive draws n turns uniformly without replacement from the ELIGIBLE
// set: every turn except the last in the 4.4 order, i.e. exactly the turns that
// have at least one strictly-later turn in the same instance.
//
// Matching the eligibility set is the conservative choice and is registered as
// such: D1 and D2 structurally CANNOT archive the newest turn, so sampling from
// all turns would let RANDOM archive the current gold at a rate the detector
// cannot, biasing the control downward and flattering D2.
//
// The caller supplies n = |A_D2*(instance)|, so the rate match is per instance
// and exact -- it matches not merely the global rate but its distribution
// across instances. Callers iterate instances in ascending question_id byte
// order with one rng, per 4.8.
//
// Registered expectation, so a null is not misread later (3.1): in the primary
// row every unarchived turn survives into R_X, so SR@1 cannot see a non-gold
// archive and RANDOM's expected delta is about zero. A near-null RANDOM is a
// PASS. Its job is to be the thing D2 must beat.
func RandomArchive(turns []Turn, n int, rng *rand.Rand) map[string]bool {
	out := map[string]bool{}
	if len(turns) < 2 || n <= 0 {
		return out
	}
	eligible := make([]string, 0, len(turns)-1)
	for _, t := range turns[:len(turns)-1] {
		eligible = append(eligible, t.ID)
	}
	rng.Shuffle(len(eligible), func(i, j int) { eligible[i], eligible[j] = eligible[j], eligible[i] })
	if n > len(eligible) {
		n = len(eligible)
	}
	for _, id := range eligible[:n] {
		out[id] = true
	}
	return out
}

// --- D3: the global recency prior, the cheap rival (2.3) ---

// Ranked is one retrieved point: its turn ID and its raw Qdrant cosine score.
type Ranked struct {
	ID    string
	Score float64
}

// Filter applies an arm's archive set to a ranked list and truncates to k. This
// is the offline set-filter R_X = [r in R : r not in A_X][:k] that every arm
// shares, licensed by pre-flight assert PF4 checking it against the real
// Search + MustNot path.
func Filter(ranked []Ranked, archived map[string]bool, k int) []Ranked {
	out := make([]Ranked, 0, len(ranked))
	for _, r := range ranked {
		if archived[r.ID] {
			continue
		}
		out = append(out, r)
		if len(out) == k {
			break
		}
	}
	return out
}

// RecencyRescore is arm D3: rescore the ranked list as
// cos + w*exp(-deltaDays/tau), delta measured from question_date to the turn's
// timestamp, then re-sort descending. It archives nothing.
//
// D3 is deliberately allowed to select its own best cell on the outcome metric.
// It is the rival, and handicapping ourselves against it is the point. Note it
// is not a novel rival either: force_recent_turns: 3 (config.yaml:42) is
// already a recency prior in production.
//
// Ties are broken by the incoming order, which is the cosine order, via a
// stable sort -- dig.go:75 uses the unstable sort.Slice, and that instability
// is precisely what makes its "order preserved" check a coin flip (4.11).
func RecencyRescore(ranked []Ranked, ageDays map[string]float64, w, tau float64) []Ranked {
	out := make([]Ranked, len(ranked))
	copy(out, ranked)
	for i := range out {
		out[i].Score += w * math.Exp(-ageDays[out[i].ID]/tau)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// --- The metric (PREREGISTRATION.md 3) ---

// SR@1 outcomes. Undefined is the "no gold survives" case, which is EXCLUDED
// from the mean, counted, and reported -- a mean over an empty set would report
// 0.0 for the wrong reason.
const (
	SRUndefined = -1 // no gold survives in R_X
	SRStale     = 0  // the first surviving gold is a STALE one
	SRCurrent   = 1  // the first surviving gold is a CURRENT one
)

// SRAt1 is the primary metric, exactly as registered:
//
//	first_gold(q,X) = the first r in R_X with r in S_q union C_q
//	SR@1 = 1 if first_gold is in C_q, 0 if in S_q, undefined if none survives
//
// Comparison of point UUIDs against a frozen gold map. Integer set membership.
// It cannot be inflated by unit length, chunk size, or candidate count, because
// every arm shares one index, one embedding pass, one query-vector set and one
// ranked list.
func SRAt1(filtered []Ranked, g Gold) int {
	for _, r := range filtered {
		if g.Current[r.ID] {
			return SRCurrent
		}
		if g.Stale[r.ID] {
			return SRStale
		}
	}
	return SRUndefined
}

// RecallAt reports whether any gold turn appears in the first k results. This
// is the control set's falsifier metric, the same arithmetic as
// hard_eval_test.go's recall curve.
func RecallAt(filtered []Ranked, gold map[string]bool, k int) bool {
	for i, r := range filtered {
		if i >= k {
			break
		}
		if gold[r.ID] {
			return true
		}
	}
	return false
}

// ReciprocalRank is 1/rank of the first gold turn, or 0 if none is present.
func ReciprocalRank(filtered []Ranked, gold map[string]bool) float64 {
	for i, r := range filtered {
		if gold[r.ID] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// --- Statistics (PREREGISTRATION.md 4.14) ---

// McNemarExact is the exact two-sided McNemar test via the binomial tail, on
// the discordant counts b and c. This is used for ONE confirmatory test -- D2*
// vs RAW on SR@1 over KU-permissive n = 70 -- and no correction is applied
// because there is one test. Every other use is EXPLORATORY and descriptive,
// and callers must print b and c raw beside any p-value.
//
// Returns 1.0 when there are no discordant pairs, which is the correct
// "no evidence of a difference" answer rather than a divide-by-zero.
func McNemarExact(b, c int) float64 {
	n := b + c
	if n == 0 {
		return 1.0
	}
	lo := b
	if c < lo {
		lo = c
	}
	// P(X <= lo) for X ~ Binomial(n, 0.5), computed in log space so that
	// n in the low hundreds does not overflow the factorials.
	var tail float64
	for k := 0; k <= lo; k++ {
		tail += math.Exp(logChoose(n, k) - float64(n)*math.Ln2)
	}
	if p := 2 * tail; p < 1.0 {
		return p
	}
	return 1.0
}

func logChoose(n, k int) float64 {
	lg, _ := math.Lgamma(float64(n + 1))
	lk, _ := math.Lgamma(float64(k + 1))
	lnk, _ := math.Lgamma(float64(n - k + 1))
	return lg - lk - lnk
}

// WilsonHalfWidth is the normal-approximation half-width used for the
// registered +/-10pp equivalence bound: 1.96*sqrt(b+c)/n.
//
// The bound is arithmetic, not taste. At n = 70 it needs b+c <= 12 discordant
// pairs, which is achievable; a +/-3pp bound would need b+c <= 1, which is
// unreachable and would silently degrade to the bare "p > 0.05" this project
// forbids.
func WilsonHalfWidth(b, c, n int) float64 {
	if n == 0 {
		return math.Inf(1)
	}
	return 1.96 * math.Sqrt(float64(b+c)) / float64(n)
}

// Discordant counts the McNemar cells between two arms' per-query outcomes.
// Queries where either arm is undefined are skipped and returned as `skipped`:
// SR@1 is undefined there, and silently scoring an undefined query as 0 is the
// exact failure mode the undefined state exists to prevent.
func Discordant(a, b []int) (bOnly, cOnly, skipped int) {
	for i := range a {
		if i >= len(b) {
			break
		}
		if a[i] == SRUndefined || b[i] == SRUndefined {
			skipped++
			continue
		}
		switch {
		case a[i] == SRCurrent && b[i] == SRStale:
			bOnly++
		case a[i] == SRStale && b[i] == SRCurrent:
			cOnly++
		}
	}
	return bOnly, cOnly, skipped
}

// MeanDefined is SR@1 over the queries where it is defined, plus the counts.
// The undefined count is returned rather than folded away because the
// evidence-found rate is itself a registered gate (V3: below 0.95 on the
// primary cell, the index or the query set is broken).
func MeanDefined(outcomes []int) (mean float64, defined, undefined int) {
	var hits int
	for _, o := range outcomes {
		switch o {
		case SRUndefined:
			undefined++
		case SRCurrent:
			hits++
			defined++
		default:
			defined++
		}
	}
	if defined == 0 {
		return 0, 0, undefined
	}
	return float64(hits) / float64(defined), defined, undefined
}
