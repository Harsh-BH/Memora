package eval

import (
	"math"
	"math/rand"
	"os"
	"testing"
)

// This file pins the pure arithmetic of the forgetting experiment: the query
// sets, the total order, the detectors, the metric, and the statistics. None of
// it needs Qdrant, ONNX, or a network, so it runs in the normal suite and fails
// loudly if any registered rule drifts.
//
// The numbers asserted here were published in cma/eval/PREREGISTRATION.md
// BEFORE any arm ran. A test here failing means either the corpus changed
// (which sha256 verification would have caught first) or a rule was edited
// after registration. Neither is a thing to fix by adjusting the expectation.

// loadCorpus skips rather than fails when the corpora are absent: they are 18MB
// of third-party data, deliberately not committed (testdata/DATASETS.md), so a
// fresh clone has no copy.
func loadCorpus(t *testing.T) []*LMEInstance {
	t.Helper()
	path := LongMemEvalPath()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("LongMemEval not present at %s (see cma/eval/testdata/DATASETS.md): %v", path, err)
	}
	all, err := LoadLongMemEval(path)
	if err != nil {
		t.Fatalf("LoadLongMemEval: %v", err)
	}
	return all
}

// TestRegisteredCorpusCounts re-derives every structural number
// PREREGISTRATION.md 4.2/4.3/4.6 published, from the sha-verified file. These
// are the numbers the primary cell's reproducibility rests on: the whole point
// of registering rules rather than ID lists is that this test can regenerate
// the sets from the document alone.
func TestRegisteredCorpusCounts(t *testing.T) {
	all := loadCorpus(t)

	if len(all) != 500 {
		t.Errorf("instances = %d, registered 500", len(all))
	}

	types := map[string]int{}
	sessions, turns, abstentions := 0, 0, 0
	for _, in := range all {
		types[in.QuestionType]++
		sessions += len(in.HaystackSessions)
		for _, s := range in.HaystackSessions {
			turns += len(s)
		}
		if in.IsAbstention() {
			abstentions++
		}
	}
	for typ, want := range map[string]int{
		"temporal-reasoning": 133, "multi-session": 133, "knowledge-update": 78,
		"single-session-user": 70, "single-session-assistant": 56, "single-session-preference": 30,
	} {
		if types[typ] != want {
			t.Errorf("question_type %s = %d, registered %d", typ, types[typ], want)
		}
	}
	if sessions != 948 || turns != 10960 {
		t.Errorf("sessions/turns = %d/%d, registered 948/10960", sessions, turns)
	}
	if abstentions != 30 {
		t.Errorf("_abs items = %d, registered 30", abstentions)
	}

	ku := Select(all, IsKUPermissive)
	if len(ku) != 70 {
		t.Fatalf("KU-permissive n = %d, registered 70 -- the PRIMARY set is wrong, nothing "+
			"downstream of this means anything", len(ku))
	}
	control := Select(all, IsControl)
	if len(control) != 407 {
		t.Errorf("control n = %d, registered 407", len(control))
	}
	controlPrimary := Select(all, IsControlPrimary)
	if len(controlPrimary) != 398 {
		t.Errorf("control-primary (falsifier) n = %d, registered 398", len(controlPrimary))
	}
	if got := len(control) - len(controlPrimary); got != 9 {
		t.Errorf("_abs inside the control set = %d, registered 9", got)
	}

	// The alternative permissive rule the plan proposed yields 68, and is NOT
	// the registered rule. Asserted so the two can never be confused.
	alt := 0
	for _, in := range all {
		if in.QuestionType != "knowledge-update" {
			continue
		}
		flags := 0
		for _, s := range in.HaystackSessions {
			for _, tn := range s {
				if tn.HasAnswer {
					flags++
				}
			}
		}
		if flags == 2 {
			alt++
		}
	}
	if alt != 68 {
		t.Errorf("the plan's alternative rule = %d, registered 68 (and is not the rule used)", alt)
	}

	// Registered structural facts the harness must assert (4.2).
	kuTurns, stale, current, goldTurns := 0, 0, 0, 0
	minTurns, maxTurns := math.MaxInt, 0
	for _, in := range ku {
		if in.IsAbstention() {
			t.Errorf("%s: KU-permissive must contain 0 _abs items", in.QuestionID)
		}
		if len(in.HaystackSessions) != 2 || len(in.AnswerSessionIDs) != 2 {
			t.Errorf("%s: %d sessions / %d answer_session_ids, registered 2/2",
				in.QuestionID, len(in.HaystackSessions), len(in.AnswerSessionIDs))
		}
		if in.HaystackDates[0] == in.HaystackDates[1] {
			t.Errorf("%s: haystack_dates are not distinct", in.QuestionID)
		}
		tns, err := in.Turns()
		if err != nil {
			t.Fatalf("%s: Turns: %v", in.QuestionID, err)
		}
		kuTurns += len(tns)
		if len(tns) < minTurns {
			minTurns = len(tns)
		}
		if len(tns) > maxTurns {
			maxTurns = len(tns)
		}
		g := GoldOf(tns)
		stale += len(g.Stale)
		current += len(g.Current)
		for _, tn := range tns {
			if tn.HasAnswer {
				goldTurns++
				if tn.Role != "user" {
					t.Errorf("%s: gold turn has role %q, registered: all 142 are \"user\"", in.QuestionID, tn.Role)
				}
			}
		}
	}
	if kuTurns != 1640 || minTurns != 20 || maxTurns != 24 {
		t.Errorf("KU-permissive turns = %d (min %d, max %d), registered 1640 (20..24)", kuTurns, minTurns, maxTurns)
	}
	if goldTurns != 142 || stale != 72 || current != 70 {
		t.Fatalf("gold = %d turns, %d stale / %d current; registered 142 = 72/70. "+
			"The stale/current split IS the gold map's orientation.", goldTurns, stale, current)
	}

	// rho* = 0.0439, the ORACLE arm's own archive rate, is the theta* selection
	// target. It was published before any arm ran; recompute it here rather
	// than trusting the constant.
	rho := float64(stale) / float64(kuTurns)
	if math.Abs(rho-0.0439) > 5e-5 {
		t.Errorf("rho* = %.6f, registered 0.0439 (72/1640)", rho)
	}

	controlTurns := 0
	for _, in := range control {
		for _, s := range in.HaystackSessions {
			controlTurns += len(s)
		}
	}
	if controlTurns != 8804 {
		t.Errorf("control turns = %d, registered 8804", controlTurns)
	}
	t.Logf("MEASURED, matching registration: 500 instances / %d sessions / %d turns; "+
		"KU-permissive n=%d (%d turns, %d..%d each, %d stale + %d current gold, rho*=%.4f); "+
		"control n=%d (%d turns), falsifier n=%d",
		sessions, turns, len(ku), kuTurns, minTurns, maxTurns, stale, current, rho,
		len(control), controlTurns, len(controlPrimary))
}

