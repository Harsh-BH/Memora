// Package pkg provides shared utilities for the CMA system.
package pkg

import (
	"strings"
	"unicode"
)

// TokenizeSimple splits text into approximate tokens for counting purposes.
// For production accuracy, use a proper tokenizer (e.g., tiktoken).
func TokenizeSimple(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
}

// TruncateText truncates text to the specified number of characters,
// adding an ellipsis if truncated. Respects word boundaries.
func TruncateText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}

	truncated := text[:maxChars]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxChars/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}

// CosineSimilarity computes the cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method.
	z := x
	for i := 0; i < 100; i++ {
		z = (z + x/z) / 2
	}
	return z
}
