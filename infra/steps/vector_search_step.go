package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step = (*VectorSearchStep)(nil)

// VectorSearchStep retrieves relevant chunks from a VectorStore based on the Query.
type VectorSearchStep struct {
	embedder abstraction.EmbeddingModel
	store    abstraction.VectorStore
	topK     int
}

// NewVectorSearchStep creates a new vector search step.
func NewVectorSearchStep(embedder abstraction.EmbeddingModel, store abstraction.VectorStore, topK int) *VectorSearchStep {
	if topK <= 0 {
		topK = 10
	}
	return &VectorSearchStep{
		embedder: embedder,
		store:    store,
		topK:     topK,
	}
}

func (s *VectorSearchStep) Execute(ctx context.Context, state *pipeline.State) error {
	query, ok := state.Get("query").(*entity.Query)
	if !ok {
		return fmt.Errorf("VectorSearchStep: 'query' (*entity.Query) not found in state")
	}

	// 1. Embed the query
	queryVector, err := s.embedder.EmbedQuery(ctx, query.Text)
	if err != nil {
		return fmt.Errorf("VectorSearchStep failed to embed query: %w", err)
	}

	// 2. Extract potential filters from state (if FilterExtractorStep ran before this)
	var filters map[string]any
	if f, ok := state.Get("filters").(map[string]any); ok {
		filters = f
	}

	// 3. Search the vector database
	vectors, _, err := s.store.Search(ctx, queryVector, s.topK, filters)
	if err != nil {
		return fmt.Errorf("VectorSearchStep failed to search store: %w", err)
	}

	// 4. Convert Vectors back to Chunks (Assuming the store populates Chunk metadata or content)
	var chunks []*entity.Chunk
	for _, v := range vectors {
		// In a real implementation, VectorStore.Search often returns the document payload in metadata.
		content, _ := v.Metadata["content"].(string)
		chunk := entity.NewChunk(v.ChunkID, "", content, 0, 0, v.Metadata)
		chunks = append(chunks, chunk)
	}

	// 5. Store retrieved chunks in state
	state.Set("retrieved_chunks", chunks)

	return nil
}
