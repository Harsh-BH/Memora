// Command postcheck completes two pieces of registered reporting that the
// forgetting harness prints but does not compute:
//
//   - V9 (PREREGISTRATION.md 3.2, 5.2). 3.1's arithmetic says that in the
//     primary row SR@1 depends only on which of a query's OWN gold turns are
//     archived, so the attribution 2x2's right-hand column ("own stale gold NOT
//     in A") must match RAW unless the detector archived one of that query's
//     CURRENT gold turns. The harness prints the 2x2 but never checks that
//     exception, so the run's V9 status was unresolved. This counts the
//     current-gold archives directly.
//   - 4.6's "all cells reported". The harness reports the archive-rate curve
//     for every grid cell (that is what selects theta*) but the mechanism
//     diagnostics only for the selected cell.
//
// Written AFTER the run, and it cannot move the primary cell: theta* is fixed
// by the control-set archive rate alone (4.6), which this program does not
// touch. Lexical only -- no embedder, no Qdrant.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/memora/cma/eval"
)

const thetaStar = 0.25 // selected by the registered 4.6 rule on the 398 control instances

type cell struct {
	Theta            float64 `json:"theta"`
	Archived         int     `json:"archived"`
	KUArchiveRate    float64 `json:"ku_archive_rate"`
	StaleGoldsHit    int     `json:"stale_golds_archived"`
	CurrentGoldsHit  int     `json:"current_golds_archived"`
	PairCatch        int     `json:"pair_catch_instances"`
	PairCatchRate    float64 `json:"pair_catch_rate"`
	InstancesWithAny int     `json:"instances_with_any_archive"`
}

func main() {
	all, err := eval.LoadLongMemEval(eval.LongMemEvalPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	ku := eval.Select(all, eval.IsKUPermissive)
	if len(ku) != 70 {
		fmt.Fprintf(os.Stderr, "KU-permissive = %d, registered 70\n", len(ku))
		os.Exit(1)
	}

	out := struct {
		Corpus         string  `json:"corpus_sha256"`
		KUInstances    int     `json:"ku_instances"`
		KUTurns        int     `json:"ku_turns"`
		D2Grid         []cell  `json:"d2_grid_on_ku"`
		V9ThetaStar    float64 `json:"v9_theta_star"`
		V9CurrentInA   int     `json:"v9_current_golds_archived_total"`
		V9RightColumn  int     `json:"v9_current_gold_archived_where_own_stale_NOT_archived"`
		V9LeftColumn   int     `json:"v9_current_gold_archived_where_own_stale_ALSO_archived"`
		V9Needed       int     `json:"v9_right_column_flips_needing_explanation"`
		V9Verdict      string  `json:"v9_verdict"`
		ProbeLexTop1   string  `json:"probe_lexical_top1_for_contrast"`
	}{Corpus: "821a2034d219ab45846873dd14c14f12cfe7776e73527a483f9dac095d38620c", KUInstances: len(ku),
		V9ThetaStar: thetaStar, ProbeLexTop1: "23/70 = 0.3286 (cma/eval/out/probe.json, measured)"}

	for _, th := range []float64{0.10, 0.15, 0.20, 0.25} {
		c := cell{Theta: th}
		turnsTotal := 0
		for _, in := range ku {
			turns, err := in.Turns()
			if err != nil {
				fmt.Fprintln(os.Stderr, "turns:", err)
				os.Exit(1)
			}
			turnsTotal += len(turns)
			gold := eval.GoldOf(turns)
			pairs := eval.NearestOlderIDF(turns, th)
			set := eval.ArchiveSet(pairs)
			c.Archived += len(set)
			if len(set) > 0 {
				c.InstancesWithAny++
			}
			for id := range set {
				if gold.Stale[id] {
					c.StaleGoldsHit++
				}
				if gold.Current[id] {
					c.CurrentGoldsHit++
				}
			}
			for _, p := range pairs {
				// The mechanism: anchored on a CURRENT gold, selected that
				// query's own STALE gold as its nearest strictly-older turn.
				if gold.Current[p.Anchor] && gold.Stale[p.Target] {
					c.PairCatch++
					break
				}
			}

			if th != thetaStar {
				continue
			}
			ownStaleArchived := false
			for id := range gold.Stale {
				if set[id] {
					ownStaleArchived = true
				}
			}
			ownCurrentArchived := false
			for id := range gold.Current {
				if set[id] {
					ownCurrentArchived = true
				}
			}
			if ownCurrentArchived {
				out.V9CurrentInA++
				if ownStaleArchived {
					out.V9LeftColumn++
				} else {
					out.V9RightColumn++
				}
			}
		}
		c.KUArchiveRate = float64(c.Archived) / float64(turnsTotal)
		c.PairCatchRate = float64(c.PairCatch) / float64(len(ku))
		out.KUTurns = turnsTotal
		out.D2Grid = append(out.D2Grid, c)
	}

	// The measured run: RAW 33/70 hits, D2* attribution left column 9 hits of
	// 10 queries, right column 22 hits of 60. Archiving a stale gold can only
	// raise SR@1, so RAW's left-column hits <= 9 and RAW's right column >= 24;
	// at least 2 right-column queries must therefore have flipped 1 -> 0, and
	// 3.2 permits that ONLY where a CURRENT gold was archived.
	out.V9Needed = 2
	if out.V9RightColumn >= out.V9Needed {
		out.V9Verdict = fmt.Sprintf("V9 SATISFIED: %d right-column queries had a current gold archived, "+
			"which is >= the %d flips the attribution 2x2 requires an explanation for", out.V9RightColumn, out.V9Needed)
	} else {
		out.V9Verdict = fmt.Sprintf("V9 VIOLATED: only %d right-column queries had a current gold archived, "+
			"fewer than the %d flips the attribution 2x2 needs explained -- harness bug", out.V9RightColumn, out.V9Needed)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
}
