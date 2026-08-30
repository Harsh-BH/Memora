package eval

import (
	"math"
	"strings"
	"unicode"
)

// tokenizeSimple lowercases and splits on runs of non-alphanumeric
// characters. Deliberately simpler than the WordPiece tokenizer the
// embedding model uses (llm.wordpieceTokenizer) -- BM25 is a whole-word
// lexical baseline, not a subword model.
func tokenizeSimple(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

// BM25 implements Okapi BM25 (Robertson & Sparck Jones), the standard
// lexical retrieval baseline, with the conventional k1=1.5, b=0.75. Plain
// Go, no dependency -- stdlib string/unicode/math only.
type BM25 struct {
	k1, b     float64
	docs      [][]string     // tokenized documents, corpus order
	docLen    []int
	avgDocLen float64
	df        map[string]int // document frequency per term
	n         int
}

// NewBM25 indexes documents (in the same order the caller will interpret
// ScoreAll's results).
func NewBM25(documents []string) *BM25 {
	m := &BM25{k1: 1.5, b: 0.75, df: make(map[string]int)}
	m.n = len(documents)
	totalLen := 0
	for _, d := range documents {
		toks := tokenizeSimple(d)
		m.docs = append(m.docs, toks)
		m.docLen = append(m.docLen, len(toks))
		totalLen += len(toks)
		seen := make(map[string]bool, len(toks))
		for _, t := range toks {
			if !seen[t] {
				seen[t] = true
				m.df[t]++
			}
		}
	}
	if m.n > 0 {
		m.avgDocLen = float64(totalLen) / float64(m.n)
	}
	return m
}

// idf uses the standard BM25 inverse document frequency with +1 smoothing,
// which keeps the value non-negative even for terms in every document.
func (m *BM25) idf(term string) float64 {
	n := float64(m.n)
	dfT := float64(m.df[term])
	return math.Log(1 + (n-dfT+0.5)/(dfT+0.5))
}

// ScoreAll returns a BM25 score for query against every indexed document,
// in the same corpus order NewBM25 was given.
func (m *BM25) ScoreAll(query string) []float64 {
	qTerms := tokenizeSimple(query)
	scores := make([]float64, m.n)
	for i, doc := range m.docs {
		tf := make(map[string]int, len(doc))
		for _, t := range doc {
			tf[t]++
		}
		var score float64
		dl := float64(m.docLen[i])
		for _, term := range qTerms {
			f := float64(tf[term])
			if f == 0 {
				continue
			}
			num := f * (m.k1 + 1)
			den := f + m.k1*(1-m.b+m.b*dl/m.avgDocLen)
			score += m.idf(term) * num / den
		}
		scores[i] = score
	}
	return scores
}

// rankedIndices returns document indices sorted by descending score,
// breaking ties by lower index (stable, deterministic).
func rankedIndices(scores []float64) []int {
	idx := make([]int, len(scores))
	for i := range idx {
		idx[i] = i
	}
	// simple insertion sort: n is at most a few hundred here, and stability
	// matters more than asymptotic speed for a reproducible eval.
	for i := 1; i < len(idx); i++ {
		j := i
		for j > 0 && scores[idx[j-1]] < scores[idx[j]] {
			idx[j-1], idx[j] = idx[j], idx[j-1]
			j--
		}
	}
	return idx
}
