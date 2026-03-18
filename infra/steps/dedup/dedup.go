// Package dedup provides deduplication steps for RAG retrieval pipelines.
package dedup

import (
	"context"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// unique removes duplicate chunks from retrieval results based on content similarity.
type unique struct {
	threshold float64
	logger    logging.Logger
	metrics   abstraction.Metrics
}

// Unique creates a new deduplication step with logger and metrics.
//
// Parameters:
//   - threshold: similarity threshold for deduplication (0.0-1.0, default: 0.95)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(dedup.Unique(0.95, logger, metrics))
func Unique(
	threshold float64,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if threshold <= 0 || threshold > 1.0 {
		threshold = 0.95 // Default threshold
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &unique{
		threshold: threshold,
		logger:    logger,
		metrics:   metrics,
	}
}

// Name returns the step name
func (s *unique) Name() string {
	return "Deduplication"
}

// Execute removes duplicate chunks from state.RetrievedChunks and state.ParallelResults.
func (s *unique) Execute(_ context.Context, state *entity.PipelineState) error {
	// Flatten all chunks from RetrievedChunks and ParallelResults
	allChunks := s.flattenChunks(state.RetrievedChunks)
	allChunks = append(allChunks, s.flattenChunks(state.ParallelResults)...)

	if len(allChunks) == 0 {
		s.logger.Debug("Deduplication: no chunks to process", map[string]interface{}{
			"step": "Deduplication",
		})
		return nil
	}

	uniqueChunks := make([]*entity.Chunk, 0, len(allChunks))
	selectedContents := make([]string, 0, len(allChunks))

	for _, chunk := range allChunks {
		isDuplicate := false

		// Check content similarity with already selected chunks
		for _, selectedContent := range selectedContents {
			similarity := s.contentSimilarity(chunk.Content, selectedContent)
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

	// Calculate duplicates removed
	duplicatesRemoved := len(allChunks) - len(uniqueChunks)

	// Put unique chunks back into RetrievedChunks
	state.RetrievedChunks = [][]*entity.Chunk{uniqueChunks}
	state.ParallelResults = nil // Clear parallel results after deduplication

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("deduplication", len(uniqueChunks))
	}

	s.logger.Info("Deduplication completed", map[string]interface{}{
		"step":               "Deduplication",
		"input_count":        len(allChunks),
		"output_count":       len(uniqueChunks),
		"duplicates_removed": duplicatesRemoved,
		"threshold":          s.threshold,
	})

	return nil
}

// flattenChunks flattens [][]*entity.Chunk into []*entity.Chunk.
func (s *unique) flattenChunks(chunks [][]*entity.Chunk) []*entity.Chunk {
	var result []*entity.Chunk
	for _, chunkSlice := range chunks {
		result = append(result, chunkSlice...)
	}
	return result
}

// contentSimilarity calculates text-based similarity between two content strings.
func (s *unique) contentSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	wordsA := s.tokenize(a)
	wordsB := s.tokenize(b)

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
func (s *unique) tokenize(text string) []string {
	words := make([]string, 0)
	currentWord := ""

	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
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
