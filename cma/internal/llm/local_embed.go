package llm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/memora/cma/internal/models"
)

// ErrNoLLMCredential is returned by every LocalEmbedProvider method that
// would need a hosted LLM (triple extraction, synthesis, DIG scoring,
// generation, token-probability segmentation). No OpenAI key is configured
// and Claude's Messages API has no logprobs surface at all -- this is not a
// missing-key situation for DIG/GetTokenProbabilities specifically, it is a
// permanent capability gap for this provider. ExtractTriples/Synthesize are
// only used by the consolidation (Sleep) cycle, which this provider's
// callers do not exercise.
var ErrNoLLMCredential = errors.New("llm: not available on LocalEmbedProvider (no hosted LLM credential; embeddings only)")

// LocalEmbedProvider implements llm.Provider's embedding methods with a
// local ONNX sentence-embedding model (all-MiniLM-L6-v2, 384-d, quint8
// quantized), so Embed/EmbedBatch/CountTokens work with no API key, no
// network call, and a byte-for-byte reproducible vector for a given input.
// Every other Provider method returns ErrNoLLMCredential.
type LocalEmbedProvider struct {
	mu        sync.Mutex // onnxruntime_go sessions are not documented as goroutine-safe
	session   *ort.DynamicAdvancedSession
	tokenizer *wordpieceTokenizer
	maxSeqLen int
	dim       int
}

// NewLocalEmbedProvider loads the ONNX Runtime shared library, the model,
// and the tokenizer vocab. onnxLibPath, modelPath and vocabPath are plain
// filesystem paths (see cma/third_party/).
func NewLocalEmbedProvider(onnxLibPath, modelPath, vocabPath string, maxSeqLen, dim int) (*LocalEmbedProvider, error) {
	if !ort.IsInitialized() {
		ort.SetSharedLibraryPath(onnxLibPath)
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("onnxruntime init: %w", err)
		}
	}

	tok, err := newWordpieceTokenizer(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("load vocab: %w", err)
	}

	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("session options: %w", err)
	}
	defer opts.Destroy()

	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		opts,
	)
	if err != nil {
		return nil, fmt.Errorf("load onnx model: %w", err)
	}

	return &LocalEmbedProvider{
		session:   session,
		tokenizer: tok,
		maxSeqLen: maxSeqLen,
		dim:       dim,
	}, nil
}

// Close releases the ONNX session. Does not tear down the shared onnxruntime
// environment (InitializeEnvironment is process-global).
func (p *LocalEmbedProvider) Close() error {
	return p.session.Destroy()
}

// Dim returns the embedding dimensionality this model produces (384 for
// all-MiniLM-L6-v2 -- NOT the 1536 the production Qdrant collection is
// configured for; callers must use a separate collection or reconfigure).
func (p *LocalEmbedProvider) Dim() int { return p.dim }

func (p *LocalEmbedProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	out, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

// EmbedBatch runs one ONNX inference over all texts (padded to the same
// sequence length), mean-pools each row's token embeddings over real
// (non-padding) tokens, and L2-normalizes -- the standard sentence-
// transformers "mean pooling" head for a bare BERT encoder.
//
// Caveat measured empirically (local_embed_test.go): this quantized model
// computes its dynamic int8 quantization scale from the whole batch's
// activation statistics, so the same text's output vector shifts slightly
// (~0.99 cosine, not 1.0) depending on what else shares its batch --
// mathematically surprising for an otherwise-independent per-row computation,
// but a real, reproducible property of dynamic quantization, not a bug here.
// For a reproducible corpus, embed with a consistent batch shape between
// ingest and query time (the eval harness uses batch size 1 throughout).
func (p *LocalEmbedProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	batch := len(texts)
	seq := p.maxSeqLen
	inputIDs := make([]int64, batch*seq)
	attnMask := make([]int64, batch*seq)
	tokenTypes := make([]int64, batch*seq) // all zero: single-sequence input

	for i, text := range texts {
		ids, mask := p.tokenizer.Encode(text, seq)
		copy(inputIDs[i*seq:(i+1)*seq], ids)
		copy(attnMask[i*seq:(i+1)*seq], mask)
	}

	shape := ort.NewShape(int64(batch), int64(seq))
	idsTensor, err := ort.NewTensor(shape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("input_ids tensor: %w", err)
	}
	defer idsTensor.Destroy()
	maskTensor, err := ort.NewTensor(shape, attnMask)
	if err != nil {
		return nil, fmt.Errorf("attention_mask tensor: %w", err)
	}
	defer maskTensor.Destroy()
	typeTensor, err := ort.NewTensor(shape, tokenTypes)
	if err != nil {
		return nil, fmt.Errorf("token_type_ids tensor: %w", err)
	}
	defer typeTensor.Destroy()

	outputs := []ort.Value{nil}

	p.mu.Lock()
	err = p.session.Run([]ort.Value{idsTensor, maskTensor, typeTensor}, outputs)
	p.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}
	hidden, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		outputs[0].Destroy()
		return nil, fmt.Errorf("unexpected output tensor type %T", outputs[0])
	}
	defer hidden.Destroy()

	data := hidden.GetData() // flattened [batch, seq, dim]
	outShape := hidden.GetShape()
	if len(outShape) != 3 || int(outShape[2]) != p.dim {
		return nil, fmt.Errorf("unexpected output shape %v (want [.,.,%d])", outShape, p.dim)
	}

	result := make([][]float32, batch)
	for b := 0; b < batch; b++ {
		result[b] = meanPoolAndNormalize(data, attnMask, b, seq, p.dim)
	}
	return result, nil
}

// meanPoolAndNormalize averages token embeddings for row b over positions
// where attnMask is 1, then L2-normalizes the result.
func meanPoolAndNormalize(data []float32, attnMask []int64, b, seq, dim int) []float32 {
	out := make([]float32, dim)
	var count float32
	rowMaskStart := b * seq
	rowDataStart := b * seq * dim

	for t := 0; t < seq; t++ {
		if attnMask[rowMaskStart+t] == 0 {
			continue
		}
		count++
		base := rowDataStart + t*dim
		for d := 0; d < dim; d++ {
			out[d] += data[base+d]
		}
	}
	if count > 0 {
		for d := range out {
			out[d] /= count
		}
	}

	var norm float32
	for _, v := range out {
		norm += v * v
	}
	if norm > 0 {
		inv := float32(1.0 / math.Sqrt(float64(norm)))
		for d := range out {
			out[d] *= inv
		}
	}
	return out
}

// CountTokens returns the real WordPiece token count (uncapped by the
// model's max sequence length), unlike the chars/4 heuristic used
// elsewhere in this codebase for providers without a real tokenizer.
func (p *LocalEmbedProvider) CountTokens(text string) int {
	return p.tokenizer.CountTokens(text)
}

func (p *LocalEmbedProvider) GetTokenProbabilities(ctx context.Context, text string) ([]TokenProb, error) {
	return nil, ErrNoLLMCredential
}

func (p *LocalEmbedProvider) ExtractTriples(ctx context.Context, content string) ([]models.Triple, error) {
	return nil, ErrNoLLMCredential
}

func (p *LocalEmbedProvider) Synthesize(ctx context.Context, episodes []models.Episode) (string, error) {
	return "", ErrNoLLMCredential
}

func (p *LocalEmbedProvider) ScoreDIG(ctx context.Context, query string, document string) (float64, error) {
	return 0, ErrNoLLMCredential
}

func (p *LocalEmbedProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return "", ErrNoLLMCredential
}

var _ Provider = (*LocalEmbedProvider)(nil)
