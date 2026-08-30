package llm

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

// wordpieceTokenizer is a minimal BERT-style WordPiece tokenizer, loaded
// from a vocab.txt (one token per line, line number == token id). This is
// the tokenizer all-MiniLM-L6-v2 (and BERT-family models generally) expect;
// implemented directly against vocab.txt rather than pulling in a tokenizer
// library, since the algorithm is small, fixed, and well documented.
//
// Simplification: no special CJK character splitting (tokenize_chinese_chars
// in the HF tokenizer config). The eval corpus here is English text, so this
// does not affect results, but a mixed-script corpus would need it added.
type wordpieceTokenizer struct {
	vocab    map[string]int64
	unkID    int64
	clsID    int64
	sepID    int64
	padID    int64
	maxChars int // WordPiece gives up on a basic-token longer than this
}

func newWordpieceTokenizer(vocabPath string) (*wordpieceTokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vocab := make(map[string]int64)
	scanner := bufio.NewScanner(f)
	var id int64
	for scanner.Scan() {
		tok := scanner.Text()
		vocab[tok] = id
		id++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	t := &wordpieceTokenizer{vocab: vocab, maxChars: 200}
	t.unkID = vocab["[UNK]"]
	t.clsID = vocab["[CLS]"]
	t.sepID = vocab["[SEP]"]
	t.padID = vocab["[PAD]"]
	return t, nil
}

// basicTokenize lowercases and splits into words and standalone punctuation,
// matching BertTokenizer's BasicTokenizer with do_lower_case=true.
func basicTokenize(text string) []string {
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
		switch {
		case unicode.IsSpace(r):
			flush()
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			flush()
			tokens = append(tokens, string(r))
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// wordpieceOne greedily matches the longest vocab entry from the start of
// word, marking continuations with "##", per the standard WordPiece
// algorithm. Returns [UNK] if any piece can't be matched.
func (t *wordpieceTokenizer) wordpieceOne(word string) []int64 {
	if len(word) > t.maxChars {
		return []int64{t.unkID}
	}
	runes := []rune(word)
	var out []int64
	start := 0
	for start < len(runes) {
		end := len(runes)
		var matchID int64 = -1
		for end > start {
			piece := string(runes[start:end])
			if start > 0 {
				piece = "##" + piece
			}
			if id, ok := t.vocab[piece]; ok {
				matchID = id
				break
			}
			end--
		}
		if matchID == -1 {
			return []int64{t.unkID}
		}
		out = append(out, matchID)
		start = end
	}
	return out
}

// Encode tokenizes text into [CLS] ... [SEP] ids padded/truncated to
// maxSeqLen, plus the corresponding attention mask. token_type_ids is all
// zeros (single-sequence input), which the caller can allocate directly.
func (t *wordpieceTokenizer) Encode(text string, maxSeqLen int) (ids []int64, mask []int64) {
	var pieces []int64
	for _, w := range basicTokenize(text) {
		pieces = append(pieces, t.wordpieceOne(w)...)
	}

	budget := maxSeqLen - 2 // room for [CLS] and [SEP]
	if len(pieces) > budget {
		pieces = pieces[:budget]
	}

	ids = make([]int64, maxSeqLen)
	mask = make([]int64, maxSeqLen)
	ids[0] = t.clsID
	mask[0] = 1
	i := 1
	for _, p := range pieces {
		ids[i] = p
		mask[i] = 1
		i++
	}
	ids[i] = t.sepID
	mask[i] = 1
	i++
	for ; i < maxSeqLen; i++ {
		ids[i] = t.padID
		mask[i] = 0
	}
	return ids, mask
}

// CountTokens returns the real WordPiece token count (CLS + pieces + SEP),
// uncapped by maxSeqLen -- for reporting, not for building model input.
func (t *wordpieceTokenizer) CountTokens(text string) int {
	n := 2 // [CLS] + [SEP]
	for _, w := range basicTokenize(text) {
		n += len(t.wordpieceOne(w))
	}
	return n
}
