package post_retrieval

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// mockResultEnhancer is a mock implementation of retrieval.ResultEnhancer
type mockResultEnhancer struct {
	enhanceFunc func(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error)
}

func newMockResultEnhancer(enhanceFunc func(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error)) *mockResultEnhancer {
	return &mockResultEnhancer{
		enhanceFunc: enhanceFunc,
	}
}

func (m *mockResultEnhancer) Enhance(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
	if m.enhanceFunc == nil {
		return results, nil
	}
	return m.enhanceFunc(ctx, results)
}

func TestCrossEncoderRerankStep_Execute(t *testing.T) {
	tests := []struct {
		name        string
		state       *entity.PipelineState
		expectError bool
		errorMsg    string
	}{
		{
			name: "normal execution with chunks",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{
					{
						{ID: "c1", Content: "chunk 1"},
						{ID: "c2", Content: "chunk 2"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty chunks - pass through",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{},
			},
			expectError: false,
		},
		{
			name: "nil query - should error",
			state: &entity.PipelineState{
				RetrievedChunks: [][]*entity.Chunk{
					{{ID: "c1", Content: "chunk 1"}},
				},
			},
			expectError: true,
			errorMsg:    "'query' not found in state",
		},
		{
			name: "enhancer returns error",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{
					{{ID: "c1", Content: "chunk 1"}},
				},
			},
			expectError: true,
			errorMsg:    "enhance failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock enhancer
			mockEnhancer := newMockResultEnhancer(func(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
				if tt.errorMsg != "" && tt.expectError {
					return nil, assert.AnError
				}
				// Return enhanced result with reordered chunks
				return entity.NewRetrievalResult(
					results.ID,
					results.QueryID,
					results.Chunks,
					make([]float32, len(results.Chunks)),
					nil,
				), nil
			})

			logger := logging.NewNoopLogger()
			step := NewCrossEncoderRerankStep(mockEnhancer, logger)

			// Execute
			ctx := context.Background()
			err := step.Execute(ctx, tt.state)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify state was updated
				assert.NotEmpty(t, tt.state.RetrievedChunks)
			}
		})
	}
}

func TestContextPruningStep_Execute(t *testing.T) {
	tests := []struct {
		name        string
		state       *entity.PipelineState
		expectError bool
		errorMsg    string
	}{
		{
			name: "normal execution with chunks",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{
					{
						{ID: "c1", Content: "relevant chunk 1"},
						{ID: "c2", Content: "irrelevant chunk 2"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty chunks - pass through",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{},
			},
			expectError: false,
		},
		{
			name: "nil query - should error",
			state: &entity.PipelineState{
				RetrievedChunks: [][]*entity.Chunk{
					{{ID: "c1", Content: "chunk 1"}},
				},
			},
			expectError: true,
			errorMsg:    "'query' not found in state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock enhancer
			mockEnhancer := newMockResultEnhancer(func(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
				// Simulate pruning by returning fewer chunks
				if len(results.Chunks) > 1 {
					return entity.NewRetrievalResult(
						results.ID,
						results.QueryID,
						results.Chunks[:1], // Keep only first chunk
						make([]float32, 1),
						nil,
					), nil
				}
				return results, nil
			})

			logger := logging.NewNoopLogger()
			step := NewContextPruningStep(mockEnhancer, logger)

			// Execute
			ctx := context.Background()
			err := step.Execute(ctx, tt.state)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify state was updated
				assert.NotEmpty(t, tt.state.RetrievedChunks)
			}
		})
	}
}

func TestParentDocExpandStep_Execute(t *testing.T) {
	tests := []struct {
		name        string
		state       *entity.PipelineState
		expectError bool
		errorMsg    string
	}{
		{
			name: "normal execution with child chunks",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{
					{
						{ID: "c1", ParentID: "doc1", Content: "child chunk 1"},
						{ID: "c2", ParentID: "doc1", Content: "child chunk 2"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty chunks - pass through",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{},
			},
			expectError: false,
		},
		{
			name: "nil query - should error",
			state: &entity.PipelineState{
				RetrievedChunks: [][]*entity.Chunk{
					{{ID: "c1", Content: "chunk 1"}},
				},
			},
			expectError: true,
			errorMsg:    "'query' not found in state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock enhancer
			mockEnhancer := newMockResultEnhancer(func(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
				// Simulate expansion by returning parent docs
				expandedChunks := make([]*entity.Chunk, len(results.Chunks))
				for i, chunk := range results.Chunks {
					expandedChunks[i] = &entity.Chunk{
						ID:      chunk.ID,
						Content: "Expanded parent document content",
						Metadata: map[string]any{
							"expanded_from": chunk.ID,
						},
					}
				}
				return entity.NewRetrievalResult(
					results.ID,
					results.QueryID,
					expandedChunks,
					make([]float32, len(expandedChunks)),
					nil,
				), nil
			})

			logger := logging.NewNoopLogger()
			step := NewParentDocExpandStep(mockEnhancer, logger)

			// Execute
			ctx := context.Background()
			err := step.Execute(ctx, tt.state)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify state was updated
				assert.NotEmpty(t, tt.state.RetrievedChunks)
			}
		})
	}
}

func TestSentenceWindowExpandStep_Execute(t *testing.T) {
	tests := []struct {
		name        string
		state       *entity.PipelineState
		expectError bool
		errorMsg    string
	}{
		{
			name: "normal execution with chunks",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{
					{
						{ID: "c1", Content: "middle sentence"},
						{ID: "c2", Content: "another sentence"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty chunks - pass through",
			state: &entity.PipelineState{
				Query: &entity.Query{
					ID:   uuid.New().String(),
					Text: "test query",
				},
				RetrievedChunks: [][]*entity.Chunk{},
			},
			expectError: false,
		},
		{
			name: "nil query - should error",
			state: &entity.PipelineState{
				RetrievedChunks: [][]*entity.Chunk{
					{{ID: "c1", Content: "chunk 1"}},
				},
			},
			expectError: true,
			errorMsg:    "'query' not found in state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock enhancer
			mockEnhancer := newMockResultEnhancer(func(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
				// Simulate sentence window expansion
				expandedChunks := make([]*entity.Chunk, len(results.Chunks))
				for i, chunk := range results.Chunks {
					expandedChunks[i] = &entity.Chunk{
						ID:      chunk.ID,
						Content: "Previous sentence. " + chunk.Content + ". Next sentence.",
						Metadata: map[string]any{
							"window_expanded": true,
						},
					}
				}
				return entity.NewRetrievalResult(
					results.ID,
					results.QueryID,
					expandedChunks,
					make([]float32, len(expandedChunks)),
					nil,
				), nil
			})

			logger := logging.NewNoopLogger()
			step := NewSentenceWindowExpandStep(mockEnhancer, logger)

			// Execute
			ctx := context.Background()
			err := step.Execute(ctx, tt.state)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify state was updated
				assert.NotEmpty(t, tt.state.RetrievedChunks)
			}
		})
	}
}
