package enhancer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestContextPruner_Enhance(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  string
		chunks        []*entity.Chunk
		maxTokens     int
		expectPrune   bool
		expectedCount int
	}{
		{
			name: "successful pruning with relevance scores",
			mockResponse: `[
				{"chunk_index": 0, "relevance": 0.9, "reason": "direct answer"},
				{"chunk_index": 1, "relevance": 0.2, "reason": "irrelevant"},
				{"chunk_index": 2, "relevance": 0.8, "reason": "useful info"},
				{"chunk_index": 3, "relevance": 0.1, "reason": "noise"}
			]`,
			chunks: []*entity.Chunk{
				entity.NewChunk("c1", "q1", "This is the answer content", 0, 30, nil),
				entity.NewChunk("c2", "q1", "Irrelevant information here", 0, 30, nil),
				entity.NewChunk("c3", "q1", "More useful details about the topic", 0, 40, nil),
				entity.NewChunk("c4", "q1", "Completely unrelated noise", 0, 30, nil),
			},
			maxTokens:     100,
			expectPrune:   true,
			expectedCount: 3, // Should keep top 3 relevant chunks (0.9, 0.8, 0.2) within token limit
		},
		{
			name:         "empty chunks",
			mockResponse: "[]",
			chunks:       []*entity.Chunk{},
			maxTokens:    1000,
			expectPrune:  false,
		},
		{
			name: "max tokens limit",
			mockResponse: `[
				{"chunk_index": 0, "relevance": 0.95, "reason": "very relevant"},
				{"chunk_index": 1, "relevance": 0.9, "reason": "relevant"},
				{"chunk_index": 2, "relevance": 0.85, "reason": "somewhat relevant"}
			]`,
			chunks: []*entity.Chunk{
				entity.NewChunk("c1", "q1", "Long content "+string(make([]byte, 500)), 0, 500, nil),
				entity.NewChunk("c2", "q1", "Another long content "+string(make([]byte, 500)), 0, 500, nil),
				entity.NewChunk("c3", "q1", "Third long content "+string(make([]byte, 500)), 0, 500, nil),
			},
			maxTokens:     1200, // Should only fit 2 chunks
			expectPrune:   true,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			llm := &mockLLMClient{response: tt.mockResponse}
			pruner := NewContextPruner(llm, WithPrunerMaxTokens(tt.maxTokens))

			results := entity.NewRetrievalResult(
				uuid.New().String(),
				uuid.New().String(),
				tt.chunks,
				make([]float32, len(tt.chunks)),
				nil,
			)

			// Execute
			ctx := context.Background()
			prunedResults, err := pruner.Enhance(ctx, results)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, prunedResults)

			if tt.expectPrune {
				assert.Len(t, prunedResults.Chunks, tt.expectedCount)
				// Verify chunks are ordered by relevance
				if len(prunedResults.Chunks) > 1 {
					assert.GreaterOrEqual(t, prunedResults.Scores[0], prunedResults.Scores[1])
				}
			} else {
				assert.Equal(t, results, prunedResults)
			}
		})
	}
}

func TestParseRelevanceScores(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		expectLen   int
	}{
		{
			name: "valid JSON array of objects",
			content: `[
				{"chunk_index": 0, "relevance": 0.9, "reason": "good"},
				{"chunk_index": 1, "relevance": 0.5, "reason": "ok"}
			]`,
			expectError: false,
			expectLen:   2,
		},
		{
			name:        "invalid format",
			content:     "not json",
			expectError: true,
		},
		{
			name:        "empty array",
			content:     "[]",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scores, err := parseRelevanceScores(tt.content)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, scores, tt.expectLen)
			}
		})
	}
}

func TestCountTokens(t *testing.T) {
	tests := []struct {
		name     string
		chunks   []*entity.Chunk
		expected int
	}{
		{
			name: "single chunk",
			chunks: []*entity.Chunk{
				entity.NewChunk("c1", "q1", "Hello World", 0, 11, nil),
			},
			expected: 2, // 11 chars / 4 ≈ 2-3 tokens
		},
		{
			name: "multiple chunks",
			chunks: []*entity.Chunk{
				entity.NewChunk("c1", "q1", "Content 1", 0, 9, nil),
				entity.NewChunk("c2", "q1", "Content 2", 0, 9, nil),
			},
			expected: 4, // 18 chars / 4 ≈ 4-5 tokens
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := countTokens(tt.chunks)
			// Allow some variance in token estimation
			assert.InDelta(t, tt.expected, count, 2)
		})
	}
}
