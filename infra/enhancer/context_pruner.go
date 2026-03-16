// Package enhancer provides query and document enhancement utilities for RAG systems.
// This file implements context pruning for retrieval result optimization.
package enhancer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.ResultEnhancer = (*ContextPruner)(nil)

const defaultContextPruningPrompt = `You are an expert at assessing the relevance of text chunks to a query.
Your task is to score how relevant each chunk is for answering the query.

Rate each chunk on a scale from 0.0 to 1.0:
- **1.0**: Perfect match, contains direct answer
- **0.7-0.9**: Highly relevant, contains useful information
- **0.4-0.6**: Somewhat relevant, contains related information
- **0.1-0.3**: Barely relevant, contains tangential information
- **0.0**: Completely irrelevant, no useful information

[Query]
%s

[Retrieved Chunks - %d total]
Chunk[0]: %s
Chunk[1]: %s
Chunk[2]: %s
...

Output your response as a valid JSON array with this exact structure:
[
  {"chunk_index": 0, "relevance": 0.0-1.0, "reason": "brief explanation"},
  {"chunk_index": 1, "relevance": 0.0-1.0, "reason": "brief explanation"},
  ...
]`

// ContextPruner prunes irrelevant context to optimize token usage.
// It evaluates each chunk's relevance and keeps only the most relevant ones.
type ContextPruner struct {
	llm       core.Client
	maxTokens int
	logger    logging.Logger
	collector observability.Collector
}

// ContextPrunerOption configures a ContextPruner instance.
type ContextPrunerOption func(*ContextPruner)

// WithPrunerMaxTokens sets the maximum number of tokens to keep.
func WithPrunerMaxTokens(maxTokens int) ContextPrunerOption {
	return func(p *ContextPruner) {
		if maxTokens > 0 {
			p.maxTokens = maxTokens
		}
	}
}

// WithPrunerLogger sets a structured logger.
func WithPrunerLogger(logger logging.Logger) ContextPrunerOption {
	return func(p *ContextPruner) {
		if logger != nil {
			p.logger = logger
		}
	}
}

// WithPrunerCollector sets an observability collector.
func WithPrunerCollector(collector observability.Collector) ContextPrunerOption {
	return func(p *ContextPruner) {
		if collector != nil {
			p.collector = collector
		}
	}
}

