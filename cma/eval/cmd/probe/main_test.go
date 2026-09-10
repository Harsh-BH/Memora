package main

import (
	"math"
	"testing"
)

// percentile feeds every number this probe reports, and its edge cases (empty
// input, q at the extremes, a single element) are exactly where a nearest-rank
// implementation goes quietly wrong. Checked against values counted by hand.
func TestPercentileNearestRank(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} // already sorted, as the callers guarantee
	for _, tc := range []struct{ q, want float64 }{
		{0.10, 1}, {0.25, 3}, {0.50, 5}, {0.75, 8}, {0.90, 9}, {1.0, 10}, {0.0, 1},
	} {
		if got := percentile(xs, tc.q); got != tc.want {
			t.Errorf("percentile(q=%.2f) = %v, want %v", tc.q, got, tc.want)
		}
	}
	if got := percentile([]float64{42}, 0.5); got != 42 {
		t.Errorf("single-element percentile = %v, want 42", got)
	}
	if got := percentile(nil, 0.5); !math.IsNaN(got) {
		t.Errorf("empty percentile = %v, want NaN -- reporting 0 for 'no data' is the failure mode "+
			"this probe exists to prevent", got)
	}
}

// overlap and isNumeric decide KU-strict membership, so a wrong answer there
// silently changes a published ID list.
func TestStrictHelpers(t *testing.T) {
	want := map[string]bool{"a": true, "b": true, "c": true, "d": true}
	if got := overlap(want, map[string]bool{"a": true, "b": true}); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("overlap = %v, want 0.5", got)
	}
	if got := overlap(map[string]bool{}, want); got != 0 {
		t.Errorf("overlap over an empty want set = %v, want 0 (not a divide by zero)", got)
	}
	for tok, want := range map[string]bool{"2023": true, "7": true, "": false, "3rd": false, "abc": false} {
		if got := isNumeric(tok); got != want {
			t.Errorf("isNumeric(%q) = %v, want %v", tok, got, want)
		}
	}
}
