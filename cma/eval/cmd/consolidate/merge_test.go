package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memora/cma/internal/models"
)

// TestMergeInto pins the one piece of arithmetic that can silently corrupt the
// frozen artifact: folding a top-up pass into an earlier, incomplete one. A
// merge that dropped a record or double-counted a triple would not fail loudly,
// it would just quietly change the input every downstream arm reads.
func TestMergeInto(t *testing.T) {
	dir := t.TempDir()
	prevPath := filepath.Join(dir, "pass1.json")

	prev := artifact{
		Kind:         "memora/consolidation-triples/v1",
		RunUTC:       "2026-08-31T21:27:02Z",
		CLIVersion:   "2.1.252",
		Model:        "sonnet",
		Turns:        20,
		Instances:    2,
		Clusters:     2,
		CLICalls:     10,
		CLIFailures:  3,
		TotalTriples: 4,
		LLMWallClock: "1h0m0s",
		// No Passes: the legacy, single-run shape mergeInto must reconstruct.
		Users: []userResult{
			{UserID: "A", Turns: 12, Rounds: 2, Remaining: 5},
			{UserID: "B", Turns: 8, Rounds: 1, Remaining: 0},
		},
		Records: []*clusterRecord{
			{Seq: 1, UserID: "A", Gist: "g1", Triples: make([]models.Triple, 1)},
			{Seq: 2, UserID: "B", Gist: "g2", Triples: make([]models.Triple, 3)},
		},
	}
	blob, err := json.MarshalIndent(prev, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prevPath, blob, 0o444); err != nil {
		t.Fatal(err)
	}

	art := artifact{
		Kind:         "memora/consolidation-triples/v1",
		RunUTC:       "2026-09-01T03:00:00Z",
		Turns:        12,
		Instances:    1,
		Clusters:     1,
		CLICalls:     5,
		CLIFailures:  0,
		TotalTriples: 3,
		LLMWallClock: "10m0s",
		Passes:       []passInfo{{Pass: 1, StartedUTC: "2026-09-01T03:00:00Z"}},
		Users:        []userResult{{UserID: "A", Turns: 12, Rounds: 1, Remaining: 0, Passes: []int{1}}},
		Records:      []*clusterRecord{{Seq: 1, UserID: "A", Gist: "g3", Triples: make([]models.Triple, 3)}},
	}
	if err := mergeInto(&art, prevPath); err != nil {
		t.Fatal(err)
	}

	if got := len(art.Records); got != 3 {
		t.Errorf("records = %d, want 3 (2 kept + 1 new)", got)
	}
	if art.Records[0].Gist != "g1" || art.Records[2].Gist != "g3" {
		t.Errorf("earlier records must come first and verbatim, got %q..%q", art.Records[0].Gist, art.Records[2].Gist)
	}
	if art.Records[0].Pass != 1 || art.Records[2].Pass != 2 {
		t.Errorf("record pass tags = %d,%d, want 1,2", art.Records[0].Pass, art.Records[2].Pass)
	}
	if art.TotalTriples != 7 {
		t.Errorf("total_triples = %d, want 7", art.TotalTriples)
	}
	if art.CLICalls != 15 || art.CLIFailures != 3 {
		t.Errorf("cli calls/failures = %d/%d, want 15/3", art.CLICalls, art.CLIFailures)
	}
	if art.Instances != 2 || art.Turns != 20 {
		t.Errorf("instances/turns = %d/%d, want 2/20 (no double count)", art.Instances, art.Turns)
	}
	if art.Clusters != 3 {
		t.Errorf("clusters_with_a_gist = %d, want 3", art.Clusters)
	}
	if len(art.Passes) != 2 || art.Passes[0].Pass != 1 || art.Passes[1].Pass != 2 {
		t.Fatalf("passes = %+v, want a reconstructed pass 1 then pass 2", art.Passes)
	}
	if art.Passes[0].CLIFailures != 3 || art.Passes[0].ModelAlias != "sonnet" {
		t.Errorf("reconstructed pass 1 lost the header it came from: %+v", art.Passes[0])
	}
	if art.RunUTC != "2026-08-31T21:27:02Z" {
		t.Errorf("run_started_utc = %q, want the FIRST pass's start", art.RunUTC)
	}
	if !strings.Contains(art.Provenance, "PASSES") || art.MergedFrom == "" {
		t.Errorf("a merged artifact must say so: provenance=%q merged_from=%q", art.Provenance, art.MergedFrom)
	}
	if art.LLMWallClock != "1h10m0s (summed over passes)" {
		t.Errorf("llm_wall_clock = %q, want the sum", art.LLMWallClock)
	}

	var a, b *userResult
	for i := range art.Users {
		switch art.Users[i].UserID {
		case "A":
			a = &art.Users[i]
		case "B":
			b = &art.Users[i]
		}
	}
	if a == nil || b == nil {
		t.Fatalf("users = %+v, want A and B", art.Users)
	}
	if a.Rounds != 3 || a.Remaining != 0 {
		t.Errorf("A rounds/pending = %d/%d, want 3/0 (rounds summed, pending from the later pass)", a.Rounds, a.Remaining)
	}
	if len(a.Passes) != 2 || a.Passes[0] != 1 || a.Passes[1] != 2 {
		t.Errorf("A passes = %v, want [1 2]", a.Passes)
	}
	if len(b.Passes) != 1 || b.Passes[0] != 1 || b.Rounds != 1 {
		t.Errorf("B was not in the top-up and must be untouched, got %+v", *b)
	}
}

// TestMergeIntoAppendsUnseenUser covers the path where the top-up names an
// instance the earlier artifact never had: the append must not lose the merge
// bookkeeping for the users already in the slice.
func TestMergeIntoAppendsUnseenUser(t *testing.T) {
	dir := t.TempDir()
	prevPath := filepath.Join(dir, "pass1.json")
	prev := artifact{
		Users:   []userResult{{UserID: "A", Turns: 5, Rounds: 1}},
		Records: []*clusterRecord{{Seq: 1, UserID: "A"}},
	}
	blob, _ := json.MarshalIndent(prev, "", "  ")
	if err := os.WriteFile(prevPath, blob, 0o644); err != nil {
		t.Fatal(err)
	}

	art := artifact{
		Passes: []passInfo{{Pass: 1}},
		Users: []userResult{
			{UserID: "A", Turns: 5, Rounds: 2},
			{UserID: "Z", Turns: 7, Rounds: 1},
		},
		Records: []*clusterRecord{{Seq: 1, UserID: "Z"}},
	}
	if err := mergeInto(&art, prevPath); err != nil {
		t.Fatal(err)
	}
	if len(art.Users) != 2 || art.Turns != 12 {
		t.Fatalf("users=%d turns=%d, want 2/12", len(art.Users), art.Turns)
	}
	for _, u := range art.Users {
		switch u.UserID {
		case "A":
			if u.Rounds != 3 || len(u.Passes) != 2 {
				t.Errorf("A merged wrong after an append reallocated the slice: %+v", u)
			}
		case "Z":
			if len(u.Passes) != 1 || u.Passes[0] != 2 {
				t.Errorf("Z should be pass 2 only: %+v", u)
			}
		}
	}
}
