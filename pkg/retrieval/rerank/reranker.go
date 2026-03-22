package rerank

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// ensure interface implementation
var _ core.ResultEnhancer = (*CrossEncoder)(nil)

const defaultRerankPrompt = `You are an expert at assessing the relevance of documents to queries.
Your task is to score how relevant each retrieved chunk is to the user's query.

Rate each chunk on a scale from 0.0 to 1.0:
- **1.0**: Perfect match
- **0.7-0.9**: Highly relevant
- **0.4-0.6**: Somewhat relevant
- **0.1-0.3**: Barely relevant
- **0.0**: Completely irrelevant

[Query]
%s

[Retrieved Chunks - %d total]
  %s

Output your response as a valid JSON array of scores:
[score1, score2, score3, ...]`

// CrossEncoder uses a CrossEncoder model to rerank results.
type CrossEncoder struct {
	llm       chat.Client
	topK      int
	logger    logging.Logger
	collector observability.Collector
}

// CrossEncoderOption configures a CrossEncoder instance.
type CrossEncoderOption func(*CrossEncoder)

// WithRerankTopK sets the number of results to keep.
func WithRerankTopK(k int) CrossEncoderOption {
	return func(r *CrossEncoder) {
		if k > 0 {
			r.topK = k
		}
	}
}

// WithRerankLogger sets the logger.
func WithRerankLogger(logger logging.Logger) CrossEncoderOption {
	return func(r *CrossEncoder) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// WithRerankCollector sets the collector.
func WithRerankCollector(collector observability.Collector) CrossEncoderOption {
	return func(r *CrossEncoder) {
		if collector != nil {
			r.collector = collector
		}
	}
}

// NewCrossEncoder creates a new cross-encoder reranker.
func NewCrossEncoder(llm chat.Client, opts ...CrossEncoderOption) *CrossEncoder {
	r := &CrossEncoder{
		llm:       llm,
		topK:      10,
		logger:    logging.DefaultNoopLogger(),
		collector: observability.DefaultNoopCollector(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Enhance implements core.ResultEnhancer.
func (r *CrossEncoder) Enhance(ctx context.Context, query *core.Query, chunks []*core.Chunk) ([]*core.Chunk, error) {
	start := time.Now()
	defer func() {
		r.collector.RecordDuration("cross_encoder_rerank", time.Since(start), nil)
	}()

	if query == nil {
		return nil, fmt.Errorf("rerank: query is nil")
	}

	if len(chunks) == 0 {
		return chunks, nil
	}

	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = fmt.Sprintf("Chunk[%d]: %s", i, chunk.Content)
	}

	prompt := fmt.Sprintf(defaultRerankPrompt,
		query.Text,
		len(chunks),
		strings.Join(chunkTexts, "\n"))

	messages := []chat.Message{chat.NewUserMessage(prompt)}
	response, err := r.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("rerank LLM call failed: %w", err)
	}

	scores, err := parseRerankScores(response.Content, len(chunks))
	if err != nil {
		return chunks, nil
	}

	indices := make([]int, len(chunks))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return scores[indices[i]] > scores[indices[j]]
	})

	var finalChunks []*core.Chunk
	for _, idx := range indices {
		finalChunks = append(finalChunks, chunks[idx])
		if len(finalChunks) >= r.topK {
			break
		}
	}

	return finalChunks, nil
}

func parseRerankScores(content string, expectedLen int) ([]float32, error) {
	content = strings.TrimSpace(content)
	jsonStart := strings.Index(content, "[")
	jsonEnd := strings.LastIndex(content, "]")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	var scores []float32
	if err := json.Unmarshal([]byte(content), &scores); err != nil {
		return nil, err
	}
	return scores, nil
}
