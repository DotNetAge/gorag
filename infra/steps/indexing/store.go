package indexing

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/indexing"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
)

// upsert stores vectors and chunks into the vector store.
type upsert struct {
	vectorStore abstraction.VectorStore
	metrics     abstraction.Metrics
}

// Upsert creates a new upsert step with metrics collection.
//
// Parameters:
//   - vectorStore: vector store implementation
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(indexing.Upsert(vectorStore, metrics))
func Upsert(vectorStore abstraction.VectorStore, metrics abstraction.Metrics) pipeline.Step[*indexing.State] {
	return &upsert{
		vectorStore: vectorStore,
		metrics:     metrics,
	}
}

// Name returns the step name
func (s *upsert) Name() string {
	return "Store"
}

// Execute stores all vectors and chunks into the vector database.
func (s *upsert) Execute(ctx context.Context, state *indexing.State) error {
	if s.vectorStore == nil {
		return fmt.Errorf("vector store not configured")
	}

	// Get vectors from state
	if state.Vectors == nil || len(state.Vectors) == 0 {
		return fmt.Errorf("no vectors to store")
	}

	// TODO: Implement actual storage logic based on VectorStore interface
	// This is a placeholder until the interface is unified
	// The actual implementation should:
	// 1. Convert entity.Vector to the format expected by VectorStore
	// 2. Call vectorStore.Upsert or equivalent method
	// 3. Handle errors appropriately

	// Record VectorStore operation metrics
	if s.metrics != nil && state.TotalChunks > 0 {
		s.metrics.RecordVectorStoreOperations("store", state.TotalChunks)
	}

	return nil
}
