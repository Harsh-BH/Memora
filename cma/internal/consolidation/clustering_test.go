package consolidation

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/memora/cma/internal/models"
)

// Production configuration, from configs/config.yaml:
//
//	consolidation.dbscan_epsilon:    0.3
//	consolidation.dbscan_min_points: 3
//
// and the embedding dimension used by the OpenAI text-embedding-3-small
// provider wired in cmd/api/main.go.
const (
	prodEpsilon   = 0.3
	prodMinPoints = 3
	prodDim       = 1536
	fixedSeed     = 42
)

// ---------------------------------------------------------------------------
// deterministic synthetic-data helpers
// ---------------------------------------------------------------------------

func newRNG(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

func normalize(v []float32) []float32 {
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	n = math.Sqrt(n)
	if n == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / n)
	}
	return out
}

// basis returns the unit vector e_i in the given dimension.
func basis(dim, i int) []float32 {
	v := make([]float32, dim)
	v[i] = 1
	return v
}

// mix returns normalize(a + coef*b).
func mix(a []float32, coef float64, b []float32) []float32 {
	out := make([]float32, len(a))
	for i := range a {
		out[i] = a[i] + float32(coef)*b[i]
	}
	return normalize(out)
}

// jitter returns a unit vector near center. alpha is the euclidean norm of the
// gaussian perturbation added before renormalising, so it is dimension
// independent: alpha=0.2 means "a perturbation 20% the length of the center".
func jitter(rng *rand.Rand, center []float32, alpha float64) []float32 {
	dim := len(center)
	out := make([]float32, dim)
	scale := alpha / math.Sqrt(float64(dim))
	for i := range center {
		out[i] = center[i] + float32(scale*rng.NormFloat64())
	}
	return normalize(out)
}

func mkEpisode(id string, emb []float32) models.Episode {
	return models.Episode{
		ID:                  id,
		UserID:              "u1",
		Content:             id,
		Embedding:           emb,
		MemoryType:          models.MemoryEpisodic,
		ConsolidationStatus: models.StatusPending,
	}
}

// plantedOrthogonal builds k clusters whose centers are mutually orthogonal
// basis vectors (cosine distance 1.0 between centers -- maximally separated
// for non-negative embeddings). Returns the episodes plus the planted label
// of each episode.
func plantedOrthogonal(rng *rand.Rand, dim, k, perCluster int, alpha float64) ([]models.Episode, []int) {
	eps := make([]models.Episode, 0, k*perCluster)
	labels := make([]int, 0, k*perCluster)
	for c := 0; c < k; c++ {
		center := basis(dim, c)
		for p := 0; p < perCluster; p++ {
			eps = append(eps, mkEpisode(fmt.Sprintf("c%d-p%d", c, p), jitter(rng, center, alpha)))
			labels = append(labels, c)
		}
	}
	return eps, labels
}

// plantedTwoTier builds groups*perGroup centers. Centers inside a group sit at
// cosine distance 1 - 1/(1+delta^2) from each other; centers in different
// groups are orthogonal (distance 1.0). This gives the epsilon sweep two
// distinct merge thresholds to find.
func plantedTwoTier(rng *rand.Rand, dim, groups, perGroup, perCenter int, delta, alpha float64) ([]models.Episode, []int) {
	eps := make([]models.Episode, 0, groups*perGroup*perCenter)
	labels := make([]int, 0, groups*perGroup*perCenter)
	label := 0
	for g := 0; g < groups; g++ {
		for j := 0; j < perGroup; j++ {
			center := mix(basis(dim, g), delta, basis(dim, groups+g*perGroup+j))
			for p := 0; p < perCenter; p++ {
				eps = append(eps, mkEpisode(fmt.Sprintf("g%d-c%d-p%d", g, j, p), jitter(rng, center, alpha)))
				labels = append(labels, label)
			}
			label++
		}
	}
	return eps, labels
}

// ---------------------------------------------------------------------------
// result inspection helpers
//
// NOTE: Cluster() returns noise points as SINGLETON clusters with a negative
// ID (clustering.go:108), while genuine clusters get positive IDs starting at
// 1. len(result) is therefore NOT the number of clusters found.
// ---------------------------------------------------------------------------

func splitResult(cs []models.Cluster) (real, noise []models.Cluster) {
	for _, c := range cs {
		if c.ID > 0 {
			real = append(real, c)
		} else {
			noise = append(noise, c)
		}
	}
	sort.Slice(real, func(i, j int) bool { return real[i].ID < real[j].ID })
	sort.Slice(noise, func(i, j int) bool { return noise[i].ID > noise[j].ID })
	return real, noise
}

// canonical renders a clustering as an order-independent fingerprint, so that
// assertions never depend on Go's randomised map iteration order.
func canonical(cs []models.Cluster) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		ids := make([]string, 0, len(c.Episodes))
		for _, e := range c.Episodes {
			ids = append(ids, e.ID)
		}
		sort.Strings(ids)
		out = append(out, strings.Join(ids, ","))
	}
	sort.Strings(out)
	return out
}