// TestSHA256GateRejectsWrongCorpus checks void condition V8 actually fires.
// A sha gate that is never exercised is a comment.
func TestSHA256GateRejectsWrongCorpus(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "notlme*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("[]"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := LoadLongMemEval(f.Name()); err == nil {
		t.Fatal("LoadLongMemEval accepted a file with the wrong sha256 -- V8 does not fire")
	}
}

// --- synthetic fixtures, so the rules are testable without the corpus ---

// fixture builds a 2-session instance. Session 0 is dated LATER than session 1,
// so a test that ignores the date and trusts the array index gets the order
// backwards -- which is the mistake the 4.4 total order exists to prevent.
func fixture() *LMEInstance {
	return &LMEInstance{
		QuestionID:         "fix_1",
		QuestionType:       "knowledge-update",
		QuestionDate:       "2023/06/01 (Thu) 12:00",
		HaystackDates:      []string{"2023/05/10 (Wed) 09:00", "2023/03/01 (Wed) 09:00"},
		HaystackSessionIDs: []string{"s0", "s1"},
		AnswerSessionIDs:   []string{"s0", "s1"},
		HaystackSessions: [][]LMETurn{
			{ // session 0, the NEWER one by date
				{Role: "user", Content: "my favourite coffee is now a flat white", HasAnswer: true},
				{Role: "assistant", Content: "noted, a flat white it is"},
			},
			{ // session 1, the OLDER one by date
				{Role: "user", Content: "my favourite coffee is a cortado", HasAnswer: true},
				{Role: "assistant", Content: "understood, cortado"},
				{Role: "user", Content: "unrelated: the dog needs a walk"},
			},
		},
	}
}

