package retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*SparseSearchStep)(nil)

// SparseSearchStep is a pipeline step that performs sparse retrieval (e.g., BM25).
// It's effective for keyword-based search and works well with dense retrieval.
type SparseSearchStep struct {
	searcher SparseSearcher
	topK     int
	logger   logging.Logger
}

// SparseSearcher defines the interface for sparse search operations.
type SparseSearcher interface {
	Search(ctx context.Context, query string, topK int) ([]*SearchResult, error)
}

// SearchResult represents a search result.
type SearchResult struct {
	ID       string
	Content  string
	Metadata map[string]any
}

// NewSparseSearchStep creates a new sparse search step.
//
// Parameters:
// - searcher: The sparse searcher to use for retrieval
// - topK: Number of top results to return (default: 10)
// - logger: optional structured logger; pass nil to use noop
//
// Returns:
// - A new SparseSearchStep instance
func NewSparseSearchStep(searcher SparseSearcher, topK int, logger logging.Logger) *SparseSearchStep {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &SparseSearchStep{
		searcher: searcher,
		topK:     topK,
		logger:   logger,
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
	results, err := s.searcher.Search(ctx, state.Query.Text, s.topK)
	if err != nil {
		return fmt.Errorf("SparseSearchStep failed to search: %w", err)
	}

	// Convert results to chunks
	var chunks []*entity.Chunk
	for _, result := range results {
		chunk := entity.NewChunk(
			result.ID,
			"", // DocumentID will be set by the indexer
			result.Content,
			0,                   // StartIndex
			len(result.Content), // EndIndex
			result.Metadata,
		)
		chunks = append(chunks, chunk)
	}

	state.RetrievedChunks = append(state.RetrievedChunks, chunks)

	s.logger.Info("SparseSearchStep completed", map[string]interface{}{
		"step":         "SparseSearchStep",
		"chunks_count": len(chunks),
	})
	return nil
}