// distanceStats measures the actual separation of a planted dataset under the
// production distance function, so the test reports the real geometry instead
// of the geometry we intended.
func distanceStats(eps []models.Episode, labels []int) (maxIntra, minInter float64) {
	maxIntra = 0
	minInter = math.Inf(1)
	for i := range eps {
		for j := i + 1; j < len(eps); j++ {
			d := cosineDistance(eps[i].Embedding, eps[j].Embedding)
			if labels[i] == labels[j] {
				if d > maxIntra {
					maxIntra = d
				}
			} else if d < minInter {
				minInter = d
			}
		}
	}
	return maxIntra, minInter
}

// assertRecovery checks that the positive-ID clusters are exactly the planted
// partition, and returns the number of real clusters and noise singletons.
func assertRecovery(t *testing.T, eps []models.Episode, labels []int, got []models.Cluster, wantK int) (int, int) {
	t.Helper()

	byLabel := make(map[int][]string)
	for i, e := range eps {
		byLabel[labels[i]] = append(byLabel[labels[i]], e.ID)
	}
	want := make([]string, 0, len(byLabel))
	for _, ids := range byLabel {
		sort.Strings(ids)
		want = append(want, strings.Join(ids, ","))
	}
	sort.Strings(want)

	real, noise := splitResult(got)
	if len(real) != wantK {
		t.Errorf("cluster count: got %d real clusters, want %d (plus %d noise singletons)",
			len(real), wantK, len(noise))
	}
	if len(noise) != 0 {
		ids := make([]string, 0, len(noise))
		for _, c := range noise {
			ids = append(ids, c.Episodes[0].ID)
		}
		t.Errorf("expected no noise, got %d noise singletons: %v", len(noise), ids)
	}
	gotCanon := canonical(real)
	if strings.Join(gotCanon, " | ") != strings.Join(want, " | ") {
		t.Errorf("membership mismatch:\n got %v\nwant %v", gotCanon, want)
	}
	return len(real), len(noise)
}

// ---------------------------------------------------------------------------
// (a) correctness on planted structure
// ---------------------------------------------------------------------------

func TestClusterRecoversPlantedStructure(t *testing.T) {
	cases := []struct {
		name       string
		dim        int
		k          int
		perCluster int
		alpha      float64
	}{
		{"reduced-dim-16", 16, 4, 8, 0.35},
		{"reduced-dim-64", 64, 5, 10, 0.35},
		{"production-dim-1536", prodDim, 4, 8, 0.35},
		{"production-dim-1536-tight", prodDim, 6, 5, 0.15},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rng := newRNG(fixedSeed)
			eps, labels := plantedOrthogonal(rng, tc.dim, tc.k, tc.perCluster, tc.alpha)

			maxIntra, minInter := distanceStats(eps, labels)

			d := NewDBSCAN(prodEpsilon, prodMinPoints)
			got := d.Cluster(eps)
			nReal, nNoise := assertRecovery(t, eps, labels, got, tc.k)

			sizes := make([]int, 0, nReal)
			real, _ := splitResult(got)
			for _, c := range real {
				sizes = append(sizes, len(c.Episodes))
			}
			t.Logf("dim=%d n=%d planted_k=%d jitter_alpha=%.2f | max_intra_dist=%.4f min_inter_dist=%.4f eps=%.2f minPts=%d | clusters=%d sizes=%v noise=%d returned_len=%d",
				tc.dim, len(eps), tc.k, tc.alpha, maxIntra, minInter,
				prodEpsilon, prodMinPoints, nReal, sizes, nNoise, len(got))
		})
	}
}

// TestClusterEmptyAndSingleton pins the degenerate inputs.
func TestClusterEmptyAndSingleton(t *testing.T) {
	d := NewDBSCAN(prodEpsilon, prodMinPoints)

	if got := d.Cluster(nil); got != nil {
		t.Errorf("Cluster(nil) = %v, want nil", got)
	}
	if got := d.Cluster([]models.Episode{}); got != nil {
		t.Errorf("Cluster(empty) = %v, want nil", got)
	}

	rng := newRNG(fixedSeed)
	one := []models.Episode{mkEpisode("solo", jitter(rng, basis(prodDim, 0), 0.1))}
	got := d.Cluster(one)
	real, noise := splitResult(got)
	if len(real) != 0 || len(noise) != 1 {
		t.Errorf("single episode: got %d real / %d noise, want 0 real / 1 noise singleton", len(real), len(noise))
	}
	t.Logf("n=1 with minPts=3 -> real=%d noise_singletons=%d (one episode can never form a cluster)", len(real), len(noise))
}

