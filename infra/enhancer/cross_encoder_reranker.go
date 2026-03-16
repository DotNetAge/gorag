// Package enhancer provides query and document enhancement utilities for RAG systems.
// This file implements CrossEncoder-based reranking for retrieval result enhancement.
package enhancer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.ResultEnhancer = (*CrossEncoderReranker)(nil)

const defaultRerankPrompt = `You are an expert at assessing the relevance of documents to queries.
Your task is to score how relevant each retrieved chunk is to the user's query.

Rate each chunk on a scale from 0.0 to 1.0:
- **1.0**: Perfect match, contains direct answer to the query
- **0.7-0.9**: Highly relevant, contains useful information for answering
- **0.4-0.6**: Somewhat relevant, contains related but incomplete information
- **0.1-0.3**: Barely relevant, contains tangentially related information
- **0.0**: Completely irrelevant, no useful information

[Query]
%s

[Retrieved Chunks - %d total]
Chunk[0]: %s
Chunk[1]: %s
Chunk[2]: %s
...

Output your response as a valid JSON array of scores, one score per chunk in order:
[score1, score2, score3, ...]`

// CrossEncoderReranker uses a CrossEncoder model to rerank retrieval results.
// It scores each (query, chunk) pair and reorders chunks by relevance.
type CrossEncoderReranker struct {
	llm       core.Client
	topK      int
	logger    logging.Logger
	collector observability.Collector
}

// CrossEncoderRerankerOption configures a CrossEncoderReranker instance.
type CrossEncoderRerankerOption func(*CrossEncoderReranker)

// WithRerankTopK sets the number of top results to return after reranking.
func WithRerankTopK(k int) CrossEncoderRerankerOption {
	return func(r *CrossEncoderReranker) {
		if k > 0 {
			r.topK = k
		}
	}
}

// WithRerankLogger sets a structured logger.
func WithRerankLogger(logger logging.Logger) CrossEncoderRerankerOption {
	return func(r *CrossEncoderReranker) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// WithRerankCollector sets an observability collector.
func WithRerankCollector(collector observability.Collector) CrossEncoderRerankerOption {
	return func(r *CrossEncoderReranker) {
		if collector != nil {
			r.collector = collector
		}
	}
}

// NewCrossEncoderReranker creates a new cross-encoder reranker.
//
// Required: llm.
// Optional (via options): WithRerankTopK (default: 10), WithRerankLogger, WithRerankCollector.
func NewCrossEncoderReranker(llm core.Client, opts ...CrossEncoderRerankerOption) *CrossEncoderReranker {
	r := &CrossEncoderReranker{
		llm:       llm,
		topK:      10,
		logger:    logging.NewNoopLogger(),
		collector: observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Enhance reranks the retrieval results using CrossEncoder scoring.
//
// Parameters:
// - ctx: The context for cancellation and timeouts
// - results: The retrieval results to rerank
//
// Returns:
// - The reranked retrieval results with chunks reordered by relevance
// - An error if reranking fails
func (r *CrossEncoderReranker) Enhance(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
	start := time.Now()
	defer func() {
		r.collector.RecordDuration("cross_encoder_rerank", time.Since(start), nil)
	}()

	if results == nil || len(results.Chunks) == 0 {
		r.logger.Debug("no chunks to rerank", map[string]interface{}{
			"operation": "cross_encoder_rerank",
		})
		return results, nil
	}

	r.logger.Debug("reranking results", map[string]interface{}{
		"operation":   "cross_encoder_rerank",
		"chunk_count": len(results.Chunks),
		"top_k":       r.topK,
	})

	// Build prompt with all chunks
	chunkTexts := make([]string, len(results.Chunks))
	for i, chunk := range results.Chunks {
		chunkTexts[i] = fmt.Sprintf("Chunk[%d]: %s", i, chunk.Content)
	}

	prompt := fmt.Sprintf(defaultRerankPrompt,
		results.QueryID,
		len(results.Chunks),
		strings.Join(chunkTexts, "\n"))

	// Call LLM for scoring
	messages := []core.Message{
		core.NewUserMessage(prompt),
	}

	response, err := r.llm.Chat(ctx, messages)
	if err != nil {
		r.logger.Error("rerank failed", err, map[string]interface{}{
			"operation": "cross_encoder_rerank",
		})
		r.collector.RecordCount("cross_encoder_rerank", "error", nil)
		return nil, fmt.Errorf("CrossEncoderReranker.Enhance failed to call LLM: %w", err)
	}

	// Parse scores from JSON response
	scores, err := parseScores(response.Content)
	if err != nil {
		r.logger.Warn("failed to parse scores, using original order", map[string]interface{}{
			"error":     err,
			"operation": "cross_encoder_rerank",
		})
		// Fallback: keep original order
		scores = make([]float32, len(results.Chunks))
		for i := range scores {
			scores[i] = float32(i + 1) // Use index as fallback score
		}
	}

	// Sort chunks and scores together
	sortedChunks, sortedScores := sortByScore(results.Chunks, results.Scores, scores)

	// Truncate to topK
	if len(sortedChunks) > r.topK {
		sortedChunks = sortedChunks[:r.topK]
		sortedScores = sortedScores[:r.topK]
	}

	r.logger.Info("reranking completed", map[string]interface{}{
		"operation":     "cross_encoder_rerank",
		"original_rank": len(results.Chunks),
		"final_rank":    len(sortedChunks),
		"top_k":         r.topK,
	})
	r.collector.RecordCount("cross_encoder_rerank", "success", nil)

	// Create new retrieval result with reranked chunks
	return entity.NewRetrievalResult(
		results.ID,
		results.QueryID,
		sortedChunks,
		sortedScores,
		results.Metadata,
	), nil
}

// parseScores extracts scores from LLM response.
func parseScores(content string) ([]float32, error) {
	content = strings.TrimSpace(content)

	// Try to parse as JSON array first
	var jsonScores []float64
	err := json.Unmarshal([]byte(content), &jsonScores)
	if err == nil && len(jsonScores) > 0 {
		scores := make([]float32, len(jsonScores))
		for i, s := range jsonScores {
			scores[i] = float32(s)
		}
		return scores, nil
	}

	// Fallback: extract numbers manually
	// Remove brackets
	content = strings.TrimPrefix(content, "[")
	content = strings.TrimSuffix(content, "]")

	parts := strings.Split(content, ",")
	scores := make([]float32, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var score float64
		_, err := fmt.Sscanf(part, "%f", &score)
		if err != nil {
			return nil, fmt.Errorf("failed to parse score '%s': %w", part, err)
		}
		scores = append(scores, float32(score))
	}

	if len(scores) == 0 {
		return nil, fmt.Errorf("no scores found in response")
	}

	return scores, nil
}

// sortByScore sorts chunks and scores by the new CrossEncoder scores (descending).
func sortByScore(chunks []*entity.Chunk, oldScores []float32, newScores []float32) ([]*entity.Chunk, []float32) {
	// Create index array for sorting
	indices := make([]int, len(chunks))
	for i := range indices {
		indices[i] = i
	}

	// Sort indices by new scores (descending)
	sort.Slice(indices, func(i, j int) bool {
		return newScores[indices[i]] > newScores[indices[j]]
	})

	// Reorder chunks and scores
	sortedChunks := make([]*entity.Chunk, len(chunks))
	sortedScores := make([]float32, len(chunks))

	for i, idx := range indices {
		sortedChunks[i] = chunks[idx]
		if i < len(oldScores) {
			sortedScores[i] = oldScores[idx]
		} else {
			sortedScores[i] = newScores[idx]
		}
	}

	return sortedChunks, sortedScores
}
