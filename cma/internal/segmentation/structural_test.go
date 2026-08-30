package segmentation

import (
	"context"
	"strings"
	"testing"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
)

// stubProvider is a network-free test double for llm.Provider. Only Embed
// and CountTokens are exercised by StructuralSegmenter.
type stubProvider struct{}

var _ llm.Provider = (*stubProvider)(nil)

func (s *stubProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

func (s *stubProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3, 0.4}
	}
	return out, nil
}

func (s *stubProvider) GetTokenProbabilities(ctx context.Context, text string) ([]llm.TokenProb, error) {
	return nil, nil // unused: the new segmenter never calls this
}

func (s *stubProvider) ExtractTriples(ctx context.Context, content string) ([]models.Triple, error) {
	return nil, nil
}

func (s *stubProvider) Synthesize(ctx context.Context, episodes []models.Episode) (string, error) {
	return "", nil
}

func (s *stubProvider) ScoreDIG(ctx context.Context, query, document string) (float64, error) {
	return 0, nil
}

func (s *stubProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (s *stubProvider) CountTokens(text string) int {
	return approxTokens(text)
}

func prodConfig() configs.SegmentationConfig {
	return configs.SegmentationConfig{
		Gamma:            2.5, // unused by StructuralSegmenter; kept for config-struct parity
		WindowSize:       50,  // unused
		MinEpisodeTokens: 50,
		MaxEpisodeTokens: 500,
	}
}

var sentencePool = []string{
	"User: I have been trying to reset my password since yesterday.",
	"Agent: I understand, let me pull up the account details right now.",
	"User: The confirmation email never arrived in my inbox or the spam folder.",
	"Agent: That is unusual, our delivery dashboard reports no failures today.",
	"User: Can you check whether the address on file is still correct?",
	"Agent: It shows an old corporate domain that was decommissioned in March.",
	"User: Oh! That explains every one of the missing notifications.",
	"Agent: Exactly. I will update the record and resend the verification link.",
}

func transcript(target int) string {
	var sb strings.Builder
	words := 0
	for i := 0; words < target; i++ {
		s := sentencePool[i%len(sentencePool)]
		sb.WriteString(s)
		sb.WriteString(" ")
		words += len(strings.Fields(s))
	}
	return strings.TrimSpace(sb.String())
}

// TestSegmentPreservesInput is the regression test for the bug fixed here.
//
// The deleted surprisal.go (see git history, commit before this one, and
// surprisal_test.go's TestOneTokenPath_LiveOpenAIShape) called
// GetTokenProbabilities on the live path, which OpenAI's chat-completions
// API answers with MaxTokens:1 -- exactly one GENERATED token, never the
// user's own text. That test measured 2,008 words in, 1 word out (the
// model's own reply). This test proves the opposite property against the
// code that replaced it: every word of a large input survives into some
// episode's content.
func TestSegmentPreservesInput(t *testing.T) {
	input := transcript(2000)
	inputWords := strings.Fields(input)

	eng := NewStructuralSegmenter(&stubProvider{}, prodConfig())
	eps, err := eng.Segment(context.Background(), "user-a", input)
	if err != nil {
		t.Fatalf("Segment returned error: %v", err)
	}
	if len(eps) == 0 {
		t.Fatal("expected at least one episode")
	}

	var reassembled strings.Builder
	for i, ep := range eps {
		if i > 0 {
			reassembled.WriteString(" ")
		}
		reassembled.WriteString(ep.Content)
	}
	outputWords := strings.Fields(reassembled.String())

	if len(outputWords) != len(inputWords) {
		t.Fatalf("word count changed: input had %d words, episodes together have %d "+
			"(the old bug: input had 2008 words, output had 1)", len(inputWords), len(outputWords))
	}
	for i := range inputWords {
		if inputWords[i] != outputWords[i] {
			t.Fatalf("word %d differs: input %q, reassembled %q", i, inputWords[i], outputWords[i])
		}
	}

	probe := "decommissioned"
	if !strings.Contains(input, probe) {
		t.Fatalf("test bug: probe %q not in generated transcript", probe)
	}
	if !strings.Contains(reassembled.String(), probe) {
		t.Fatalf("DID NOT FIX THE BUG: probe word %q from the input is missing from episode content", probe)
	}

	t.Logf("input: %d words -> %d episodes, %d words reassembled, 100%% preserved",
		len(inputWords), len(eps), len(outputWords))
}

// TestSegmentRespectsTokenBounds checks episodes land near [minTokens,
// maxTokens] rather than one giant or thousands of tiny ones.
func TestSegmentRespectsTokenBounds(t *testing.T) {
	cfg := prodConfig()
	eng := NewStructuralSegmenter(&stubProvider{}, cfg)

	input := transcript(3000)
	eps, err := eng.Segment(context.Background(), "user-b", input)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(eps) < 2 {
		t.Fatalf("expected multiple episodes from a 3000-word transcript, got %d", len(eps))
	}

	for i, ep := range eps {
		tc := approxTokens(ep.Content)
		isLast := i == len(eps)-1
		if tc > cfg.MaxEpisodeTokens {
			t.Errorf("episode[%d]: %d tokens exceeds max %d", i, tc, cfg.MaxEpisodeTokens)
		}
		if !isLast && tc < cfg.MinEpisodeTokens {
			t.Errorf("episode[%d]: %d tokens under min %d (non-final episode)", i, tc, cfg.MinEpisodeTokens)
		}
		t.Logf("episode[%d]: ~%d tokens", i, tc)
	}
}

// TestSegmentEmptyInput checks the boundary case the old singleEpisode
// fallback used to special-case.
func TestSegmentEmptyInput(t *testing.T) {
	eng := NewStructuralSegmenter(&stubProvider{}, prodConfig())
	eps, err := eng.Segment(context.Background(), "user-c", "")
	if err != nil {
		t.Fatalf("Segment(\"\"): %v", err)
	}
	if len(eps) != 0 {
		t.Fatalf("expected 0 episodes for empty input, got %d", len(eps))
	}
}

// TestSegmentDoesNotSplitMidSentence checks a single sentence longer than
// maxTokens is still emitted whole rather than truncated or errored on.
func TestSegmentDoesNotSplitMidSentence(t *testing.T) {
	cfg := prodConfig()
	eng := NewStructuralSegmenter(&stubProvider{}, cfg)

	longSentence := "This is one very long sentence that keeps going " +
		strings.Repeat("with more words ", 200) + "and finally ends."
	if approxTokens(longSentence) <= cfg.MaxEpisodeTokens {
		t.Fatalf("test bug: sentence is not actually longer than max (%d tokens)", approxTokens(longSentence))
	}

	eps, err := eng.Segment(context.Background(), "user-d", longSentence)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(eps) != 1 {
		t.Fatalf("expected the oversized sentence to be its own single episode, got %d episodes", len(eps))
	}
	if eps[0].Content != longSentence {
		t.Fatalf("sentence was altered:\n got: %q\nwant: %q", eps[0].Content, longSentence)
	}
}