// TestClusterIsDeterministic asserts the PARTITION is stable across runs, and
// measures how often the returned SLICE ORDER varies (it is built from a Go
// map, clustering.go:104-122, so ordering is randomised by the runtime).
func TestClusterIsDeterministic(t *testing.T) {
	rng := newRNG(fixedSeed)
	eps, _ := plantedOrthogonal(rng, 64, 5, 6, 0.3)
	d := NewDBSCAN(prodEpsilon, prodMinPoints)

	const runs = 200
	base := canonical(d.Cluster(eps))
	orderings := make(map[string]int)
	for i := 0; i < runs; i++ {
		got := d.Cluster(eps)
		if strings.Join(canonical(got), " | ") != strings.Join(base, " | ") {
			t.Fatalf("run %d produced a different partition:\n got %v\nwant %v", i, canonical(got), base)
		}
		ids := make([]string, 0, len(got))
		for _, c := range got {
			ids = append(ids, fmt.Sprint(c.ID))
		}
		orderings[strings.Join(ids, ",")]++
	}
	t.Logf("partition stable over %d runs; distinct returned-slice orderings observed = %d (%d clusters returned)",
		runs, len(orderings), len(base))
	if len(orderings) > 1 {
		t.Logf("DEFECT (non-fatal): Cluster() output ORDER is not deterministic -- clusters are collected from a map without sorting")
	}
}

// ---------------------------------------------------------------------------
// (b) noise handling
// ---------------------------------------------------------------------------

func TestNoiseIsNotAbsorbed(t *testing.T) {
	const (
		dim        = prodDim
		k          = 3
		perCluster = 6
		nOutliers  = 5
	)
	rng := newRNG(fixedSeed)
	eps, labels := plantedOrthogonal(rng, dim, k, perCluster, 0.3)

	// Outliers sit on basis vectors that no cluster occupies, so each is at
	// cosine distance ~1.0 from every cluster AND from every other outlier.
	outlierIDs := make(map[string]bool)
	for o := 0; o < nOutliers; o++ {
		id := fmt.Sprintf("noise-%d", o)
		eps = append(eps, mkEpisode(id, jitter(rng, basis(dim, k+o), 0.05)))
		labels = append(labels, 1000+o)
		outlierIDs[id] = true
	}

	d := NewDBSCAN(prodEpsilon, prodMinPoints)
	got := d.Cluster(eps)
	real, noise := splitResult(got)

	if len(real) != k {
		t.Errorf("got %d real clusters, want %d", len(real), k)
	}
	for _, c := range real {
		for _, e := range c.Episodes {
			if outlierIDs[e.ID] {
				t.Errorf("outlier %s was absorbed into cluster %d", e.ID, c.ID)
			}
		}
	}
	gotNoise := make([]string, 0, len(noise))
	for _, c := range noise {
		if len(c.Episodes) != 1 {
			t.Errorf("noise cluster %d has %d episodes, want singleton", c.ID, len(c.Episodes))
		}
		gotNoise = append(gotNoise, c.Episodes[0].ID)
	}
	sort.Strings(gotNoise)
	if len(gotNoise) != nOutliers {
		t.Errorf("got %d noise singletons %v, want %d", len(gotNoise), gotNoise, nOutliers)
	}
	for id := range outlierIDs {
		found := false
		for _, g := range gotNoise {
			if g == id {
				found = true
			}
		}
		if !found {
			t.Errorf("outlier %s was not reported as noise", id)
		}
	}

	// A near-duplicate pair is still noise: 2 < minPoints=3.
	pairRng := newRNG(fixedSeed + 1)
	pair := []models.Episode{
		mkEpisode("pair-a", jitter(pairRng, basis(dim, 0), 0.02)),
		mkEpisode("pair-b", jitter(pairRng, basis(dim, 0), 0.02)),
	}
	pairReal, pairNoise := splitResult(d.Cluster(pair))
	if len(pairReal) != 0 || len(pairNoise) != 2 {
		t.Errorf("2 near-identical points: got %d real / %d noise, want 0 / 2 (minPts=3)", len(pairReal), len(pairNoise))
	}

	t.Logf("n=%d (%d clustered + %d outliers) dim=%d | real_clusters=%d noise_singletons=%d noise_ids=%v | returned_len=%d",
		len(eps), k*perCluster, nOutliers, dim, len(real), len(noise), gotNoise, len(got))
	t.Logf("2 near-duplicate points at distance %.5f -> real=%d noise=%d (minPts=3 rejects a pair)",
		cosineDistance(pair[0].Embedding, pair[1].Embedding), len(pairReal), len(pairNoise))
}

