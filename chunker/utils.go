package chunker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// embedBatch computes embeddings for multiple texts using core.Embedder
// Returns a slice of float32 slices (embeddings) and an error
func embedBatch(embedder core.Embedder, texts []string) ([][]float32, error) {
	if embedder == nil || len(texts) == 0 {
		return nil, nil
	}

	// Create temporary chunks for bulk operation
	chunks := make([]*core.Chunk, len(texts))
	for i, text := range texts {
		chunks[i] = &core.Chunk{
			Content: text,
		}
	}

	// Use Bulk method for efficient batch processing
	vectors, err := embedder.Bulk(chunks)
	if err != nil {
		return nil, err
	}

	// Extract embedding values from vectors
	embeddings := make([][]float32, len(vectors))
	for i, vector := range vectors {
		embeddings[i] = vector.Values
	}
	return embeddings, nil
}

// CosineSimilarity computes the cosine similarity between two vectors
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// GenerateChunkID generates a unique chunk ID
// Format: chunk_{docID}_{index}_{hash8}
func GenerateChunkID(docID string, index int, content string) string {
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])[:8]
	return fmt.Sprintf("chunk_%s_%d_%s", docID, index, hashStr)
}

// NormalizeWhitespace normalizes whitespace characters
// Multiple spaces/newlines are merged into single spaces while preserving line breaks
func NormalizeWhitespace(text string) string {
	// Split by any whitespace, then rejoin with single spaces
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	// Preserve paragraphs by joining with space but keeping single spaces within lines
	result := strings.Join(fields, " ")
	return result
}

// CountLines counts the number of lines in text
func CountLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

// Clamp limits a value to be within [min, max]
func Clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
