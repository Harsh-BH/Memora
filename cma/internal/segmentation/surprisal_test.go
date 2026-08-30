package segmentation

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
)

// ---------------------------------------------------------------------------
// Test double for llm.Provider.
//
// No network, no API key. GetTokenProbabilities is the only interesting method;
// everything else returns a fixed value so createEpisode() can run.
// ---------------------------------------------------------------------------

type stubProvider struct {
	probs func(text string) ([]llm.TokenProb, error)
}

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
	return s.probs(text)
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

// CountTokens mirrors OpenAIProvider.CountTokens exactly (openai.go:326).
func (s *stubProvider) CountTokens(text string) int {
	count := len(text) / 4
	if count == 0 && len(text) > 0 {
		count = 1
	}
	return count
}

// ---------------------------------------------------------------------------
// Exact replica of OpenAIProvider.syntheticTokenProbs (openai.go:128-163).
// Copied line for line so the measured numbers transfer to production.
// ---------------------------------------------------------------------------

func syntheticTokenProbs(text string) []llm.TokenProb {
	words := strings.Fields(text)
	probs := make([]llm.TokenProb, 0, len(words))

	for i, word := range words {
		logProb := -1.0 // baseline

		if i > 0 {
			prev := words[i-1]
			if strings.HasSuffix(prev, ".") || strings.HasSuffix(prev, "!") || strings.HasSuffix(prev, "?") {
				logProb = -4.0 // high surprisal after sentence boundary
			}
		}

		if len(word) > 8 {
			logProb -= 1.5
		}

		if strings.ContainsAny(word, "?!") {
			logProb -= 2.0
		}

		probs = append(probs, llm.TokenProb{Token: word, LogProb: logProb, Offset: i})
	}

	return probs
}

func fallbackStub() *stubProvider {
	return &stubProvider{probs: func(text string) ([]llm.TokenProb, error) {
		return syntheticTokenProbs(text), nil
	}}
}

// prodConfig is the literal configs/config.yaml segmentation block (lines 34-38).
func prodConfig() configs.SegmentationConfig {
	return configs.SegmentationConfig{
		Gamma:            2.5,
		WindowSize:       50,
		MinEpisodeTokens: 50,
		MaxEpisodeTokens: 500,
	}
}

// ---------------------------------------------------------------------------
// Deterministic transcript generator. No rand, no time, no maps.
// ---------------------------------------------------------------------------

var sentencePool = []string{
	"User: I have been trying to reset my password since yesterday.",
	"Agent: I understand, let me pull up the account details right now.",
	"User: The confirmation email never arrived in my inbox or the spam folder.",
	"Agent: That is unusual, our delivery dashboard reports no failures today.",
	"User: Can you check whether the address on file is still correct?",
	"Agent: It shows an old corporate domain that was decommissioned in March.",
	"User: Oh! That explains every one of the missing notifications.",
	"Agent: Exactly. I will update the record and resend the verification link.",
	"User: How long does propagation usually take on your side?",
	"Agent: Roughly ten minutes for authentication and about an hour for billing.",
	"User: Meanwhile the subscription renewal charged my card twice last week.",
	"Agent: I see two authorizations, one of which is only a pending hold.",
	"User: Will that hold drop off automatically or do I file a dispute?",
	"Agent: It expires in five business days without any action from you.",
	"User: Good. Please also downgrade the plan before the next cycle.",
	"Agent: Done, the workspace moves to the standard tier on the first.",
	"User: One more thing, the export job keeps timing out at ninety percent.",
	"Agent: That is a known issue with large attachment bundles this month.",
	"User: Is there a workaround I can run tonight?",
	"Agent: Split the range into two halves and export each separately.",
}

// transcript builds a deterministic multi-paragraph transcript of at least
// target whitespace words, starting from a given offset in the pool.
func transcript(start, target int) string {
	var sb strings.Builder
	words := 0
	for i := 0; words < target; i++ {
		s := sentencePool[(start+i)%len(sentencePool)]
		sb.WriteString(s)
		if (i+1)%4 == 0 {
			sb.WriteString("\n\n")
		} else {
			sb.WriteString(" ")
		}
		words += len(strings.Fields(s))
	}
	return strings.TrimSpace(sb.String())
}