// TestNoiseSingletonsInflateClusterCount measures the operational consequence
// of the "noise points become singleton clusters" design (clustering.go:107).
// worker.go:118-127 issues one LLM Synthesize + one ExtractTriples call per
// returned cluster, with no filter on cluster size.
func TestNoiseSingletonsInflateClusterCount(t *testing.T) {
	const (
		dim        = prodDim
		k          = 3
		perCluster = 6
		nOutliers  = 20
	)
	rng := newRNG(fixedSeed)
	eps, _ := plantedOrthogonal(rng, dim, k, perCluster, 0.3)
	for o := 0; o < nOutliers; o++ {
		eps = append(eps, mkEpisode(fmt.Sprintf("noise-%d", o), normalize(randVec(rng, dim))))
	}

	d := NewDBSCAN(prodEpsilon, prodMinPoints)
	got := d.Cluster(eps)
	real, noise := splitResult(got)

	if len(real) != k {
		t.Errorf("got %d real clusters, want %d", len(real), k)
	}
	if len(noise) != nOutliers {
		t.Errorf("got %d noise singletons, want %d", len(noise), nOutliers)
	}

	wastePct := 100 * float64(len(noise)) / float64(len(got))
	t.Logf("n=%d (%d in %d planted clusters + %d random outliers) | len(Cluster())=%d = %d real + %d singletons",
		len(eps), k*perCluster, k, nOutliers, len(got), len(real), len(noise))
	t.Logf("worker.go makes 2 LLM calls per returned cluster => %d calls, of which %d (%.1f%%) synthesize a 'gist' from a SINGLE episode",
		2*len(got), 2*len(noise), wastePct)
	t.Logf("metrics ClustersFormed would report %d, not %d", len(got), len(real))
}

func randVec(rng *rand.Rand, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = float32(rng.NormFloat64())
	}
	return v
}

// ---------------------------------------------------------------------------
// (c) epsilon / minPoints sensitivity
// ---------------------------------------------------------------------------

func TestEpsilonSensitivity(t *testing.T) {
	const (
		dim       = prodDim
		groups    = 2
		perGroup  = 3
		perCenter = 5
		delta     = 0.5 // -> intra-group center distance 1 - 1/(1+delta^2) = 0.20
		alpha     = 0.12
	)
	rng := newRNG(fixedSeed)
	eps, labels := plantedTwoTier(rng, dim, groups, perGroup, perCenter, delta, alpha)
	maxIntra, minInter := distanceStats(eps, labels)
	t.Logf("dataset: n=%d dim=%d, %d centers in %d groups; measured max intra-center dist=%.4f, min inter-center dist=%.4f",
		len(eps), dim, groups*perGroup, groups, maxIntra, minInter)
	t.Logf("by construction: centers within a group are 0.20 apart, centers across groups are 1.00 apart")
	t.Logf("")
	t.Logf("  epsilon | real clusters | noise pts | largest | returned len")
	t.Logf("  --------+---------------+-----------+---------+-------------")

	sweep := []float64{0.01, 0.02, 0.03, 0.05, 0.10, 0.15, 0.19, 0.21, 0.25, 0.30, 0.40, 0.60, 0.80, 0.95, 1.00, 1.05, 1.20}
	results := make(map[float64][2]int, len(sweep))
	for _, e := range sweep {
		got := NewDBSCAN(e, prodMinPoints).Cluster(eps)
		real, noise := splitResult(got)
		largest := 0
		for _, c := range real {
			if len(c.Episodes) > largest {
				largest = len(c.Episodes)
			}
		}
		marker := ""
		if e == prodEpsilon {
			marker = "   <-- production value"
		}
		t.Logf("  %7.2f | %13d | %9d | %7d | %12d%s", e, len(real), len(noise), largest, len(got), marker)
		results[e] = [2]int{len(real), len(noise)}
	}

	// Invariants the sweep must satisfy, otherwise the table is meaningless.
	if got := results[0.01][0]; got != 0 {
		t.Errorf("epsilon=0.01: got %d clusters, want 0 (every point isolated)", got)
	}
	if got := results[0.01][1]; got != len(eps) {
		t.Errorf("epsilon=0.01: got %d noise points, want %d (all of them)", got, len(eps))
	}
	if got := results[0.10][0]; got != groups*perGroup {
		t.Errorf("epsilon=0.10: got %d clusters, want %d (each center its own cluster)", got, groups*perGroup)
	}
	if got := results[prodEpsilon][0]; got != groups {
		t.Errorf("epsilon=%.2f (production): got %d clusters, want %d (centers merge within a group)", prodEpsilon, got, groups)
	}
	if got := results[1.20][0]; got != 1 {
		t.Errorf("epsilon=1.20: got %d clusters, want 1 (everything merges)", got)
	}

	// Cluster count must be monotonically non-increasing once past the
	// fragmentation regime -- more reach can only merge, never split.
	prev := -1
	for _, e := range sweep {
		if e < 0.05 {
			continue
		}
		c := results[e][0]
		if prev >= 0 && c > prev {
			t.Errorf("cluster count rose from %d to %d at epsilon=%.2f; merging must be monotone", prev, c, e)
		}
		prev = c
	}
	// Derive the stable plateau around the production value from the measured
	// table rather than asserting a hand-written range.
	prodCount := results[prodEpsilon][0]
	lo, hi := prodEpsilon, prodEpsilon
	for _, e := range sweep {
		if results[e][0] != prodCount {
			continue
		}
		contiguous := true
		for _, m := range sweep {
			if m > math.Min(e, prodEpsilon) && m < math.Max(e, prodEpsilon) && results[m][0] != prodCount {
				contiguous = false
			}
		}
		if !contiguous {
			continue
		}
		if e < lo {
			lo = e
		}
		if e > hi {
			hi = e
		}
	}
	t.Logf("")
	t.Logf("production epsilon=%.2f sits inside the measured [%.2f, %.2f] plateau where the answer is stably %d clusters:",
		prodEpsilon, lo, hi, prodCount)
	t.Logf("  a %.2f decrease or a %.2f increase would be needed to change the result",
		prodEpsilon-lo, hi-prodEpsilon)
}

