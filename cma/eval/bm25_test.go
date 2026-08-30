package eval

import "testing"

// TestBM25RanksExactTermMatchFirst is BM25's own sanity check, independent
// of the embedding model or Qdrant: a query containing a term that appears
// in exactly one document must rank that document first.
func TestBM25RanksExactTermMatchFirst(t *testing.T) {
	docs := []string{
		"the quick brown fox jumps over the lazy dog",
		"a completely unrelated sentence about tax law",
		"another unrelated sentence about deep sea fish",
	}
	bm := NewBM25(docs)
	scores := bm.ScoreAll("fox jumps")
	ranked := rankedIndices(scores)
	if ranked[0] != 0 {
		t.Fatalf("expected doc 0 (contains 'fox jumps') to rank first, got doc %d (scores=%v)", ranked[0], scores)
	}
	if scores[1] != 0 || scores[2] != 0 {
		t.Fatalf("expected zero score for documents sharing no query terms, got %v", scores)
	}
}

func TestTokenizeSimple(t *testing.T) {
	got := tokenizeSimple("Hello, World! It's BM25.")
	want := []string{"hello", "world", "it", "s", "bm25"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