// ---------------------------------------------------------------------------
// (a) THE ONE-TOKEN PATH.
//
// openai.go:95 sends MaxTokens: 1. The OpenAI chat API returns logprobs for
// GENERATED tokens only, so choice.LogProbs.Content has exactly one entry, and
// that entry is the model's own first output token -- not the user's text.
// This test reproduces that shape and records what the segmenter does with it.
// ---------------------------------------------------------------------------

func TestOneTokenPath_LiveOpenAIShape(t *testing.T) {
	input := transcript(0, 2000)
	inputWords := len(strings.Fields(input))

	// Exactly what MaxTokens:1 + LogProbs yields: one generated token.
	oneToken := &stubProvider{probs: func(text string) ([]llm.TokenProb, error) {
		return []llm.TokenProb{{Token: "I", LogProb: -0.1873, Offset: 0}}, nil
	}}

	eng := NewSurprisalEngine(oneToken, prodConfig())
	eps, err := eng.Segment(context.Background(), "user-a", input)
	if err != nil {
		t.Fatalf("Segment returned error: %v", err)
	}

	t.Logf("input: %d words, %d chars", inputWords, len(input))
	t.Logf("episodes returned: %d", len(eps))
	for i, ep := range eps {
		t.Logf("  episode[%d]: TokenCount=%d ContentLen=%d Content=%q SurprisalValue=%.4f",
			i, ep.TokenCount, len(ep.Content), ep.Content, ep.SurprisalValue)
	}

	if len(eps) != 1 {
		t.Fatalf("expected 1 episode from a 1-token logprob response, got %d", len(eps))
	}

	// The decisive assertion: does ANY of the user's transcript survive?
	probe := "decommissioned"
	if !strings.Contains(input, probe) {
		t.Fatalf("test bug: probe %q not in generated transcript", probe)
	}
	survived := strings.Contains(eps[0].Content, probe)
	retained := float64(len(eps[0].Content)) / float64(len(input)) * 100

	t.Logf("input text survives into episode content: %v", survived)
	t.Logf("chars retained: %d of %d (%.4f%%)", len(eps[0].Content), len(input), retained)

	if survived {
		t.Fatalf("DOCUMENTED BEHAVIOUR CHANGED: input text now survives the one-token path")
	}
	if eps[0].Content != "I" {
		t.Fatalf("expected episode content to be the single GENERATED token %q, got %q", "I", eps[0].Content)
	}
	if eps[0].TokenCount != 1 {
		t.Fatalf("expected TokenCount 1, got %d", eps[0].TokenCount)
	}
	t.Logf("VERDICT: with MaxTokens:1 the segmenter emits 1 episode containing the model's "+
		"generated token only. %d words of user input are discarded, not segmented.", inputWords)
}

// TestErrorPathPreservesText documents the ONLY branch that keeps the user's
// text: Segment's err != nil fallback. Note that OpenAIProvider.
// GetTokenProbabilities never returns a non-nil error (openai.go:99-102 and
// 118-120 swallow every failure into syntheticTokenProbs), so in production
// this branch is unreachable.
func TestErrorPathPreservesText(t *testing.T) {
	input := transcript(0, 300)
	failing := &stubProvider{probs: func(text string) ([]llm.TokenProb, error) {
		return nil, fmt.Errorf("api down")
	}}

	eng := NewSurprisalEngine(failing, prodConfig())
	eps, err := eng.Segment(context.Background(), "user-err", input)
	if err != nil {
		t.Fatalf("Segment returned error: %v", err)
	}
	if len(eps) != 1 || eps[0].Content != input {
		t.Fatalf("error path should return the whole text as one episode")
	}
	// TokenCount here comes from CountTokens (chars/4), NOT from the loop.
	want := len(input) / 4
	t.Logf("error path: 1 episode, %d words, TokenCount=%d (chars/4=%d)",
		len(strings.Fields(input)), eps[0].TokenCount, want)
	if eps[0].TokenCount != want {
		t.Fatalf("expected TokenCount %d, got %d", want, eps[0].TokenCount)
	}
}

// ---------------------------------------------------------------------------
// (b) THE FALLBACK PATH: full episode length distribution.
// ---------------------------------------------------------------------------

