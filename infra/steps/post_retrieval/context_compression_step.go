// Package post_retrieval provides steps that process and optimize retrieval results.
package post_retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ContextCompressionStep compresses retrieved chunks to extract only the most relevant information.
type ContextCompressionStep struct {
	llm       core.Client
	logger    logging.Logger
	maxTokens int
}

// NewContextCompressionStep creates a new context compression step.
func NewContextCompressionStep(llm core.Client, logger logging.Logger, maxTokens int) *ContextCompressionStep {
	if maxTokens <= 0 {
		maxTokens = 300
	}
	return &ContextCompressionStep{
		llm:       llm,
		logger:    logger,
		maxTokens: maxTokens,
	}
}

// Name returns the step name.
func (s *ContextCompressionStep) Name() string {
	return "ContextCompressionStep"
}

// Execute compresses all retrieved chunks to extract only relevant information.
func (s *ContextCompressionStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if len(state.RetrievedChunks) == 0 {
		return nil
	}

	query := state.Query.Text
	if query == "" {
		s.logger.Warn("ContextCompressionStep: empty query, skipping compression")
		return nil
	}

	// Flatten all chunks
	allChunks := flattenChunks(state.RetrievedChunks)
	compressedChunks := make([]*entity.Chunk, 0, len(allChunks))

	for _, chunk := range allChunks {
		compressed, err := s.compressChunk(ctx, query, chunk)
		if err != nil {
			s.logger.Warn("Failed to compress chunk", map[string]interface{}{
				"error":     err.Error(),
				"chunk_id":  chunk.ID,
				"chunk_len": len(chunk.Content),
			})
			// Keep original chunk if compression fails
			compressedChunks = append(compressedChunks, chunk)
			continue
		}

		if compressed != "" {
			compressedChunks = append(compressedChunks, &entity.Chunk{
				ID:         chunk.ID,
				DocumentID: chunk.DocumentID,
				Content:    compressed,
				Metadata:   chunk.Metadata,
				StartIndex: chunk.StartIndex,
				EndIndex:   chunk.EndIndex,
			})
		}
	}

	state.RetrievedChunks = [][]*entity.Chunk{compressedChunks}

	s.logger.Debug("Context compression completed", map[string]interface{}{
		"original_count":   len(allChunks),
		"compressed_count": len(compressedChunks),
	})

	return nil
}

// compressChunk compresses a single chunk to extract relevant information.
func (s *ContextCompressionStep) compressChunk(ctx context.Context, query string, chunk *entity.Chunk) (string, error) {
	prompt := s.buildCompressionPrompt(query, chunk.Content)

	messages := []core.Message{
		{Role: core.RoleUser, Content: []core.ContentBlock{{Type: core.ContentTypeText, Text: prompt}}},
	}

	response, err := s.llm.Chat(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM compression failed: %w", err)
	}

	// Use response.Content directly (it's already a string)
	return response.Content, nil
}

// buildCompressionPrompt constructs the prompt for context compression.
func (s *ContextCompressionStep) buildCompressionPrompt(query, content string) string {
	return fmt.Sprintf(`You are an expert at extracting and compressing information for question answering.

Your task is to extract ONLY the most relevant information from the following text that helps answer the query.

Query: "%s"

Text to compress:
---
%s
---

Instructions:
1. Extract only information directly relevant to the query
2. Remove redundant or irrelevant details
3. Preserve key facts, numbers, and specific details
4. Keep the compressed text concise but complete
5. Do NOT add information not present in the original text
6. If the text contains no relevant information, return "NO_RELEVANT_INFO"

Compressed text (max %d tokens):`, query, content, s.maxTokens/4)
}

// flattenChunks converts [][]*Chunk to []*Chunk
func flattenChunks(chunks [][]*entity.Chunk) []*entity.Chunk {
	var result []*entity.Chunk
	for _, group := range chunks {
		result = append(result, group...)
	}
	return result
}