// NewContextPruner creates a new context pruner.
//
// Required: llm.
// Optional (via options): WithPrunerMaxTokens (default: 2000), WithPrunerLogger, WithPrunerCollector.
func NewContextPruner(llm core.Client, opts ...ContextPrunerOption) *ContextPruner {
	p := &ContextPruner{
		llm:       llm,
		maxTokens: 2000,
		logger:    logging.NewNoopLogger(),
		collector: observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Enhance prunes the retrieval results to keep only the most relevant chunks.
//
// Parameters:
// - ctx: The context for cancellation and timeouts
// - results: The retrieval results to prune
//
// Returns:
// - The pruned retrieval results with only relevant chunks
// - An error if pruning fails
func (p *ContextPruner) Enhance(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
	start := time.Now()
	defer func() {
		p.collector.RecordDuration("context_pruning", time.Since(start), nil)
	}()

	if results == nil || len(results.Chunks) == 0 {
		p.logger.Debug("no chunks to prune", map[string]interface{}{
			"operation": "context_pruning",
		})
		return results, nil
	}

	p.logger.Debug("pruning context", map[string]interface{}{
		"operation":      "context_pruning",
		"chunk_count":    len(results.Chunks),
		"max_tokens":     p.maxTokens,
		"current_tokens": countTokens(results.Chunks),
	})

	// Build prompt with all chunks
	chunkTexts := make([]string, len(results.Chunks))
	for i, chunk := range results.Chunks {
		chunkTexts[i] = fmt.Sprintf("Chunk[%d]: %s", i, chunk.Content)
	}

	prompt := fmt.Sprintf(defaultContextPruningPrompt,
		results.QueryID,
		len(results.Chunks),
		strings.Join(chunkTexts, "\n"))

	// Call LLM for scoring
	messages := []core.Message{
		core.NewUserMessage(prompt),
	}

	response, err := p.llm.Chat(ctx, messages)
	if err != nil {
		p.logger.Error("pruning failed", err, map[string]interface{}{
			"operation": "context_pruning",
		})
		p.collector.RecordCount("context_pruning", "error", nil)
		return nil, fmt.Errorf("ContextPruner.Enhance failed to call LLM: %w", err)
	}

	// Parse relevance scores from JSON response
	relevanceScores, err := parseRelevanceScores(response.Content)
	if err != nil {
		p.logger.Warn("failed to parse relevance scores, using original order", map[string]interface{}{
			"error":     err,
			"operation": "context_pruning",
		})
		// Fallback: use original scores
		relevanceScores = make([]float32, len(results.Chunks))
		for i := range relevanceScores {
			relevanceScores[i] = results.Scores[i]
		}
	}

	// Sort chunks by relevance
	sortedChunks, sortedScores := sortByScoreWithRelevance(results.Chunks, results.Scores, relevanceScores)

	// Truncate to maxTokens
	var finalChunks []*entity.Chunk
	var finalScores []float32
	currentTokens := 0

	for i, chunk := range sortedChunks {
		chunkTokens := utf8.RuneCountInString(chunk.Content)
		if currentTokens+chunkTokens <= p.maxTokens {
			finalChunks = append(finalChunks, chunk)
			finalScores = append(finalScores, sortedScores[i])
			currentTokens += chunkTokens
		} else {
			p.logger.Debug("reached max tokens limit", map[string]interface{}{
				"operation":      "context_pruning",
				"kept_chunks":    len(finalChunks),
				"skipped_chunks": len(sortedChunks) - i,
				"total_tokens":   currentTokens,
				"max_tokens":     p.maxTokens,
			})
			break
		}
	}

	p.logger.Info("context pruning completed", map[string]interface{}{
		"operation":        "context_pruning",
		"original_chunks":  len(results.Chunks),
		"pruned_chunks":    len(finalChunks),
		"original_tokens":  countTokens(results.Chunks),
		"pruned_tokens":    currentTokens,
		"max_tokens":       p.maxTokens,
		"compression_rate": fmt.Sprintf("%.2f%%", float64(currentTokens)/float64(countTokens(results.Chunks))*100),
	})
	p.collector.RecordCount("context_pruning", "success", nil)

	// Create new retrieval result with pruned chunks
	return entity.NewRetrievalResult(
		results.ID,
		results.QueryID,
		finalChunks,
		finalScores,
		results.Metadata,
	), nil
}

// countTokens counts the approximate number of tokens in chunks.
func countTokens(chunks []*entity.Chunk) int {
	total := 0
	for _, chunk := range chunks {
		total += utf8.RuneCountInString(chunk.Content) / 4 // Rough estimate: 1 token ≈ 4 characters
	}
	return total
}

// RelevanceScore represents the relevance score of a chunk.
type RelevanceScore struct {
	ChunkIndex int     `json:"chunk_index"`
	Relevance  float32 `json:"relevance"`
	Reason     string  `json:"reason"`
}

// parseRelevanceScores extracts relevance scores from LLM response.
func parseRelevanceScores(content string) ([]float32, error) {
	content = strings.TrimSpace(content)

	// Try to parse as JSON array of objects
	var scores []RelevanceScore
	err := parseJSON(content, &scores)
	if err == nil && len(scores) > 0 {
		result := make([]float32, len(scores))
		for _, s := range scores {
			result[s.ChunkIndex] = s.Relevance
		}
		return result, nil
	}

	// Fallback: try to extract numbers directly
	return parseScores(content)
}

// parseJSON is a simple JSON parser fallback.
func parseJSON(content string, v interface{}) error {
	return json.Unmarshal([]byte(content), v)
}

// sortByScoreWithRelevance sorts chunks by relevance scores (descending).
func sortByScoreWithRelevance(chunks []*entity.Chunk, oldScores []float32, relevanceScores []float32) ([]*entity.Chunk, []float32) {
	if len(relevanceScores) != len(chunks) {
		// Fallback to original order if lengths don't match
		return chunks, oldScores
	}

	// Create index array for sorting
	indices := make([]int, len(chunks))
	for i := range indices {
		indices[i] = i
	}

	// Sort indices by relevance scores (descending)
	sort.Slice(indices, func(i, j int) bool {
		return relevanceScores[indices[i]] > relevanceScores[indices[j]]
	})

	// Reorder chunks and scores
	sortedChunks := make([]*entity.Chunk, len(chunks))
	sortedScores := make([]float32, len(chunks))

	for i, idx := range indices {
		sortedChunks[i] = chunks[idx]
		if i < len(oldScores) {
			sortedScores[i] = oldScores[idx]
		} else {
			sortedScores[i] = relevanceScores[idx]
		}
	}

	return sortedChunks, sortedScores
}