func TestMinPointsSensitivity(t *testing.T) {
	const dim = prodDim
	rng := newRNG(fixedSeed)
	// Four clusters of deliberately different sizes: 2, 3, 5 and 9 points.
	var eps []models.Episode
	sizes := []int{2, 3, 5, 9}
	for c, sz := range sizes {
		center := basis(dim, c)
		for p := 0; p < sz; p++ {
			eps = append(eps, mkEpisode(fmt.Sprintf("c%d-p%d", c, p), jitter(rng, center, 0.2)))
		}
	}

	t.Logf("dataset: n=%d dim=%d, planted cluster sizes %v, epsilon fixed at %.2f", len(eps), dim, sizes, prodEpsilon)
	t.Logf("")
	t.Logf("  minPoints | real clusters | noise pts | surviving sizes")
	t.Logf("  ----------+---------------+-----------+----------------")
	survivors := make(map[int]int)
	for mp := 1; mp <= 10; mp++ {
		got := NewDBSCAN(prodEpsilon, mp).Cluster(eps)
		real, noise := splitResult(got)
		var ss []int
		for _, c := range real {
			ss = append(ss, len(c.Episodes))
		}
		sort.Ints(ss)
		marker := ""
		if mp == prodMinPoints {
			marker = "   <-- production value"
		}
		t.Logf("  %9d | %13d | %9d | %v%s", mp, len(real), len(noise), ss, marker)
		survivors[mp] = len(real)
	}

	// minPoints=k keeps exactly the planted clusters of size >= k.
	for mp := 1; mp <= 10; mp++ {
		want := 0
		for _, sz := range sizes {
			if sz >= mp {
				want++
			}
		}
		if survivors[mp] != want {
			t.Errorf("minPoints=%d: got %d clusters, want %d (planted sizes %v)", mp, survivors[mp], want, sizes)
		}
	}
	t.Logf("")
	t.Logf("minPoints acts as a hard floor on cluster size: a cluster of size s survives iff s >= minPoints")
	t.Logf("production minPoints=3 therefore discards every pair of near-duplicate episodes as noise")
}

// TestClusterTightnessBreakingPoint answers "how much intra-cluster spread can
// epsilon=0.3 tolerate before well-separated clusters stop being recovered".
func TestClusterTightnessBreakingPoint(t *testing.T) {
	const (
		dim        = prodDim
		k          = 4
		perCluster = 8
	)
	t.Logf("dim=%d, %d orthogonal planted clusters of %d points, eps=%.2f minPts=%d",
		dim, k, perCluster, prodEpsilon, prodMinPoints)
	t.Logf("")
	t.Logf("  jitter alpha | max intra dist | min inter dist | real clusters | noise | recovered")
	t.Logf("  -------------+----------------+----------------+---------------+-------+----------")

	d := NewDBSCAN(prodEpsilon, prodMinPoints)
	lastOK := 0.0
	firstBad := 0.0
	for _, alpha := range []float64{0.05, 0.10, 0.20, 0.30, 0.40, 0.50, 0.60, 0.70, 0.80, 0.90, 1.00} {
		rng := newRNG(fixedSeed)
		eps, labels := plantedOrthogonal(rng, dim, k, perCluster, alpha)
		maxIntra, minInter := distanceStats(eps, labels)
		real, noise := splitResult(d.Cluster(eps))

		ok := len(real) == k && len(noise) == 0
		if ok {
			// verify membership too
			byLabel := map[int][]string{}
			for i, e := range eps {
				byLabel[labels[i]] = append(byLabel[labels[i]], e.ID)
			}
			want := []string{}
			for _, ids := range byLabel {
				sort.Strings(ids)
				want = append(want, strings.Join(ids, ","))
			}
			sort.Strings(want)
			ok = strings.Join(canonical(real), " | ") == strings.Join(want, " | ")
		}
		if ok {
			lastOK = maxIntra
		} else if firstBad == 0 {
			firstBad = maxIntra
		}
		t.Logf("  %12.2f | %14.4f | %14.4f | %13d | %5d | %v",
			alpha, maxIntra, minInter, len(real), len(noise), ok)
	}
	if lastOK == 0 {
		t.Errorf("no jitter level recovered the planted structure; the sweep is not measuring anything")
	}
	t.Logf("")
	t.Logf("recovery holds up to a measured max intra-cluster cosine distance of %.4f (%.0f%% of epsilon);",
		lastOK, 100*lastOK/prodEpsilon)
	t.Logf("first failure at a max intra-cluster distance of %.4f (%.0f%% of epsilon)",
		firstBad, 100*firstBad/prodEpsilon)
	t.Logf("the breaking point tracks epsilon almost exactly: DBSCAN chaining buys essentially no extra slack here,")
	t.Logf("because once the widest pair inside a cluster exceeds epsilon the cluster shatters into noise rather than degrading gracefully")
}

