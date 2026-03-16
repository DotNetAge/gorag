package retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*VectorSearchStep)(nil)

// VectorSearchStep retrieves relevant chunks from a VectorStore based on the Query.
type VectorSearchStep struct {
	embedder embedding.Provider
	store    abstraction.VectorStore
	topK     int
}

// NewVectorSearchStep creates a new vector search step.
func NewVectorSearchStep(embedder embedding.Provider, store abstraction.VectorStore, topK int) *VectorSearchStep {
	if topK <= 0 {
		topK = 10
	}
	return &VectorSearchStep{
		embedder: embedder,
		store:    store,
		topK:     topK,
	}
}

func (s *VectorSearchStep) Name() string {
	return "VectorSearchStep"
}

func (s *VectorSearchStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("VectorSearchStep: 'query' not found in state")
	}

	// 1. Obtain the query vector.
	// When embedder is nil (multimodal pipeline), use the pre-computed vector written
	// by MultimodalEmbeddingStep into state.Agentic.Custom["query_vector"].
	var queryVector []float32
	if s.embedder != nil {
		embeddings, err := s.embedder.Embed(ctx, []string{state.Query.Text})
		if err != nil {
			return fmt.Errorf("VectorSearchStep failed to embed query: %w", err)
		}
		if len(embeddings) == 0 {
			return fmt.Errorf("VectorSearchStep failed to get query embedding")
		}
		queryVector = embeddings[0]
	} else {
		// Expect MultimodalEmbeddingStep to have populated query_vector.
		if state.Agentic == nil {
			return fmt.Errorf("VectorSearchStep: embedder is nil and no AgenticMetadata found")
		}
		vec, ok := state.Agentic.Custom["query_vector"].([]float32)
		if !ok || len(vec) == 0 {
			return fmt.Errorf("VectorSearchStep: embedder is nil and query_vector not set in AgenticMetadata")
		}
		queryVector = vec
	}

	// 2. Use filters from state (if FilterExtractorStep ran before this)
	filters := state.Filters

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
	state.RetrievedChunks = append(state.RetrievedChunks, chunks)

	return nil
}