func percentile(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func TestFallbackPath_EpisodeLengthDistribution(t *testing.T) {
	targets := []int{100, 250, 400, 600, 800, 1000, 1200, 1500, 1800, 2000, 2500, 3000}
	cfg := prodConfig()

	var all []int
	var atMax, belowMin int

	t.Logf("config: gamma=%.1f window=%d min=%d max=%d (configs/config.yaml:34-38)",
		cfg.Gamma, cfg.WindowSize, cfg.MinEpisodeTokens, cfg.MaxEpisodeTokens)
	t.Logf("%-4s %-8s %-9s %s", "#", "words_in", "episodes", "episode token counts")

	for i, target := range targets {
		text := transcript(i, target)
		words := len(strings.Fields(text))

		eng := NewSurprisalEngine(fallbackStub(), cfg)
		eps, err := eng.Segment(context.Background(), fmt.Sprintf("user-b-%d", i), text)
		if err != nil {
			t.Fatalf("transcript %d: %v", i, err)
		}

		counts := make([]int, 0, len(eps))
		sum := 0
		for _, ep := range eps {
			counts = append(counts, ep.TokenCount)
			sum += ep.TokenCount
			all = append(all, ep.TokenCount)
			if ep.TokenCount == cfg.MaxEpisodeTokens {
				atMax++
			}
			if ep.TokenCount < cfg.MinEpisodeTokens {
				belowMin++
			}
		}
		// Conservation: every input word lands in exactly one episode.
		if sum != words {
			t.Fatalf("transcript %d: token counts sum to %d, input had %d words", i, sum, words)
		}
		t.Logf("%-4d %-8d %-9d %v", i, words, len(eps), counts)
	}

	sorted := append([]int(nil), all...)
	sort.Ints(sorted)

	t.Logf("--- episode length distribution (n=%d episodes over %d transcripts) ---", len(sorted), len(targets))
	t.Logf("count   = %d", len(sorted))
	t.Logf("min     = %d", sorted[0])
	t.Logf("median  = %d (lower median, element n/2)", sorted[len(sorted)/2])
	t.Logf("p95     = %d (ceil(0.95n)-1)", percentile(sorted, 0.95))
	t.Logf("max     = %d", sorted[len(sorted)-1])
	t.Logf("below min_episode_tokens(%d) = %d of %d (%.1f%%)",
		cfg.MinEpisodeTokens, belowMin, len(sorted), 100*float64(belowMin)/float64(len(sorted)))
	t.Logf("exactly max_episode_tokens(%d) = %d of %d (%.1f%%)",
		cfg.MaxEpisodeTokens, atMax, len(sorted), 100*float64(atMax)/float64(len(sorted)))

	// How many boundaries does the SURPRISAL RULE itself produce, with the
	// max-length force removed? Anything left is genuinely surprise-driven.
	noForce := cfg
	noForce.MaxEpisodeTokens = 1 << 30
	surprisalBoundaries := 0
	totalEpisodesNoForce := 0
	for i, target := range targets {
		text := transcript(i, target)
		eng := NewSurprisalEngine(fallbackStub(), noForce)
		eps, err := eng.Segment(context.Background(), fmt.Sprintf("user-nf-%d", i), text)
		if err != nil {
			t.Fatalf("transcript %d (no force): %v", i, err)
		}
		surprisalBoundaries += len(eps) - 1
		totalEpisodesNoForce += len(eps)
	}
	t.Logf("--- with max-length force disabled (max=%d) ---", noForce.MaxEpisodeTokens)
	t.Logf("episodes = %d over the same %d transcripts", totalEpisodesNoForce, len(targets))
	t.Logf("surprisal-driven boundaries = %d", surprisalBoundaries)

	if len(sorted) == 0 {
		t.Fatal("no episodes produced")
	}
	// Characterization lock: as measured, the S > mu + gamma*sigma rule produces
	// no boundaries at all on these transcripts. Every boundary above came from
	// the max-length force. If this ever stops being true, the number in the CV
	// changes and this test must be re-read, not deleted.
	if surprisalBoundaries != 0 {
		t.Fatalf("MEASUREMENT CHANGED: surprisal rule produced %d boundaries, previously 0", surprisalBoundaries)
	}
}

// ---------------------------------------------------------------------------
// (c) THE UNIT QUESTION.
//
// From the code: episodes emitted inside the loop get ep.TokenCount = tokenCount
// (surprisal.go:178, 207), which is the number of llm.TokenProb entries consumed.
// On the fallback path a TokenProb is one strings.Fields WORD, so TokenCount is a
// whitespace word count, not a model token count. createEpisode's chars/4 value
// (surprisal.go:222) is overwritten and only survives on the singleEpisode path.
// ---------------------------------------------------------------------------

func TestTokenCountUnit_IsWhitespaceWordsNotModelTokens(t *testing.T) {
	text := strings.TrimSpace(strings.Repeat("alpha ", 1200))
	eng := NewSurprisalEngine(fallbackStub(), prodConfig())
	eps, err := eng.Segment(context.Background(), "user-c", text)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}

	stub := fallbackStub()
	for i, ep := range eps {
		words := len(strings.Fields(ep.Content))
		chars4 := stub.CountTokens(ep.Content)
		t.Logf("episode[%d]: TokenCount=%d  words(Fields)=%d  CountTokens(chars/4)=%d  ratio=%.3f",
			i, ep.TokenCount, words, chars4, float64(chars4)/float64(ep.TokenCount))
		if ep.TokenCount != words {
			t.Fatalf("episode[%d]: TokenCount %d != whitespace words %d", i, ep.TokenCount, words)
		}
		if ep.TokenCount == chars4 {
			t.Fatalf("episode[%d]: TokenCount coincides with chars/4 (%d); the two units are supposed to differ here", i, chars4)
		}
	}
	t.Logf("CONCLUSION: on the fallback path TokenCount counts whitespace words. " +
		"The same field on the singleEpisode path counts len(text)/4 characters. Two units, one field.")
}

func TestNoTokenizerInDependencyTree(t *testing.T) {
	for _, f := range []string{"../../go.mod", "../../go.sum"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		low := strings.ToLower(string(data))
		for _, needle := range []string{"tiktoken", "tokenizer", "sentencepiece"} {
			if strings.Contains(low, needle) {
				t.Fatalf("%s mentions %q -- a real tokenizer may now be present", f, needle)
			}
		}
		t.Logf("%s: no tiktoken/tokenizer/sentencepiece dependency", f)
	}
}

// ---------------------------------------------------------------------------
// (d) THE RESIDUAL EPISODE.
//
// surprisal.go:197-209 flushes the tail with no minTokens check, so the last
// episode of any Segment call can be arbitrarily short.
// ---------------------------------------------------------------------------

func TestResidualEpisodeIgnoresMinimum(t *testing.T) {
	cfg := prodConfig()

	// 501 unpunctuated words: one forced boundary at 500, then a 1-word tail.
	text := strings.TrimSpace(strings.Repeat("alpha ", 501))
	eng := NewSurprisalEngine(fallbackStub(), cfg)
	eps, err := eng.Segment(context.Background(), "user-d1", text)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	got := make([]int, len(eps))
	for i, ep := range eps {
		got[i] = ep.TokenCount
	}
	t.Logf("501 words -> %d episodes, token counts %v (min_episode_tokens=%d)", len(eps), got, cfg.MinEpisodeTokens)
	if len(eps) != 2 || got[0] != 500 || got[1] != 1 {
		t.Fatalf("expected [500 1], got %v", got)
	}
	if got[1] >= cfg.MinEpisodeTokens {
		t.Fatalf("residual episode unexpectedly respected the minimum")
	}

	// Whole input shorter than the minimum: still exactly one episode.
	short := "just three words"
	eng2 := NewSurprisalEngine(fallbackStub(), cfg)
	eps2, err := eng2.Segment(context.Background(), "user-d2", short)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	t.Logf("%q -> %d episode(s), TokenCount=%d", short, len(eps2), eps2[0].TokenCount)
	if len(eps2) != 1 || eps2[0].TokenCount != 3 {
		t.Fatalf("expected one 3-token episode, got %d episodes", len(eps2))
	}
	t.Logf("CONCLUSION: min_episode_tokens gates only in-loop boundaries. " +
		"The flushed tail is exempt, so 'episodes are at least 50 tokens' is false.")
}