// ---------------------------------------------------------------------------
// (d) complexity in practice -- measured, never asserted
// ---------------------------------------------------------------------------

func TestClusterScalingAtProductionDim(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing sweep in -short mode")
	}
	const reps = 3
	d := NewDBSCAN(prodEpsilon, prodMinPoints)

	t.Logf("wall clock for DBSCAN.Cluster at dim=%d, eps=%.2f, minPts=%d; best of %d runs",
		prodDim, prodEpsilon, prodMinPoints, reps)
	t.Logf("dataset: 5 orthogonal planted clusters, points split evenly, alpha=0.3")
	t.Logf("")
	t.Logf("      n |   best time | ratio vs prev | ns / (n^2 * d)")
	t.Logf("  ------+-------------+---------------+---------------")

	var prev time.Duration
	var norms []float64
	for _, n := range []int{50, 100, 200, 400} {
		rng := newRNG(fixedSeed)
		eps, _ := plantedOrthogonal(rng, prodDim, 5, n/5, 0.3)
		if len(eps) != n {
			t.Fatalf("dataset size %d != %d", len(eps), n)
		}
		best := time.Duration(math.MaxInt64)
		for r := 0; r < reps; r++ {
			start := time.Now()
			got := d.Cluster(eps)
			el := time.Since(start)
			if len(got) == 0 {
				t.Fatalf("n=%d produced no clusters", n)
			}
			if el < best {
				best = el
			}
		}
		ratio := "     -"
		if prev > 0 {
			ratio = fmt.Sprintf("%6.2fx", float64(best)/float64(prev))
		}
		norm := float64(best.Nanoseconds()) / (float64(n) * float64(n) * float64(prodDim))
		norms = append(norms, norm)
		t.Logf("  %5d | %11s | %13s | %14.4f", n, best.Round(time.Microsecond), ratio, norm)
		prev = best
	}

	minN, maxN := norms[0], norms[0]
	for _, v := range norms {
		if v < minN {
			minN = v
		}
		if v > maxN {
			maxN = v
		}
	}
	t.Logf("")
	t.Logf("normalised cost ns/(n^2*d) spans %.4f..%.4f over an 8x range of n (spread %.2fx)", minN, maxN, maxN/minN)
	t.Logf("a flat normalised cost is what O(n^2 * d) predicts: doubling n should cost ~4x")
	t.Logf("code basis: regionQuery (clustering.go:128) scans all n episodes and cosineDistance (clustering.go:140) walks all d components;")
	t.Logf("regionQuery runs once per point in the main loop plus once per point when it is expanded, so at most 2n scans -> Theta(n^2 * d)")
}

// ---------------------------------------------------------------------------
// (e) the distance function itself
// ---------------------------------------------------------------------------