// TestTotalOrderIsByDateNotIndex pins PREREGISTRATION.md 4.4.
func TestTotalOrderIsByDateNotIndex(t *testing.T) {
	turns, err := fixture().Turns()
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 5 {
		t.Fatalf("got %d turns, want 5", len(turns))
	}
	// Session 1 is older by date, so all three of its turns must come first
	// even though its array index is higher.
	wantSessions := []int{1, 1, 1, 0, 0}
	wantTurnIdx := []int{0, 1, 2, 0, 1}
	for i, tn := range turns {
		if tn.SessionIdx != wantSessions[i] || tn.TurnIdx != wantTurnIdx[i] {
			t.Fatalf("position %d is session %d turn %d, want session %d turn %d -- "+
				"the 4.4 order is (session date, session index, turn index), not array order",
				i, tn.SessionIdx, tn.TurnIdx, wantSessions[i], wantTurnIdx[i])
		}
	}
	// Turn timestamps are session date + turn_idx minutes, and are NOT the
	// ordering authority.
	if d := turns[1].Timestamp().Sub(turns[0].Timestamp()); d.Minutes() != 1 {
		t.Errorf("consecutive turns are %v apart, want 1 minute", d)
	}
	// IDs are deterministic across calls: a frozen gold map keyed by UUID
	// would be worthless otherwise.
	again, _ := fixture().Turns()
	for i := range turns {
		if turns[i].ID != again[i].ID {
			t.Fatalf("turn %d got a different ID on a second load (%s vs %s) -- IDs must be deterministic",
				i, turns[i].ID, again[i].ID)
		}
	}
}

// TestGoldOrientation is the unit-level version of the ANTI-ORACLE check: the
// OLDER gold turn is stale, the NEWER one current. Swapping these is invisible
// to ORACLE and is exactly what V1 exists to catch.
func TestGoldOrientation(t *testing.T) {
	turns, _ := fixture().Turns()
	g := GoldOf(turns)
	if len(g.Stale) != 1 || len(g.Current) != 1 {
		t.Fatalf("gold = %d stale / %d current, want 1/1", len(g.Stale), len(g.Current))
	}
	if !g.Stale[turns[0].ID] {
		t.Errorf("the cortado turn (older session) must be STALE")
	}
	if !g.Current[turns[3].ID] {
		t.Errorf("the flat-white turn (newer session) must be CURRENT")
	}
}

// TestNearestOlderIDFRules pins the D2 detector's registered behaviour.
func TestNearestOlderIDFRules(t *testing.T) {
	turns, _ := fixture().Turns()

	// theta = 0 archives a target for every turn that has a strictly-older one,
	// and never for the first turn: the first turn has empty U(t).
	pairs := NearestOlderIDF(turns, 0)
	if len(pairs) != len(turns)-1 {
		t.Fatalf("at theta=0 got %d pairs, want %d (one per turn with a strictly-older neighbour)",
			len(pairs), len(turns)-1)
	}
	for _, p := range pairs {
		if p.Target == turns[len(turns)-1].ID {
			t.Errorf("the newest turn was archived; D2 structurally cannot archive it")
		}
	}

	// theta = 1.1 is above any possible similarity, so nothing is archived.
	if got := NearestOlderIDF(turns, 1.1); len(got) != 0 {
		t.Errorf("at theta above 1.0 got %d pairs, want 0", len(got))
	}

	// Sign convention (4.5): D2 thresholds a SIMILARITY with >=, so raising
	// theta can only shrink the archive set.
	prev := len(ArchiveSet(NearestOlderIDF(turns, 0)))
	for _, theta := range []float64{0.10, 0.15, 0.20, 0.25} {
		n := len(ArchiveSet(NearestOlderIDF(turns, theta)))
		if n > prev {
			t.Errorf("archive set GREW from %d to %d as theta rose to %.2f -- the sign is flipped", prev, n, theta)
		}
		prev = n
	}

	// The paired anchor for the current gold should be its own stale twin here:
	// "my favourite coffee is now a flat white" vs "my favourite coffee is a
	// cortado" share four content tokens, against an unrelated dog turn.
	g := GoldOf(turns)
	var caught bool
	for _, p := range NearestOlderIDF(turns, 0) {
		if g.Current[p.Anchor] && g.Stale[p.Target] {
			caught = true
		}
	}
	if !caught {
		t.Errorf("D2 did not pair the current gold with its stale twin on a fixture built to be easy; " +
			"that is a detector bug, not a finding")
	}
}

