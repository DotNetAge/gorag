// Package post_retrieval provides steps that process and optimize retrieval results.
package post_retrieval

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// DeduplicationStep removes duplicate chunks from retrieval results based on content similarity.
//
// This step is crucial for:
// - Eliminating redundant information
// - Improving context window efficiency
// - Reducing noise in generation phase
//
// Algorithm:
// 1. Flatten all retrieved chunks from parallel retrieval paths
// 2. For each chunk, check content similarity with already selected chunks
// 3. If similarity > threshold, skip as duplicate
// 4. Otherwise, add to unique results
type DeduplicationStep struct {
	threshold float64 // Similarity threshold (default: 0.95)
}

// NewDeduplicationStep creates a new deduplication step.
//
// Parameters:
//   - threshold: Similarity threshold for considering chunks as duplicates (0.0-1.0)
//     Recommended: 0.90-0.98, higher = more aggressive deduplication
func NewDeduplicationStep(threshold float64) *DeduplicationStep {
	if threshold <= 0 || threshold > 1.0 {
		threshold = 0.95 // Default threshold
	}
	return &DeduplicationStep{
		threshold: threshold,
	}
}

// Name returns the step name.
func (s *DeduplicationStep) Name() string {
	return "DeduplicationStep"
}

// Execute removes duplicate chunks from state.RetrievedChunks and state.ParallelResults.
func (s *DeduplicationStep) Execute(_ context.Context, state *entity.PipelineState) error {
	// Flatten all chunks from RetrievedChunks and ParallelResults
	allChunks := flattenChunks(state.RetrievedChunks)
	allChunks = append(allChunks, flattenChunks(state.ParallelResults)...)

	if len(allChunks) == 0 {
		return nil
	}

	uniqueChunks := make([]*entity.Chunk, 0, len(allChunks))
	selectedContents := make([]string, 0, len(allChunks))

	for _, chunk := range allChunks {
		isDuplicate := false

		// Check content similarity with already selected chunks
		for _, selectedContent := range selectedContents {
			similarity := contentSimilarity(chunk.Content, selectedContent)
			if similarity >= s.threshold {
				isDuplicate = true
				break
			}
		}

		// Add to unique results if not a duplicate
		if !isDuplicate {
			uniqueChunks = append(uniqueChunks, chunk)
			selectedContents = append(selectedContents, chunk.Content)
		}
	}

	// Put unique chunks back into RetrievedChunks
	state.RetrievedChunks = [][]*entity.Chunk{uniqueChunks}
	state.ParallelResults = nil // Clear parallel results after deduplication
	return nil
}

// contentSimilarity calculates text-based similarity between two content strings.
// Uses simple word overlap ratio as a proxy for semantic similarity.
func contentSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	wordsA := tokenize(a)
	wordsB := tokenize(b)

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0.0
	}

	// Calculate Jaccard similarity
	intersection := 0
	wordSetB := make(map[string]bool)
	for _, w := range wordsB {
		wordSetB[w] = true
	}

	for _, w := range wordsA {
		if wordSetB[w] {
			intersection++
		}
	}

	union := len(wordsA) + len(wordsB) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// tokenize splits text into lowercase words (simple tokenization).
func tokenize(text string) []string {
	// Simple whitespace and punctuation splitting
	// In production, use proper tokenizer
	words := make([]string, 0)
	currentWord := ""

	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			currentWord += string(r)
		} else {
			if currentWord != "" {
				words = append(words, currentWord)
				currentWord = ""
			}
		}
	}

	if currentWord != "" {
		words = append(words, currentWord)
	}

	return words
}