func TestCosineDistanceIsGenuinelyCosine(t *testing.T) {
	const tol = 1e-6
	cases := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical", []float32{1, 2, 3}, []float32{1, 2, 3}, 0},
		{"orthogonal", []float32{1, 0}, []float32{0, 1}, 1},
		{"opposite", []float32{1, 0}, []float32{-1, 0}, 2},
		{"60deg", []float32{1, 0}, []float32{0.5, float32(math.Sqrt(3) / 2)}, 0.5},
		{"45deg", []float32{1, 0}, []float32{1, 1}, 1 - math.Sqrt2/2},
		{"scale-invariant-b", []float32{1, 2, 3}, []float32{10, 20, 30}, 0},
		{"scale-invariant-mixed", []float32{3, 0, 0}, []float32{0.001, 0, 0}, 0},
		{"antiparallel-scaled", []float32{2, 4}, []float32{-100, -200}, 2},
	}
	for _, tc := range cases {
		got := cosineDistance(tc.a, tc.b)
		if math.Abs(got-tc.want) > tol {
			t.Errorf("cosineDistance(%v, %v) = %.9f, want %.9f", tc.a, tc.b, got, tc.want)
		}
		t.Logf("%-22s -> %.9f (want %.9f)", tc.name, got, tc.want)
	}

	// Symmetry and range over pseudo-random production-dimension vectors.
	rng := newRNG(fixedSeed)
	minD, maxD := math.Inf(1), math.Inf(-1)
	for i := 0; i < 500; i++ {
		a := randVec(rng, 32)
		b := randVec(rng, 32)
		ab := cosineDistance(a, b)
		ba := cosineDistance(b, a)
		if math.Abs(ab-ba) > tol {
			t.Fatalf("asymmetric: d(a,b)=%v d(b,a)=%v", ab, ba)
		}
		if ab < -tol || ab > 2+tol {
			t.Fatalf("distance %v outside [0,2]", ab)
		}
		if ab < minD {
			minD = ab
		}
		if ab > maxD {
			maxD = ab
		}
		if math.IsNaN(ab) {
			t.Fatalf("NaN for a=%v b=%v", a, b)
		}
	}
	t.Logf("500 random 32-d pairs: symmetric to within %g, observed range [%.4f, %.4f] inside the theoretical [0, 2]", tol, minD, maxD)

	// Self-distance is exactly 0 for every non-zero vector.
	worst := 0.0
	for i := 0; i < 200; i++ {
		v := randVec(rng, 128)
		if d := math.Abs(cosineDistance(v, v)); d > worst {
			worst = d
		}
	}
	if worst > 1e-9 {
		t.Errorf("self-distance drifted to %g", worst)
	}
	t.Logf("200 random 128-d vectors: worst |d(v,v)| = %g", worst)
}

// TestCosineDistanceDegenerateInputs documents how the function behaves on
// zero, nil, empty and mismatched-length vectors. It must not NaN or panic.
func TestCosineDistanceDegenerateInputs(t *testing.T) {
	zero3 := []float32{0, 0, 0}
	v3 := []float32{1, 2, 3}

	cases := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"zero-vs-vector", zero3, v3, 1.0},
		{"vector-vs-zero", v3, zero3, 1.0},
		{"zero-vs-zero", zero3, zero3, 1.0},
		{"nil-vs-vector", nil, v3, 1.0},
		{"nil-vs-nil", nil, nil, 1.0},
		{"empty-vs-empty", []float32{}, []float32{}, 1.0},
		{"length-mismatch", []float32{1, 0}, []float32{1, 0, 0}, 1.0},
	}
	for _, tc := range cases {
		var got float64
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("cosineDistance panicked on %s: %v", tc.name, r)
				}
			}()
			got = cosineDistance(tc.a, tc.b)
		}()
		if math.IsNaN(got) {
			t.Errorf("%s produced NaN", tc.name)
		}
		if got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
		t.Logf("%-18s -> %.4f (no NaN, no panic)", tc.name, got)
	}

	t.Logf("")
	t.Logf("NO NaN AND NO PANIC: the normA==0 || normB==0 guard (clustering.go:152) and the")
	t.Logf("length/empty guard (clustering.go:141) both short-circuit to 1.0 before any division.")
	t.Logf("DEFECT (design, non-crashing): d(zero, zero) = 1.0, not 0. The function is therefore")
	t.Logf("not a metric -- identity of indiscernibles fails -- and a zero embedding is not even")
	t.Logf("its own neighbour, so it can never be a core point. Same for a length mismatch:")
	t.Logf("differing dimensions are silently reported as 'maximally dissimilar' rather than as")
	t.Logf("an error, which hides an embedding-model mismatch instead of surfacing it.")
}

// TestClusterWithDegenerateEmbeddings drives those degenerate vectors through
// the public Cluster path.
func TestClusterWithDegenerateEmbeddings(t *testing.T) {
	const dim = 64
	rng := newRNG(fixedSeed)
	eps, _ := plantedOrthogonal(rng, dim, 2, 5, 0.2)
	eps = append(eps,
		mkEpisode("zero", make([]float32, dim)),
		mkEpisode("nil", nil),
		mkEpisode("empty", []float32{}),
		mkEpisode("short", []float32{1, 0, 0}),
	)

	d := NewDBSCAN(prodEpsilon, prodMinPoints)
	var got []models.Cluster
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Cluster panicked on degenerate embeddings: %v", r)
			}
		}()
		got = d.Cluster(eps)
	}()

	real, noise := splitResult(got)
	if len(real) != 2 {
		t.Errorf("got %d real clusters, want 2", len(real))
	}
	degenerate := map[string]bool{"zero": true, "nil": true, "empty": true, "short": true}
	noiseIDs := []string{}
	for _, c := range noise {
		noiseIDs = append(noiseIDs, c.Episodes[0].ID)
	}
	sort.Strings(noiseIDs)
	for _, c := range real {
		for _, e := range c.Episodes {
			if degenerate[e.ID] {
				t.Errorf("degenerate episode %s ended up inside cluster %d", e.ID, c.ID)
			}
		}
	}
	if len(noise) != len(degenerate) {
		t.Errorf("got %d noise singletons %v, want %d", len(noise), noiseIDs, len(degenerate))
	}
	for _, c := range real {
		if c.Centroid == nil {
			t.Errorf("cluster %d has a nil centroid", c.ID)
		}
		for _, x := range c.Centroid {
			if math.IsNaN(float64(x)) {
				t.Fatalf("cluster %d centroid contains NaN", c.ID)
			}
		}
	}
	t.Logf("n=%d including zero/nil/empty/short embeddings: no panic, real_clusters=%d noise=%v",
		len(eps), len(real), noiseIDs)
	t.Logf("every degenerate embedding is isolated as noise, because a mismatched or zero vector reports distance 1.0 > epsilon 0.3")
}

