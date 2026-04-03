package prune

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// compress compresses retrieved chunks to extract only the most relevant information.
type compress struct {
	llm       chat.Client
	logger    logging.Logger
	metrics   core.Metrics
	maxTokens int
}

// Compress creates a context compression step with logger and metrics.
//
// Parameters:
//   - llm: LLM client for context compression
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//   - maxTokens: maximum tokens in compressed output (default: 300)
//
// Example:
//
//	p.AddStep(prune.Compress(llm, logger, metrics, 300))
func Compress(llm chat.Client, logger logging.Logger, metrics core.Metrics, maxTokens int) pipeline.Step[*core.RetrievalContext] {
	if maxTokens <= 0 {
		maxTokens = 300
	}
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &compress{
		llm:       llm,
		logger:    logger,
		metrics:   metrics,
		maxTokens: maxTokens,
	}
}

// Name returns the step name
func (s *compress) Name() string {
	return "ContextCompression"
}

// Execute compresses all retrieved chunks to extract only relevant information.
func (s *compress) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if len(state.RetrievedChunks) == 0 {
		return nil
	}

	query := state.Query.Text
	if query == "" {
		s.logger.Warn("ContextCompression: empty query, skipping compression")
		return nil
	}

	// Flatten all chunks
	allChunks := s.flattenChunks(state.RetrievedChunks)
	if len(allChunks) == 0 {
		return nil
	}

	// Build context from chunks
	var contextBuilder strings.Builder
	for i, chunk := range allChunks {
		contextBuilder.WriteString(fmt.Sprintf("[Chunk %d]\n%s\n", i+1, chunk.Content))
	}
	contextText := contextBuilder.String()

	// Create compression prompt
	prompt := fmt.Sprintf(`Given the following context and query, extract only the most relevant information (max %d tokens):

Query: %s

Context:
%s

Extracted Information:`, s.maxTokens, query, contextText)

	// Call LLM for compression
	result, err := s.llm.Chat(ctx, []chat.Message{
		chat.NewUserMessage(prompt),
	})
	if err != nil {
		s.logger.Error("compression failed", err, map[string]interface{}{
			"step":  "ContextCompression",
			"query": query,
		})
		return fmt.Errorf("ContextCompression: Chat failed: %w", err)
	}

	// Replace answer with compressed content
	state.Answer = &core.Result{Answer: result.Content}

	s.logger.Info("context compressed successfully", map[string]interface{}{
		"step":          "ContextCompression",
		"chunks_count":  len(allChunks),
		"output_length": len(result.Content),
	})

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("compression", 1)
	}

	return nil
}

// flattenChunks flattens grouped chunks into a single slice.
func (s *compress) flattenChunks(chunks [][]*core.Chunk) []*core.Chunk {
	var all []*core.Chunk
	for _, group := range chunks {
		all = append(all, group...)
	}
	return all
}
