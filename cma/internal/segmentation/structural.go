package segmentation

import (
	"context"
	"regexp"
	"strings"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
)

// StructuralSegmenter packs raw text into episodes at sentence boundaries,
// targeting between minTokens and maxTokens per episode.
//
// Replaces the surprisal-based design in surprisal.go (kept in git history,
// not in the tree): GetTokenProbabilities requested logprobs at MaxTokens:1,
// so the live path only ever saw the model's own one-token reply, never the
// user's text -- every real ingest call silently discarded its input
// (proven by surprisal_test.go's TestOneTokenPath_LiveOpenAIShape). No chat
// completions API returns per-input-token logprobs, so that design could
// not be rescued; this is a deterministic replacement instead of a patch.
type StructuralSegmenter struct {
	llmProvider llm.Provider
	minTokens   int
	maxTokens   int
}

// NewStructuralSegmenter creates a new deterministic segmentation engine.
func NewStructuralSegmenter(provider llm.Provider, cfg configs.SegmentationConfig) *StructuralSegmenter {
	return &StructuralSegmenter{
		llmProvider: provider,
		minTokens:   cfg.MinEpisodeTokens,
		maxTokens:   cfg.MaxEpisodeTokens,
	}
}

// sentenceEnd matches a run of sentence-ending punctuation followed by
// whitespace or end of string.
var sentenceEnd = regexp.MustCompile(`[.!?]+(?:\s+|$)`)

// splitSentences breaks text into sentence-ish chunks on ./!/? boundaries.
// Deterministic, no model call, no network.
func splitSentences(text string) []string {
	locs := sentenceEnd.FindAllStringIndex(text, -1)
	var out []string
	last := 0
	for _, loc := range locs {
		if s := strings.TrimSpace(text[last:loc[1]]); s != "" {
			out = append(out, s)
		}
		last = loc[1]
	}
	if s := strings.TrimSpace(text[last:]); s != "" {
		out = append(out, s)
	}
	return out
}

// approxTokens mirrors OpenAIProvider.CountTokens (openai.go:326): chars/4,
// minimum 1 for non-empty input.
func approxTokens(text string) int {
	return tokensForChars(len(text))
}

// tokensForChars applies the chars/4 rule to a character count directly,
// so running totals can be tracked without re-materializing strings.
func tokensForChars(chars int) int {
	n := chars / 4
	if n == 0 && chars > 0 {
		n = 1
	}
	return n
}

// Segment packs text into episodes at sentence boundaries. A boundary is
// forced once the running episode has at least minTokens and the next
// sentence would push it past maxTokens; a single sentence longer than
// maxTokens is emitted alone rather than split mid-sentence.
func (s *StructuralSegmenter) Segment(ctx context.Context, userID string, text string) ([]models.Episode, error) {
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return nil, nil
	}

	var episodes []models.Episode
	var current []string
	currentChars := 0 // == len(strings.Join(current, " ")), tracked incrementally

	flush := func() error {
		if len(current) == 0 {
			return nil
		}
		content := strings.Join(current, " ")
		ep, err := s.createEpisode(ctx, userID, content)
		if err != nil {
			return err
		}
		episodes = append(episodes, *ep)
		current = nil
		currentChars = 0
		return nil
	}

	for _, sent := range sentences {
		// Chars this sentence would add if joined onto the running episode
		// (a separating space is only needed once current is non-empty) --
		// this must match strings.Join exactly, since approxTokens is
		// computed from character length, not from summed per-sentence
		// token counts (which drift from the real count once separators
		// and floor-division truncation accumulate over many sentences).
		addChars := len(sent)
		if len(current) > 0 {
			addChars++ // the join separator
		}
		currentTokens := tokensForChars(currentChars)
		wouldBeTokens := tokensForChars(currentChars + addChars)

		if currentTokens >= s.minTokens && wouldBeTokens > s.maxTokens {
			if err := flush(); err != nil {
				return nil, err
			}
			addChars = len(sent) // current is now empty, no separator needed
		}

		current = append(current, sent)
		currentChars += addChars

		if tokensForChars(currentChars) >= s.maxTokens {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}

	return episodes, nil
}

// createEpisode builds an Episode with embedding. Surprisal is gone (dead
// machinery, see above), so importance/surprisal start at a neutral 0
// rather than a fabricated number; DIG's heuristic fallback (dig.go) no
// longer weights on it.
func (s *StructuralSegmenter) createEpisode(ctx context.Context, userID string, content string) (*models.Episode, error) {
	embedding, err := s.llmProvider.Embed(ctx, content)
	if err != nil {
		return nil, err
	}
	ep := models.NewEpisode(userID, content, embedding, 0)
	ep.TokenCount = s.llmProvider.CountTokens(content)
	return ep, nil
}
