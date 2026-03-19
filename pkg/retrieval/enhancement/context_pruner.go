// Package enhancement provides query and document enhancement utilities for RAG systems.
package enhancement

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// ensure interface implementation
var _ core.ResultEnhancer = (*ContextPruner)(nil)

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
  %s

Output your response as a valid JSON array with this exact structure:
[
  {"chunk_index": 0, "relevance": 0.0-1.0, "reason": "brief explanation"},
  {"chunk_index": 1, "relevance": 0.0-1.0, "reason": "brief explanation"},
  ...
]`

// ContextPruner prunes irrelevant context to optimize token usage.
type ContextPruner struct {
	llm       chat.Client
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
func NewContextPruner(llm chat.Client, opts ...ContextPrunerOption) *ContextPruner {
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

// Enhance implements core.ResultEnhancer.
func (p *ContextPruner) Enhance(ctx context.Context, query *core.Query, chunks []*core.Chunk) ([]*core.Chunk, error) {
	start := time.Now()
	defer func() {
		p.collector.RecordDuration("context_pruning", time.Since(start), nil)
	}()

	if len(chunks) == 0 {
		return chunks, nil
	}

	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = fmt.Sprintf("Chunk[%d]: %s", i, chunk.Content)
	}

	prompt := fmt.Sprintf(defaultContextPruningPrompt,
		query.Text,
		len(chunks),
		strings.Join(chunkTexts, "\n"))

	messages := []chat.Message{chat.NewUserMessage(prompt)}
	response, err := p.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("context pruning LLM call failed: %w", err)
	}

	relevanceScores, err := parseRelevanceScores(response.Content, len(chunks))
	if err != nil {
		return chunks, nil // Fallback to original
	}

	indices := make([]int, len(chunks))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return relevanceScores[indices[i]] > relevanceScores[indices[j]]
	})

	var finalChunks []*core.Chunk
	currentTokens := 0
	for _, idx := range indices {
		chunk := chunks[idx]
		chunkTokens := utf8.RuneCountInString(chunk.Content) / 4
		if currentTokens+chunkTokens <= p.maxTokens {
			finalChunks = append(finalChunks, chunk)
			currentTokens += chunkTokens
		} else {
			break
		}
	}

	return finalChunks, nil
}

func parseRelevanceScores(content string, expectedLen int) ([]float32, error) {
	var results []struct {
		ChunkIndex int     `json:"chunk_index"`
		Relevance  float32 `json:"relevance"`
	}
	
	jsonStart := strings.Index(content, "[")
	jsonEnd := strings.LastIndex(content, "]")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	if err := json.Unmarshal([]byte(content), &results); err != nil {
		return nil, err
	}

	scores := make([]float32, expectedLen)
	for _, r := range results {
		if r.ChunkIndex >= 0 && r.ChunkIndex < expectedLen {
			scores[r.ChunkIndex] = r.Relevance
		}
	}
	return scores, nil
}
