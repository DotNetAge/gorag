package enhancer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// mockLLMClient is a mock LLM client for testing
type mockLLMClient struct {
	response string
}

func (m *mockLLMClient) Chat(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error) {
	return &core.Response{Content: m.response}, nil
}

func (m *mockLLMClient) ChatStream(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Stream, error) {
	return nil, nil
}

func TestCrossEncoderReranker_Enhance(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse string
		chunkCount   int
		expectedTopK int
		expectRerank bool
	}{
		{
			name:         "successful reranking with scores",
			mockResponse: "[0.9, 0.2, 0.8, 0.1]",
			chunkCount:   4,
			expectedTopK: 4,
			expectRerank: true,
		},
		{
			name:         "rerank with topK truncation",
			mockResponse: "[0.9, 0.2, 0.8, 0.1]",
			chunkCount:   4,
			expectedTopK: 2,
			expectRerank: true,
		},
		{
			name:         "empty chunks",
			mockResponse: "[]",
			chunkCount:   0,
			expectedTopK: 10,
			expectRerank: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			llm := &mockLLMClient{response: tt.mockResponse}
			reranker := NewCrossEncoderReranker(llm, WithRerankTopK(tt.expectedTopK))

			// Create test retrieval results
			chunks := make([]*entity.Chunk, tt.chunkCount)
			scores := make([]float32, tt.chunkCount)
			queryID := uuid.New().String()

			for i := 0; i < tt.chunkCount; i++ {
				chunks[i] = entity.NewChunk(
					uuid.New().String(),
					uuid.New().String(),
					"test content "+string(rune(i)),
					i*10,
					(i+1)*10,
					map[string]any{"index": i},
				)
				scores[i] = float32(i + 1) // Original scores in order
			}

			results := entity.NewRetrievalResult(
				uuid.New().String(),
				queryID,
				chunks,
				scores,
				nil,
			)

			// Execute
			ctx := context.Background()
			rerankedResults, err := reranker.Enhance(ctx, results)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, rerankedResults)

			if tt.expectRerank {
				// Verify chunks are reordered by score (highest first)
				assert.Len(t, rerankedResults.Chunks, min(tt.chunkCount, tt.expectedTopK))
				assert.Len(t, rerankedResults.Scores, min(tt.chunkCount, tt.expectedTopK))

				// Verify scores are in descending order
				for i := 1; i < len(rerankedResults.Scores); i++ {
					assert.GreaterOrEqual(t, rerankedResults.Scores[i-1], rerankedResults.Scores[i])
				}
			} else {
				// Should return original results
				assert.Equal(t, results, rerankedResults)
			}
		})
	}
}

func TestParseScores(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectScore []float32
		expectError bool
	}{
		{
			name:        "valid JSON array",
			content:     "[0.9, 0.8, 0.7]",
			expectScore: []float32{0.9, 0.8, 0.7},
			expectError: false,
		},
		{
			name:        "JSON with whitespace",
			content:     "  [ 0.9 , 0.8 , 0.7 ]  ",
			expectScore: []float32{0.9, 0.8, 0.7},
			expectError: false,
		},
		{
			name:        "invalid format",
			content:     "not a json array",
			expectScore: nil,
			expectError: true,
		},
		{
			name:        "empty array",
			content:     "[]",
			expectScore: nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scores, err := parseScores(tt.content)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectScore, scores)
			}
		})
	}
}

func TestSortByScore(t *testing.T) {
	// Setup
	chunks := []*entity.Chunk{
		entity.NewChunk("c1", "q1", "chunk1", 0, 10, nil),
		entity.NewChunk("c2", "q1", "chunk2", 0, 10, nil),
		entity.NewChunk("c3", "q1", "chunk3", 0, 10, nil),
	}
	oldScores := []float32{0.5, 0.6, 0.7}
	newScores := []float32{0.9, 0.2, 0.8} // Should reorder to: c1, c3, c2

	// Execute
	sortedChunks, sortedScores := sortByScore(chunks, oldScores, newScores)

	// Assert
	assert.Len(t, sortedChunks, 3)
	assert.Len(t, sortedScores, 3)

	// First should be c1 (score 0.9)
	assert.Equal(t, "c1", sortedChunks[0].ID)
	assert.Equal(t, float32(0.9), sortedScores[0])

	// Second should be c3 (score 0.8)
	assert.Equal(t, "c3", sortedChunks[1].ID)
	assert.Equal(t, float32(0.8), sortedScores[1])

	// Third should be c2 (score 0.2)
	assert.Equal(t, "c2", sortedChunks[2].ID)
	assert.Equal(t, float32(0.2), sortedScores[2])
}

// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }
