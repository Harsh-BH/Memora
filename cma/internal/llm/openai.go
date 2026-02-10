package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
)

// OpenAIProvider implements Provider using the OpenAI API.
type OpenAIProvider struct {
	client         *openai.Client
	model          string
	embeddingModel string
	maxTokens      int
	temperature    float64
}

// NewOpenAIProvider creates a new OpenAI-backed LLM provider.
func NewOpenAIProvider(cfg configs.LLMConfig) *OpenAIProvider {
	client := openai.NewClient(cfg.APIKey)

	model := cfg.Model
	if model == "" {
		model = "gpt-4o"
	}
	embModel := cfg.EmbeddingModel
	if embModel == "" {
		embModel = "text-embedding-3-small"
	}

	return &OpenAIProvider{
		client:         client,
		model:          model,
		embeddingModel: embModel,
		maxTokens:      cfg.MaxTokens,
		temperature:    cfg.Temperature,
	}
}

// Embed generates a dense vector embedding for the given text.
func (o *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := o.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(o.embeddingModel),
		Input: []string{text},
	})
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai embed: no embedding returned")
	}

	return resp.Data[0].Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts in a single API call.
func (o *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := o.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(o.embeddingModel),
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("openai embed batch: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		embeddings[i] = d.Embedding
	}

	return embeddings, nil
}

// GetTokenProbabilities returns per-token log probabilities using the chat completions
// API with logprobs enabled. Used by the surprisal segmentation engine for
// Surprisal(x_t) = -log P(x_t | x_<t).
func (o *OpenAIProvider) GetTokenProbabilities(ctx context.Context, text string) ([]TokenProb, error) {
	logprobs := true
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: text,
			},
		},
		MaxTokens:   1,
		LogProbs:    logprobs,
		TopLogProbs: 1,
	})
	if err != nil {
		// Fallback: generate synthetic probabilities based on word frequency heuristic.
		return o.syntheticTokenProbs(text), nil
	}

	var probs []TokenProb
	if resp.Choices != nil && len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.LogProbs != nil {
			for _, tokenLP := range choice.LogProbs.Content {
				probs = append(probs, TokenProb{
					Token:   tokenLP.Token,
					LogProb: float64(tokenLP.LogProb),
				})
			}
		}
	}

	// If logprobs not available from the API, fall back to synthetic.
	if len(probs) == 0 {
		return o.syntheticTokenProbs(text), nil
	}

	return probs, nil
}

// syntheticTokenProbs generates heuristic-based token probabilities when
// real logprobs are unavailable. Uses sentence boundary and punctuation
// signals as a proxy for surprisal.
func (o *OpenAIProvider) syntheticTokenProbs(text string) []TokenProb {
	words := strings.Fields(text)
	probs := make([]TokenProb, 0, len(words))

	for i, word := range words {
		// Heuristic: shorter common words get higher probability (lower surprisal).
		// Sentence starters and words after punctuation get lower probability (higher surprisal).
		logProb := -1.0 // baseline

		// Increase surprisal at sentence boundaries.
		if i > 0 {
			prev := words[i-1]
			if strings.HasSuffix(prev, ".") || strings.HasSuffix(prev, "!") || strings.HasSuffix(prev, "?") {
				logProb = -4.0 // high surprisal after sentence boundary
			}
		}

		// Longer words tend to be less predictable.
		if len(word) > 8 {
			logProb -= 1.5
		}

		// Question marks and exclamation create surprise.
		if strings.ContainsAny(word, "?!") {
			logProb -= 2.0
		}

		probs = append(probs, TokenProb{
			Token:   word,
			LogProb: logProb,
			Offset:  i,
		})
	}

	return probs
}