// TestNearestOlderTieBreakIsEarliest pins the 4.5 tie-break: on equal score,
// the u EARLIEST in the 4.4 order wins. With three identical older turns the
// choice is otherwise arbitrary, and an arbitrary choice makes A irreproducible.
func TestNearestOlderTieBreakIsEarliest(t *testing.T) {
	in := &LMEInstance{
		QuestionID:         "tie",
		HaystackDates:      []string{"2023/01/01 (Sun) 09:00"},
		HaystackSessionIDs: []string{"s0"},
		HaystackSessions: [][]LMETurn{{
			{Role: "user", Content: "alpha beta gamma"},
			{Role: "user", Content: "alpha beta gamma"},
			{Role: "user", Content: "alpha beta gamma"},
		}},
	}
	turns, _ := in.Turns()
	pairs := NearestOlderIDF(turns, 0)
	last := pairs[len(pairs)-1]
	if last.Anchor != turns[2].ID || last.Target != turns[0].ID {
		t.Fatalf("the third turn paired with %s, want the EARLIEST tied candidate %s",
			last.Target, turns[0].ID)
	}
	if math.Abs(last.Score-1.0) > 1e-9 {
		t.Errorf("identical texts scored simIDF %.6f, want exactly 1.0", last.Score)
	}
}

// TestIDFJaccardArithmetic checks simIDF against a value computed by hand, so
// a refactor cannot quietly change what "IDF-weighted Jaccard" means.
func TestIDFJaccardArithmetic(t *testing.T) {
	// Two docs. "shared" appears in both (df=2), "a" and "b" in one each (df=1).
	docs := []string{"shared a", "shared b"}
	idf := NewBM25(docs).idf
	a, b := tokenSet(docs[0]), tokenSet(docs[1])
	// intersection = {shared}; union = {shared, a, b}
	want := idf("shared") / (idf("shared") + idf("a") + idf("b"))
	if got := idfJaccard(a, b, idf); math.Abs(got-want) > 1e-12 {
		t.Fatalf("idfJaccard = %.12f, want %.12f", got, want)
	}
	// Rarer terms carry more weight than the shared common term, so this is
	// below the unweighted Jaccard of 1/3.
	if got := idfJaccard(a, b, idf); got >= 1.0/3.0 {
		t.Errorf("idfJaccard = %.4f, expected below the unweighted 0.3333 since the shared "+
			"term is the common one", got)
	}
	if got := idfJaccard(a, a, idf); math.Abs(got-1.0) > 1e-12 {
		t.Errorf("a document against itself scored %.12f, want 1.0", got)
	}
	if got := idfJaccard(a, map[string]bool{"zzz": true}, idf); got != 0 {
		t.Errorf("disjoint token sets scored %.12f, want 0", got)
	}
}

// TestNearestOlderDenseSignConvention pins D1's opposite sign: it thresholds a
// DISTANCE with <=, so raising theta can only GROW the archive set. D1 and D2
// moving the same way under theta would mean one of them is inverted.
func TestNearestOlderDenseSignConvention(t *testing.T) {
	turns, _ := fixture().Turns()
	vecs := map[string][]float32{}
	for i, tn := range turns {
		// Spread the turns along a circle so distances are distinct and known
		// to be strictly between 0 and 2.
		ang := float64(i) * 0.3
		vecs[tn.ID] = []float32{float32(math.Cos(ang)), float32(math.Sin(ang))}
	}
	prev := -1
	for _, theta := range []float64{0.0, 0.20, 0.30, 0.40, 0.50, 2.0} {
		n := len(ArchiveSet(NearestOlderDense(turns, vecs, theta)))
		if prev >= 0 && n < prev {
			t.Fatalf("archive set SHRANK from %d to %d as theta rose to %.2f -- D1 thresholds a "+
				"distance with <=, so it must grow", prev, n, theta)
		}
		prev = n
	}
	if prev != len(turns)-1 {
		t.Errorf("at theta=2.0 (everything within range) got %d archived, want %d", prev, len(turns)-1)
	}
	if got := len(NearestOlderDense(turns, vecs, -1)); got != 0 {
		t.Errorf("at a negative distance threshold got %d pairs, want 0", got)
	}
}