// TestComputeCentroidMixedDimensionsPanics documents a real crash. It is
// unreachable at the production epsilon of 0.3 -- mismatched dimensions report
// distance 1.0, which exceeds 0.3 -- but configs/config.yaml's dbscan_epsilon
// is a free parameter and any value >= 1.0 makes it reachable.
func TestComputeCentroidMixedDimensionsPanics(t *testing.T) {
	short := mkEpisode("short", []float32{1, 0})
	long := mkEpisode("long", []float32{1, 0, 0, 0})

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_ = computeCentroid([]models.Episode{short, long})
	}()
	if recovered == nil {
		t.Errorf("computeCentroid([dim2, dim4]) did not panic; behaviour changed, update this test")
	}
	t.Logf("DEFECT: computeCentroid sizes the accumulator from episodes[0] (clustering.go:166) and then")
	t.Logf("        indexes it with a longer episode's range (clustering.go:173) -> panic: %v", recovered)

	// Reverse order is silently wrong rather than fatal: the extra components
	// of the shorter vector are simply never added.
	c := computeCentroid([]models.Episode{long, short})
	t.Logf("        reverse order does not panic, it silently truncates: centroid=%v (the 'short' episode contributes 2 of 4 dims)", c)

	// Reachable through the public API once epsilon >= 1.0.
	var clusterPanic any
	func() {
		defer func() { clusterPanic = recover() }()
		_ = NewDBSCAN(1.0, 1).Cluster([]models.Episode{short, long})
	}()
	if clusterPanic == nil {
		t.Logf("        Cluster(eps=1.0, minPts=1) did NOT panic on mixed dimensions")
	} else {
		t.Logf("        reachable via Cluster(epsilon=1.0, minPoints=1): panic: %v", clusterPanic)
	}
	t.Logf("        at the production epsilon of %.2f it is unreachable, since a dimension mismatch reports distance 1.0 > %.2f", prodEpsilon, prodEpsilon)
}

// TestNewDBSCANDefaults pins the fallback values, which are the same literals
// as configs/config.yaml.
func TestNewDBSCANDefaults(t *testing.T) {
	for _, tc := range []struct {
		eps  float64
		mp   int
		wEps float64
		wMP  int
	}{
		{0, 0, 0.3, 3},
		{-1, -5, 0.3, 3},
		{0.5, 7, 0.5, 7},
		{0.3, 3, 0.3, 3},
	} {
		d := NewDBSCAN(tc.eps, tc.mp)
		if d.epsilon != tc.wEps || d.minPoints != tc.wMP {
			t.Errorf("NewDBSCAN(%v, %v) = {%v, %v}, want {%v, %v}", tc.eps, tc.mp, d.epsilon, d.minPoints, tc.wEps, tc.wMP)
		}
	}
	t.Logf("NewDBSCAN falls back to epsilon=0.3 / minPoints=3 for non-positive inputs, matching configs/config.yaml")
}

// TestRegionQueryIncludesSelf pins the semantics minPoints is compared against:
// the neighbourhood includes the query point, so minPoints=3 means "2 other
// points nearby", not 3.
func TestRegionQueryIncludesSelf(t *testing.T) {
	rng := newRNG(fixedSeed)
	eps := []models.Episode{
		mkEpisode("a", jitter(rng, basis(32, 0), 0.02)),
		mkEpisode("b", jitter(rng, basis(32, 0), 0.02)),
		mkEpisode("far", basis(32, 5)),
	}
	d := NewDBSCAN(prodEpsilon, prodMinPoints)
	n0 := d.regionQuery(eps, 0)
	n2 := d.regionQuery(eps, 2)
	if len(n0) != 2 {
		t.Errorf("regionQuery(a) = %v, want 2 neighbours (a itself + b)", n0)
	}
	if len(n2) != 1 || n2[0] != 2 {
		t.Errorf("regionQuery(far) = %v, want [2] (only itself)", n2)
	}
	t.Logf("regionQuery includes the query point: |N(a)|=%d %v, |N(far)|=%d %v", len(n0), n0, len(n2), n2)
	t.Logf("=> minPoints=3 requires 2 OTHER episodes within epsilon, so the minimum cluster size is 3")
}
