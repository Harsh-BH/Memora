package llm

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// modelPaths locates the third_party assets relative to this package, and
// skips (not fails) if they aren't present -- these are a few dozen MB of
// downloaded model/runtime files, not checked into git, so a clone without
// them should skip this test rather than break the build.
func modelPaths(t *testing.T) (lib, model, vocab string) {
	t.Helper()
	root := filepath.Join("..", "..", "third_party")
	lib = filepath.Join(root, "onnxruntime", "lib", "libonnxruntime.so")
	model = filepath.Join(root, "models", "all-MiniLM-L6-v2", "model_quint8_avx2.onnx")
	vocab = filepath.Join(root, "models", "all-MiniLM-L6-v2", "vocab.txt")
	for _, p := range []string{lib, model, vocab} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("third_party asset missing (%s): %v -- see README for the download commands", p, err)
		}
	}
	return lib, model, vocab
}

func cosineSimilarity(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// TestLocalEmbedProviderSemanticSimilarity is the regression check for the
// whole ONNX + WordPiece pipeline: a paraphrase must score closer than an
// unrelated sentence. If the tokenizer, the input tensor layout, or the
// mean-pooling math is wrong, embeddings degrade toward noise and this is
// the cheapest way to notice.
func TestLocalEmbedProviderSemanticSimilarity(t *testing.T) {
	lib, model, vocab := modelPaths(t)

	p, err := NewLocalEmbedProvider(lib, model, vocab, 64, 384)
	if err != nil {
		t.Fatalf("NewLocalEmbedProvider: %v", err)
	}
	defer p.Close()

	texts := []string{
		"The cat sat on the mat.",         // 0
		"A feline rested on the rug.",     // 1: paraphrase of 0
		"Quantum entanglement is a physics phenomenon.", // 2: unrelated
	}
	embs, err := p.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(embs) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(embs))
	}
	for i, e := range embs {
		if len(e) != 384 {
			t.Fatalf("embedding[%d]: expected dim 384, got %d", i, len(e))
		}
		norm := math.Sqrt(float64(cosineSimilarity(e, e))) // ~1.0 iff already unit-normalized
		if math.Abs(norm-1.0) > 1e-3 {
			t.Errorf("embedding[%d]: expected unit norm, got %.6f", i, norm)
		}
	}

	simParaphrase := cosineSimilarity(embs[0], embs[1])
	simUnrelated := cosineSimilarity(embs[0], embs[2])
	t.Logf("cos(cat, feline-paraphrase)=%.4f  cos(cat, quantum-unrelated)=%.4f", simParaphrase, simUnrelated)
	if simParaphrase <= simUnrelated {
		t.Fatalf("paraphrase should score higher than an unrelated sentence: %.4f vs %.4f",
			simParaphrase, simUnrelated)
	}

	// Embed(text) and EmbedBatch([]string{text}) both call the model with a
	// batch of exactly 1, so they must agree essentially exactly. A same
	// text embedded as part of a LARGER batch is deliberately not compared
	// here: this quantized model computes its dynamic quantization scale
	// from the whole batch's activation statistics, so the same row's
	// output shifts slightly (empirically ~0.99 cosine, not 1.0) depending
	// on what else shares its batch. That is a real property of dynamic
	// int8 quantization, not a bug -- see EmbedBatch's doc comment. It
	// means reproducibility requires embedding with a consistent batch
	// shape between ingest and query time, which the eval harness does
	// (batch size 1 throughout).
	single, err := p.Embed(context.Background(), texts[0])
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	soloBatch, err := p.EmbedBatch(context.Background(), []string{texts[0]})
	if err != nil {
		t.Fatalf("EmbedBatch(single): %v", err)
	}
	if diff := cosineSimilarity(single, soloBatch[0]); diff < 0.9999 {
		t.Fatalf("Embed and EmbedBatch(single-item) disagree on the same text: cosine=%.6f", diff)
	}
}

// TestLocalEmbedProviderUnimplementedMethods documents (and locks in) which
// Provider methods are intentionally unavailable on this provider.
func TestLocalEmbedProviderUnimplementedMethods(t *testing.T) {
	lib, model, vocab := modelPaths(t)
	p, err := NewLocalEmbedProvider(lib, model, vocab, 64, 384)
	if err != nil {
		t.Fatalf("NewLocalEmbedProvider: %v", err)
	}
	defer p.Close()

	ctx := context.Background()
	if _, err := p.GetTokenProbabilities(ctx, "x"); err != ErrNoLLMCredential {
		t.Errorf("GetTokenProbabilities: expected ErrNoLLMCredential, got %v", err)
	}
	if _, err := p.ExtractTriples(ctx, "x"); err != ErrNoLLMCredential {
		t.Errorf("ExtractTriples: expected ErrNoLLMCredential, got %v", err)
	}
	if _, err := p.Synthesize(ctx, nil); err != ErrNoLLMCredential {
		t.Errorf("Synthesize: expected ErrNoLLMCredential, got %v", err)
	}
	if _, err := p.ScoreDIG(ctx, "q", "d"); err != ErrNoLLMCredential {
		t.Errorf("ScoreDIG: expected ErrNoLLMCredential, got %v", err)
	}
	if _, err := p.Generate(ctx, "x"); err != ErrNoLLMCredential {
		t.Errorf("Generate: expected ErrNoLLMCredential, got %v", err)
	}
}
