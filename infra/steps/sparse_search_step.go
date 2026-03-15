package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*SparseSearchStep)(nil)

// SparseSearchStep is a pipeline step that performs sparse retrieval (e.g., BM25).
// It's effective for keyword-based search and works well with dense retrieval.
type SparseSearchStep struct {
	sparseIndex abstraction.SparseIndex
	topK        int
}

// NewSparseSearchStep creates a new sparse search step.
//
// Parameters:
// - sparseIndex: The sparse index (BM25) to use for retrieval
// - topK: Number of top results to return (default: 10)
//
// Returns:
// - A new SparseSearchStep instance
func NewSparseSearchStep(sparseIndex abstraction.SparseIndex, topK int) *SparseSearchStep {
	if topK <= 0 {
		topK = 10
	}
	return &SparseSearchStep{
		sparseIndex: sparseIndex,
		topK:        topK,
	}
}

// Name returns the step name
func (s *SparseSearchStep) Name() string {
	return "SparseSearchStep"
}

// Execute performs sparse retrieval using the query text.
// Results are stored in state.RetrievedChunks.
func (s *SparseSearchStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("SparseSearchStep: 'query' not found or empty")
	}

	// Perform sparse search
	results, err := s.sparseIndex.Search(ctx, state.Query.Text, s.topK)
	if err != nil {
		return fmt.Errorf("SparseSearchStep failed to search: %w", err)
	}

	// Convert results to chunks
	var chunks []*entity.Chunk
	for _, result := range results {
		chunk := &entity.Chunk{
			ID:       result.ID,
			Document: result.Document,
			Content:  result.Content,
			Score:    result.Score,
			Metadata: result.Metadata,
		}
		chunks = append(chunks, chunk)
	}

	state.RetrievedChunks = append(state.RetrievedChunks, chunks)

	fmt.Printf("SparseSearchStep: retrieved %d chunks using BM25\n", len(chunks))
	return nil
}