// TestCosineDistanceMatchesConsolidation pins the conventions D0 and D1 share,
// including the two degenerate cases consolidation.cosineDistance defines as 1.0.
func TestCosineDistanceMatchesConsolidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical", []float32{1, 0}, []float32{1, 0}, 0},
		{"orthogonal", []float32{1, 0}, []float32{0, 1}, 1},
		{"opposite", []float32{1, 0}, []float32{-1, 0}, 2},
		{"unnormalised is scale-invariant", []float32{2, 0}, []float32{5, 0}, 0},
		{"length mismatch", []float32{1, 0}, []float32{1, 0, 0}, 1},
		{"empty", nil, nil, 1},
		{"zero vector", []float32{0, 0}, []float32{1, 0}, 1},
	} {
		if got := CosineDistance(tc.a, tc.b); math.Abs(got-tc.want) > 1e-6 {
			t.Errorf("%s: CosineDistance = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestArmSelfChecks is the unit-level rehearsal of V1/V2/V7. If ORACLE is not
// 1.0, ANTI-ORACLE not 0.0, and FLOOR not 1.0 on a fixture where the answer is
// known by construction, the harness's own self-checks would be measuring the
// harness rather than the corpus.
func TestArmSelfChecks(t *testing.T) {
	turns, _ := fixture().Turns()
	g := GoldOf(turns)

	// A ranked list in which the STALE gold outranks the current one: the
	// adversarial case, and the only one where archiving changes the answer.
	ranked := []Ranked{
		{ID: turns[0].ID, Score: 0.9}, // stale gold
		{ID: turns[2].ID, Score: 0.8}, // non-gold
		{ID: turns[3].ID, Score: 0.7}, // current gold
		{ID: turns[1].ID, Score: 0.6},
		{ID: turns[4].ID, Score: 0.5},
	}

	if got := SRAt1(Filter(ranked, nil, 50), g); got != SRStale {
		t.Fatalf("RAW on this fixture = %d, want SRStale (%d) -- the fixture has no headroom otherwise",
			got, SRStale)
	}
	if got := SRAt1(Filter(ranked, OracleArchive(g), 50), g); got != SRCurrent {
		t.Errorf("ORACLE = %d, want SRCurrent (V2 voids the run otherwise)", got)
	}
	if got := SRAt1(Filter(ranked, AntiOracleArchive(g), 50), g); got != SRStale {
		t.Errorf("ANTI-ORACLE = %d, want SRStale, i.e. SR@1 exactly 0.0 (V1)", got)
	}
	if got := SRAt1(Filter(ranked, FloorArchive(turns), 50), g); got != SRCurrent {
		t.Errorf("FLOOR = %d, want SRCurrent (V7 voids the run otherwise)", got)
	}
	// Archiving every gold turn leaves SR@1 undefined, which must NOT be
	// silently scored as 0.
	all := OracleArchive(g)
	for id := range AntiOracleArchive(g) {
		all[id] = true
	}
	if got := SRAt1(Filter(ranked, all, 50), g); got != SRUndefined {
		t.Errorf("with every gold archived SR@1 = %d, want SRUndefined (%d)", got, SRUndefined)
	}
}

// TestFloorKeepsOnlyNewestSession pins FLOOR's definition against the 4.4 order
// rather than the array order.
func TestFloorKeepsOnlyNewestSession(t *testing.T) {
	turns, _ := fixture().Turns()
	a := FloorArchive(turns)
	if len(a) != 3 {
		t.Fatalf("FLOOR archived %d turns, want the 3 in the older session", len(a))
	}
	for _, tn := range turns {
		if want := tn.SessionIdx != 0; a[tn.ID] != want { // session 0 is the newer by DATE
			t.Errorf("turn s%d/t%d archived=%v, want %v", tn.SessionIdx, tn.TurnIdx, a[tn.ID], want)
		}
	}
}

// TestRandomArchiveEligibilityAndRate pins the three registered properties of
// the mandatory rate-matched control (2.1).
func TestRandomArchiveEligibilityAndRate(t *testing.T) {
	turns, _ := fixture().Turns()
	last := turns[len(turns)-1].ID

	for r := 0; r < 20; r++ {
		rng := rand.New(rand.NewSource(20260901 + int64(r)))
		a := RandomArchive(turns, 2, rng)
		if len(a) != 2 {
			t.Fatalf("replicate %d archived %d turns, want the exact rate match 2", r, len(a))
		}
		if a[last] {
			t.Fatalf("replicate %d archived the newest turn; the eligibility set is every turn "+
				"EXCEPT the last, because D1/D2 structurally cannot archive it", r)
		}
	}

	// Deterministic given the seed: 4.8 fixes base seed 20260901 and says there
	// is no other randomness in the experiment.
	one := RandomArchive(turns, 2, rand.New(rand.NewSource(20260901)))
	two := RandomArchive(turns, 2, rand.New(rand.NewSource(20260901)))
	if len(one) != len(two) {
		t.Fatal("same seed produced different sizes")
	}
	for id := range one {
		if !two[id] {
			t.Fatal("same seed produced a different archive set -- RANDOM-ARCHIVE is not reproducible")
		}
	}

	// n larger than the eligible set is clamped, never a panic.
	if got := len(RandomArchive(turns, 999, rand.New(rand.NewSource(1)))); got != len(turns)-1 {
		t.Errorf("over-large n gave %d, want the whole eligible set %d", got, len(turns)-1)
	}
	if got := len(RandomArchive(turns, 0, rand.New(rand.NewSource(1)))); got != 0 {
		t.Errorf("n=0 gave %d, want 0 (this is the RAW-matched case)", got)
	}
}

// TestShippedConsolidatorNeedsThreePoints is the D0 arm's structural finding,
// as a test: at the shipped dbscan_min_points 3, a 2-point supersession pair
// CANNOT form a cluster, so the shipped consolidator's supersession recall is
// structurally zero regardless of epsilon.
func TestShippedConsolidatorNeedsThreePoints(t *testing.T) {
	in := &LMEInstance{
		QuestionID:         "d0",
		HaystackDates:      []string{"2023/01/01 (Sun) 09:00"},
		HaystackSessionIDs: []string{"s0"},
		HaystackSessions: [][]LMETurn{{
			{Role: "user", Content: "a"},
			{Role: "user", Content: "b"},
		}},
	}
	turns, _ := in.Turns()
	vecs := map[string][]float32{turns[0].ID: {1, 0}, turns[1].ID: {1, 0}} // identical, distance 0

	if got := ShippedConsolidatorArchive(turns, vecs, 0.3, 3); len(got) != 0 {
		t.Errorf("two identical points at the SHIPPED minPoints=3 archived %d turns, want 0 -- "+
			"a 2-point pair cannot reach minPoints", len(got))
	}
	// At minPoints 2 the same pair does cluster, which proves the zero above is
	// minPoints and not a broken call.
	if got := ShippedConsolidatorArchive(turns, vecs, 0.3, 2); len(got) != 1 {
		t.Errorf("at minPoints=2 the identical pair archived %d turns, want 1 (all but the newest)", len(got))
	}
	// A single turn yields nothing: clustering.go returns noise points as
	// singleton clusters, so the len>=2 guard is load-bearing.
	one, _ := (&LMEInstance{
		QuestionID: "d0b", HaystackDates: []string{"2023/01/01 (Sun) 09:00"},
		HaystackSessionIDs: []string{"s0"},
		HaystackSessions:   [][]LMETurn{{{Role: "user", Content: "a"}}},
	}).Turns()
	if got := ShippedConsolidatorArchive(one, map[string][]float32{one[0].ID: {1, 0}}, 0.3, 3); len(got) != 0 {
		t.Errorf("a 1-turn instance archived %d turns, want 0", len(got))
	}
}

// TestShippedConsolidatorArchivesAllButNewest pins which member survives.
func TestShippedConsolidatorArchivesAllButNewest(t *testing.T) {
	in := &LMEInstance{
		QuestionID:         "d0c",
		HaystackDates:      []string{"2023/05/10 (Wed) 09:00", "2023/03/01 (Wed) 09:00"},
		HaystackSessionIDs: []string{"s0", "s1"},
		HaystackSessions: [][]LMETurn{
			{{Role: "user", Content: "x"}}, // newer by date
			{{Role: "user", Content: "x"}, {Role: "user", Content: "x"}},
		},
	}
	turns, _ := in.Turns()
	vecs := map[string][]float32{}
	for _, tn := range turns {
		vecs[tn.ID] = []float32{1, 0}
	}
	got := ShippedConsolidatorArchive(turns, vecs, 0.3, 3)
	newest := turns[len(turns)-1].ID // 4.4 order, i.e. session 0 by date
	if len(got) != 2 || got[newest] {
		t.Fatalf("archived %d turns and newest-archived=%v; want 2 archived and the newest kept",
			len(got), got[newest])
	}
}

// TestRecencyRescoreD3 pins the rival arm's arithmetic and its stable tie-break.
func TestRecencyRescoreD3(t *testing.T) {
	ranked := []Ranked{{ID: "old", Score: 0.60}, {ID: "new", Score: 0.55}}
	age := map[string]float64{"old": 365, "new": 1}

	// w=0 must be an exact no-op: D3 at zero weight is RAW.
	if out := RecencyRescore(ranked, age, 0, 30); out[0].ID != "old" || out[1].ID != "new" {
		t.Errorf("w=0 reordered the list; it must be a no-op")
	}
	// A large enough weight promotes the recent-but-lower-cosine item.
	if out := RecencyRescore(ranked, age, 0.3, 30); out[0].ID != "new" {
		t.Errorf("w=0.3 tau=30 left %q on top; the recency prior should promote \"new\"", out[0].ID)
	}
	// Equal scores keep the incoming (cosine) order: sort.SliceStable, not
	// dig.go:75's unstable sort.Slice.
	tied := []Ranked{{ID: "first", Score: 0.5}, {ID: "second", Score: 0.5}}
	flat := map[string]float64{"first": 10, "second": 10}
	if out := RecencyRescore(tied, flat, 0.1, 30); out[0].ID != "first" {
		t.Errorf("tied scores reordered to %q first; the tie-break must be the incoming order", out[0].ID)
	}
}

// TestMcNemarExact checks the confirmatory test against values computable by
// hand, including the one PREREGISTRATION.md 4.14 quotes in its power section.
func TestMcNemarExact(t *testing.T) {
	// The registered power projection: 8 discordant pairs all in one direction
	// gives p = 2 * 0.5^8 = 0.0078125.
	if got := McNemarExact(8, 0); math.Abs(got-0.0078125) > 1e-12 {
		t.Errorf("McNemarExact(8,0) = %.10f, want 0.0078125 (the 4.14 power projection)", got)
	}
	if got := McNemarExact(0, 0); got != 1.0 {
		t.Errorf("McNemarExact(0,0) = %v, want 1.0 -- no discordant pairs is no evidence, not a divide by zero", got)
	}
	if got := McNemarExact(5, 5); got != 1.0 {
		t.Errorf("McNemarExact(5,5) = %v, want 1.0 (capped)", got)
	}
	// Symmetric: which arm is "b" cannot change a two-sided p.
	if McNemarExact(9, 2) != McNemarExact(2, 9) {
		t.Error("McNemarExact is not symmetric in b and c")
	}
	// 6 vs 0 is 2*0.5^6 = 0.03125, i.e. just under alpha; 5 vs 0 is 0.0625, over.
	if got := McNemarExact(6, 0); math.Abs(got-0.03125) > 1e-12 {
		t.Errorf("McNemarExact(6,0) = %.10f, want 0.03125", got)
	}
	if McNemarExact(5, 0) <= 0.05 {
		t.Errorf("McNemarExact(5,0) = %.6f, expected above alpha=0.05", McNemarExact(5, 0))
	}
	// Large n must not overflow: the naive factorial form does.
	if got := McNemarExact(200, 150); got <= 0 || got > 1 {
		t.Errorf("McNemarExact(200,150) = %v, want a probability in (0,1]", got)
	}
}

// TestEquivalenceBoundArithmetic pins why the registered bound is +/-10pp: at
// n=70 that allows 12 discordant pairs, while +/-3pp would allow 1, which is
// unreachable and would silently degrade to a bare p>0.05.
func TestEquivalenceBoundArithmetic(t *testing.T) {
	if WilsonHalfWidth(12, 0, 70) > 0.10 {
		t.Errorf("12 discordant pairs give half-width %.4f, registered as within 0.10", WilsonHalfWidth(12, 0, 70))
	}
	if WilsonHalfWidth(13, 0, 70) <= 0.10 {
		t.Errorf("13 discordant pairs give half-width %.4f, registered as OUTSIDE 0.10", WilsonHalfWidth(13, 0, 70))
	}
	if WilsonHalfWidth(2, 0, 70) <= 0.03 {
		t.Errorf("a +/-3pp bound would be reachable at 2 discordant pairs (half-width %.4f); "+
			"4.14 says it needs b+c <= 1", WilsonHalfWidth(2, 0, 70))
	}
}

// TestDiscordantSkipsUndefined: a query where either arm has no surviving gold
// carries no information about a difference, and scoring it as 0 would
// manufacture discordant pairs out of the evidence-found rate.
func TestDiscordantSkipsUndefined(t *testing.T) {
	a := []int{SRCurrent, SRStale, SRUndefined, SRCurrent, SRCurrent}
	b := []int{SRStale, SRStale, SRCurrent, SRCurrent, SRUndefined}
	bOnly, cOnly, skipped := Discordant(a, b)
	if bOnly != 1 || cOnly != 0 || skipped != 2 {
		t.Fatalf("Discordant = (%d, %d, skipped %d), want (1, 0, skipped 2)", bOnly, cOnly, skipped)
	}
	mean, defined, undefined := MeanDefined(a)
	if defined != 4 || undefined != 1 || math.Abs(mean-0.75) > 1e-12 {
		t.Errorf("MeanDefined = (%.4f, %d defined, %d undefined), want (0.75, 4, 1)", mean, defined, undefined)
	}
	if m, d, _ := MeanDefined([]int{SRUndefined, SRUndefined}); d != 0 || m != 0 {
		t.Errorf("an all-undefined arm gave mean %v over %d defined; the mean must not be taken over an empty set", m, d)
	}
}

// TestFilterTruncatesAndExcludes pins the offline set-filter every arm shares.
func TestFilterTruncatesAndExcludes(t *testing.T) {
	ranked := []Ranked{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	got := Filter(ranked, map[string]bool{"b": true}, 2)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Fatalf("Filter = %v, want [a c]", got)
	}
	if len(Filter(ranked, nil, 50)) != 4 {
		t.Error("Filter with an empty archive set must return the whole list (this is RAW)")
	}
	// recall@k counts within k of the FILTERED list, so archiving can promote
	// a gold item into the window -- the pooled row's only mechanism.
	gold := map[string]bool{"c": true}
	if RecallAt(ranked, gold, 2) {
		t.Error("recall@2 on the unfiltered list should miss gold at position 3")
	}
	if !RecallAt(Filter(ranked, map[string]bool{"a": true}, 50), gold, 2) {
		t.Error("after archiving position 1, gold moves into the top 2 and recall@2 should hit")
	}
	if rr := ReciprocalRank(ranked, gold); math.Abs(rr-1.0/3.0) > 1e-12 {
		t.Errorf("ReciprocalRank = %.6f, want 1/3", rr)
	}
	if rr := ReciprocalRank(ranked, map[string]bool{"zz": true}); rr != 0 {
		t.Errorf("ReciprocalRank with no gold present = %v, want 0", rr)
	}
}

// TestLoCoMoStripsGistFields enforces the rule PREREGISTRATION.md 4.1 requires
// be in code and not prose: observation, session_summary and event_summary are
// GPT-written gists tagged with the same dia_ids the gold evidence points to,
// so reading any of them is a guaranteed, meaningless win.
func TestLoCoMoStripsGistFields(t *testing.T) {
	path := LoCoMoPath()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("LoCoMo not present at %s (see cma/eval/testdata/DATASETS.md): %v", path, err)
	}
	// The raw file must actually contain the banned fields, otherwise this test
	// would pass on a corpus that never had them.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range loCoMoBannedFields {
		if !containsKey(string(raw), banned) {
			t.Fatalf("the raw corpus has no %q key, so the strip test proves nothing", banned)
		}
	}

	samples, err := LoadLoCoMo(path)
	if err != nil {
		t.Fatalf("LoadLoCoMo: %v", err)
	}
	if len(samples) != 10 {
		t.Errorf("LoCoMo samples = %d, want 10", len(samples))
	}
	cat2 := 0
	for _, s := range samples {
		for _, banned := range loCoMoBannedFields {
			if _, ok := s.Conversation[banned]; ok {
				t.Errorf("sample %s still carries %q after loading", s.SampleID, banned)
			}
		}
		for _, qa := range s.QA {
			if qa.Category == 2 {
				cat2++
			}
		}
	}
	if cat2 != 321 {
		t.Errorf("LoCoMo cat2 (temporal) questions = %d, registered 321", cat2)
	}
}

func containsKey(s, key string) bool {
	needle := `"` + key + `"`
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