// ExtractTriples extracts atomic (Subject, Predicate, Object) triples from text
// using structured LLM output during the consolidation Sleep cycle.
func (o *OpenAIProvider) ExtractTriples(ctx context.Context, content string) ([]models.Triple, error) {
	prompt := fmt.Sprintf(`Extract all factual relationships from the following text as atomic triples.
Return a JSON array where each element has:
- "subject": the entity performing or being described
- "predicate": the relationship or action
- "object": the target entity or value
- "confidence": a float between 0.0 and 1.0 indicating certainty

Only extract clearly stated facts. Do not infer or hallucinate relationships.
Return ONLY valid JSON, no markdown formatting.

Text:
%s`, content)

	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a precise knowledge extraction engine. Extract atomic facts as (Subject, Predicate, Object) triples with confidence scores. Return only valid JSON.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxTokens:   o.maxTokens,
		Temperature: float32(0.0), // deterministic extraction
	})
	if err != nil {
		return nil, fmt.Errorf("openai extract triples: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai extract triples: no response")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimSpace(raw)
	// Strip markdown code fences if present.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var triples []models.Triple
	if err := json.Unmarshal([]byte(raw), &triples); err != nil {
		return nil, fmt.Errorf("openai extract triples parse: %w (raw: %s)", err, raw)
	}

	return triples, nil
}

// Synthesize generates a gist proposition from a cluster of episodes.
func (o *OpenAIProvider) Synthesize(ctx context.Context, episodes []models.Episode) (string, error) {
	var sb strings.Builder
	for i, ep := range episodes {
		sb.WriteString(fmt.Sprintf("Episode %d (t=%s): %s\n", i+1, ep.Timestamp.Format("2006-01-02T15:04"), ep.Content))
	}

	prompt := fmt.Sprintf(`Synthesize the following episodic memory fragments into a single concise semantic proposition.
The proposition should capture the core factual knowledge that persists across episodes.
Be atomic and precise. Return only the proposition text.

Fragments:
%s`, sb.String())

	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a memory consolidation engine. Distill episodic fragments into a single semantic fact.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxTokens:   256,
		Temperature: float32(0.1),
	})
	if err != nil {
		return "", fmt.Errorf("openai synthesize: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai synthesize: no response")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// ScoreDIG computes the Document Information Gain:
//
//	DIG(d|x) = log P(y|x,d) - log P(y|x)
//
// This measures how much a document reduces uncertainty about the answer.
func (o *OpenAIProvider) ScoreDIG(ctx context.Context, query string, document string) (float64, error) {
	// Step 1: Get log probability of answer without context.
	baselineResp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: query},
		},
		MaxTokens:   64,
		LogProbs:    true,
		TopLogProbs: 1,
	})
	if err != nil {
		return 0, fmt.Errorf("dig baseline: %w", err)
	}

	// Step 2: Get log probability of answer with context document.
	contextPrompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", document, query)
	contextResp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: contextPrompt},
		},
		MaxTokens:   64,
		LogProbs:    true,
		TopLogProbs: 1,
	})
	if err != nil {
		return 0, fmt.Errorf("dig context: %w", err)
	}

	baselineLP := avgLogProb(baselineResp)
	contextLP := avgLogProb(contextResp)

	// DIG = log P(y|x,d) - log P(y|x)
	dig := contextLP - baselineLP
	return dig, nil
}

// Generate produces a completion for general-purpose use.
func (o *OpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   o.maxTokens,
		Temperature: float32(o.temperature),
	})
	if err != nil {
		return "", fmt.Errorf("openai generate: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai generate: no response")
	}

	return resp.Choices[0].Message.Content, nil
}

// CountTokens returns an approximate token count using the ~4 chars per token heuristic.
// For production, integrate tiktoken.
func (o *OpenAIProvider) CountTokens(text string) int {
	// Reasonable approximation: 1 token ≈ 4 characters for English text.
	count := len(text) / 4
	if count == 0 && len(text) > 0 {
		count = 1
	}
	return count
}

// --- Helpers ---

func avgLogProb(resp openai.ChatCompletionResponse) float64 {
	if len(resp.Choices) == 0 {
		return math.Log(0.5) // neutral prior
	}

	choice := resp.Choices[0]
	if choice.LogProbs == nil || len(choice.LogProbs.Content) == 0 {
		return math.Log(0.5)
	}

	sum := 0.0
	for _, lp := range choice.LogProbs.Content {
		sum += float64(lp.LogProb)
	}
	return sum / float64(len(choice.LogProbs.Content))
}
