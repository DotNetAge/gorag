// Package enhancer provides query and document enhancement utilities for RAG systems.
// This file implements sentence window expansion for context enhancement.
package enhancer

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.ResultEnhancer = (*SentenceWindowExpander)(nil)

// SentenceWindowExpander expands retrieved chunks with surrounding sentences.
// It provides more complete context while maintaining retrieval precision.
type SentenceWindowExpander struct {
	windowSize int // Number of sentences to add before and after
	maxChars   int // Maximum characters per expanded chunk
	logger     logging.Logger
	collector  observability.Collector
}

// SentenceWindowExpanderOption configures a SentenceWindowExpander instance.
type SentenceWindowExpanderOption func(*SentenceWindowExpander)

// WithWindowSize sets the number of sentences to expand (before and after).
func WithWindowSize(size int) SentenceWindowExpanderOption {
	return func(e *SentenceWindowExpander) {
		if size > 0 {
			e.windowSize = size
		}
	}
}

// WithMaxChars sets the maximum characters per expanded chunk.
func WithMaxChars(maxChars int) SentenceWindowExpanderOption {
	return func(e *SentenceWindowExpander) {
		if maxChars > 0 {
			e.maxChars = maxChars
		}
	}
}

// WithExpanderLogger sets a structured logger.
func WithExpanderLogger(logger logging.Logger) SentenceWindowExpanderOption {
	return func(e *SentenceWindowExpander) {
		if logger != nil {
			e.logger = logger
		}
	}
}

// WithExpanderCollector sets an observability collector.
func WithExpanderCollector(collector observability.Collector) SentenceWindowExpanderOption {
	return func(e *SentenceWindowExpander) {
		if collector != nil {
			e.collector = collector
		}
	}
}

// NewSentenceWindowExpander creates a new sentence window expander.
//
// Required: none.
// Optional (via options): WithWindowSize (default: 2), WithMaxChars (default: 2000),
// WithExpanderLogger, WithExpanderCollector.
func NewSentenceWindowExpander(opts ...SentenceWindowExpanderOption) *SentenceWindowExpander {
	e := &SentenceWindowExpander{
		windowSize: 2,
		maxChars:   2000,
		logger:     logging.NewNoopLogger(),
		collector:  observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Enhance expands chunks with surrounding sentences from the original document.
//
// Parameters:
// - ctx: The context for cancellation and timeouts
// - results: The retrieval results to expand
//
// Returns:
// - The expanded retrieval results with sentence windows
// - An error if expansion fails
func (e *SentenceWindowExpander) Enhance(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("sentence_window_expansion", time.Since(start), nil)
	}()

	if results == nil || len(results.Chunks) == 0 {
		e.logger.Debug("no chunks to expand", map[string]interface{}{
			"operation": "sentence_window_expansion",
		})
		return results, nil
	}

	e.logger.Debug("expanding sentence windows", map[string]interface{}{
		"operation":   "sentence_window_expansion",
		"chunk_count": len(results.Chunks),
		"window_size": e.windowSize,
	})

	// Expand each chunk with sentence window
	expandedChunks := make([]*entity.Chunk, 0, len(results.Chunks))

	for i, chunk := range results.Chunks {
		expanded := e.expandChunk(chunk)
		expandedChunks = append(expandedChunks, expanded)

		e.logger.Debug("expanded chunk", map[string]interface{}{
			"operation":       "sentence_window_expansion",
			"chunk_index":     i,
			"original_length": len(chunk.Content),
			"expanded_length": len(expanded.Content),
		})
	}

	e.logger.Info("sentence window expansion completed", map[string]interface{}{
		"operation":       "sentence_window_expansion",
		"original_chunks": len(results.Chunks),
		"expanded_chunks": len(expandedChunks),
		"window_size":     e.windowSize,
	})
	e.collector.RecordCount("sentence_window_expansion", "success", nil)

	// Create new retrieval result with expanded chunks
	return entity.NewRetrievalResult(
		results.ID,
		results.QueryID,
		expandedChunks,
		results.Scores,
		results.Metadata,
	), nil
}

// expandChunk expands a single chunk with surrounding sentences.
func (e *SentenceWindowExpander) expandChunk(chunk *entity.Chunk) *entity.Chunk {
	// Get full document content from metadata if available
	fullContent, ok := chunk.Metadata["full_document"].(string)
	if !ok {
		// No full document available, return original chunk
		return chunk
	}

	// Split into sentences
	sentences := splitIntoSentences(fullContent)

	// Find the position of current chunk in sentences
	chunkStart := -1
	for i, sent := range sentences {
		if strings.Contains(sent, chunk.Content[:min(50, len(chunk.Content))]) {
			chunkStart = i
			break
		}
	}

	if chunkStart == -1 {
		// Can't find chunk position, return original
		return chunk
	}

	// Calculate window boundaries
	windowStart := max(0, chunkStart-e.windowSize)
	windowEnd := min(len(sentences)-1, chunkStart+e.windowSize)

	// Build expanded content
	var builder strings.Builder
	for i := windowStart; i <= windowEnd; i++ {
		builder.WriteString(sentences[i])
		if i < windowEnd {
			builder.WriteString(" ")
		}

		// Check max chars limit
		if builder.Len() >= e.maxChars {
			break
		}
	}

	// Create expanded chunk
	return &entity.Chunk{
		ID:         chunk.ID,
		DocumentID: chunk.DocumentID,
		ParentID:   chunk.ParentID,
		Level:      chunk.Level,
		Content:    builder.String(),
		Metadata:   chunk.Metadata,
		CreatedAt:  chunk.CreatedAt,
		StartIndex: chunk.StartIndex,
		EndIndex:   chunk.StartIndex + builder.Len(),
		VectorID:   chunk.VectorID,
	}
}

// splitIntoSentences splits text into sentences.
func splitIntoSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for i, r := range text {
		current.WriteRune(r)

		// Check for sentence endings
		if isSentenceEnd(r, text, i) {
			sentence := strings.TrimSpace(current.String())
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			current.Reset()
		}
	}

	// Add remaining text as last sentence
	if current.Len() > 0 {
		sentence := strings.TrimSpace(current.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// isSentenceEnd checks if a rune marks a sentence end.
func isSentenceEnd(r rune, text string, pos int) bool {
	if r != '.' && r != '!' && r != '?' {
		return false
	}

	// Check for abbreviations (simple heuristic)
	if pos+1 < len(text) && unicode.IsLower(rune(text[pos+1])) {
		return false
	}

	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