// ---------------------------------------------------------------------------
// (e) Boundary behaviour: the sentence-end gate and the max-length force.
// ---------------------------------------------------------------------------

func TestSentenceEndGateSuppressesBoundary(t *testing.T) {
	cfg := prodConfig()
	base := strings.Repeat("alpha ", 60) + "stop. "

	// Case A: the high-surprisal word itself ends in '!' -> gate passes.
	withPunct := strings.TrimSpace(base + "Wow!")
	// Case B: identical position and prior context, no trailing punctuation.
	withoutPunct := strings.TrimSpace(base + "Wow")

	run := func(name, text string) []int {
		eng := NewSurprisalEngine(fallbackStub(), cfg)
		eps, err := eng.Segment(context.Background(), "user-e-"+name, text)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out := make([]int, len(eps))
		for i, ep := range eps {
			out[i] = ep.TokenCount
		}
		probs := syntheticTokenProbs(text)
		last := probs[len(probs)-1]
		t.Logf("%-14s words=%d last token=%q surprisal=%.1f -> %d episode(s) %v",
			name, len(probs), last.Token, -last.LogProb, len(eps), out)
		return out
	}

	a := run("ends-with-!", withPunct)
	b := run("no-punct", withoutPunct)

	if len(a) != 2 || a[0] != 61 || a[1] != 1 {
		t.Fatalf("expected the sentence-ending token to split into [61 1], got %v", a)
	}
	if len(b) != 1 || b[0] != 62 {
		t.Fatalf("expected the unpunctuated token to be suppressed into [62], got %v", b)
	}
	t.Logf("CONCLUSION: identical surprisal (6.0 vs 4.0, both above threshold), " +
		"only the token ending in sentence punctuation produces a boundary.")
}

func TestMaxLengthForceFires(t *testing.T) {
	cfg := prodConfig()
	// 1200 unpunctuated words: every surprisal is exactly 1.0, so sigma -> 0
	// and the threshold equals the value; no surprisal boundary can fire.
	text := strings.TrimSpace(strings.Repeat("alpha ", 1200))
	eng := NewSurprisalEngine(fallbackStub(), cfg)
	eps, err := eng.Segment(context.Background(), "user-e-max", text)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	got := make([]int, len(eps))
	for i, ep := range eps {
		got[i] = ep.TokenCount
	}
	t.Logf("1200 flat-surprisal words -> %d episodes %v", len(eps), got)
	if len(eps) != 3 || got[0] != 500 || got[1] != 500 || got[2] != 200 {
		t.Fatalf("expected [500 500 200], got %v", got)
	}

	// Same text with the force removed: one episode, no boundaries at all.
	noForce := cfg
	noForce.MaxEpisodeTokens = 1 << 30
	eng2 := NewSurprisalEngine(fallbackStub(), noForce)
	eps2, err := eng2.Segment(context.Background(), "user-e-nomax", text)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	t.Logf("same text, max-length force disabled -> %d episode(s) of %d tokens", len(eps2), eps2[0].TokenCount)
	if len(eps2) != 1 || eps2[0].TokenCount != 1200 {
		t.Fatalf("expected a single 1200-token episode, got %d episodes", len(eps2))
	}
}

// TestContentJoinLosesFormatting documents that episode Content is
// strings.Join(tokens, " ") (surprisal.go:172, 198), so paragraph structure
// present in the input does not survive into stored memory.
func TestContentJoinLosesFormatting(t *testing.T) {
	text := transcript(0, 120)
	if !strings.Contains(text, "\n") {
		t.Fatalf("test bug: generated transcript has no newlines")
	}
	eng := NewSurprisalEngine(fallbackStub(), prodConfig())
	eps, err := eng.Segment(context.Background(), "user-f", text)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	joined := strings.Join(func() []string {
		out := make([]string, len(eps))
		for i, ep := range eps {
			out[i] = ep.Content
		}
		return out
	}(), " ")
	t.Logf("input newlines=%d, output newlines=%d",
		strings.Count(text, "\n"), strings.Count(joined, "\n"))
	if strings.Contains(joined, "\n") {
		t.Fatalf("expected all newlines to be collapsed by strings.Join")
	}
	if strings.Join(strings.Fields(text), " ") != joined {
		t.Fatalf("expected word sequence to be preserved apart from whitespace")
	}
}
