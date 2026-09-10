package eval

// A disk-backed embedding cache, so re-running the forgetting experiment does
// not re-embed 10,960 turns every time.
//
// The safety property that matters here is that a STALE cache must never
// silently change a result. Two guards: entries are keyed by the sha256 of the
// text itself, so edited text simply misses; and the file carries a header with
// the embedder's identity (model file sha256, max sequence length, dimension)
// which must match exactly or the whole cache is discarded.

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"os"
	"path/filepath"
)

// EmbedCache maps sha256(text) -> vector for one exact embedder configuration.
type EmbedCache struct {
	ModelSHA  string
	MaxSeqLen int
	Dim       int
	Vectors   map[string][]float32
}

// NewEmbedCache creates an empty cache pinned to one embedder configuration.
func NewEmbedCache(modelSHA string, maxSeqLen, dim int) *EmbedCache {
	return &EmbedCache{ModelSHA: modelSHA, MaxSeqLen: maxSeqLen, Dim: dim,
		Vectors: map[string][]float32{}}
}

// TextKey is the cache key: the sha256 of the text, so a changed turn misses
// rather than returning the previous turn's vector.
func TextKey(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Get returns a cached vector, if present.
func (c *EmbedCache) Get(text string) ([]float32, bool) {
	v, ok := c.Vectors[TextKey(text)]
	return v, ok
}

// Put stores a vector.
func (c *EmbedCache) Put(text string, vec []float32) { c.Vectors[TextKey(text)] = vec }

// LoadEmbedCache reads a cache file, returning an empty cache when the file is
// missing, unreadable, or was written by a different embedder configuration.
// Never an error: a cache miss is always recoverable by embedding again, and
// failing a run over a cold cache would be worse than the cost of refilling it.
func LoadEmbedCache(path, modelSHA string, maxSeqLen, dim int) *EmbedCache {
	empty := NewEmbedCache(modelSHA, maxSeqLen, dim)
	f, err := os.Open(path)
	if err != nil {
		return empty
	}
	defer f.Close()
	var c EmbedCache
	if err := gob.NewDecoder(f).Decode(&c); err != nil {
		return empty
	}
	if c.ModelSHA != modelSHA || c.MaxSeqLen != maxSeqLen || c.Dim != dim || c.Vectors == nil {
		return empty
	}
	return &c
}

// Save writes the cache atomically (temp file + rename), so an interrupted run
// cannot leave a half-written cache that the header check would still accept.
func (c *EmbedCache) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".embedcache*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := gob.NewEncoder(tmp).Encode(c); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// FileSHA256 returns a file's sha256, used to pin the cache to the exact model
// binary that produced it.
func FileSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
