// Package vector provides vector search steps for RAG retrieval pipelines.
package stepvec

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// search retrieves relevant chunks from a VectorStore based on query embedding.
type search struct {
	embedder embedding.Provider
	store    core.VectorStore
	topK     int
	logger   logging.Logger
	metrics  core.Metrics
}

// Search creates a new vector search step with logger and metrics.
//
// Parameters:
//   - embedder: embedding provider (required for text queries)
//   - store: vector store to search
//   - topK: number of results to retrieve (default: 10)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(vector.Search(embedder, store, 20, logger, metrics))
func Search(
	embedder embedding.Provider,
	store core.VectorStore,
	topK int,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.State] {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &search{
		embedder: embedder,
		store:    store,
		topK:     topK,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *search) Name() string {
	return "VectorSearch"
}

// Execute retrieves relevant chunks by embedding the query and searching the vector store.
func (s *search) Execute(ctx context.Context, state *core.State) error {
	if state.Query == nil {
		return fmt.Errorf("VectorSearch: 'query' not found in state")
	}

	// Obtain query vector
	var queryVector []float32
	if s.embedder != nil {
		// Generate embedding for query text
		embeddings, err := s.embedder.Embed(ctx, []string{state.Query.Text})
		if err != nil {
			return fmt.Errorf("VectorSearch failed to embed query: %w", err)
		}
		if len(embeddings) == 0 {
			return fmt.Errorf("VectorSearch failed to get query embedding")
		}
		queryVector = embeddings[0]
	} else {
		// Use pre-computed vector from AgenticMetadata (multimodal pipeline)
		if state.Agentic == nil {
			return fmt.Errorf("VectorSearch: embedder is nil and no AgenticMetadata found")
		}
		vec, ok := state.Agentic.Custom["query_vector"].([]float32)
		if !ok || len(vec) == 0 {
			return fmt.Errorf("VectorSearch: embedder is nil and query_vector not set")
		}
		queryVector = vec
	}

	// Use filters from state (if FilterExtractorStep ran before this)
	filters := state.Filters

	// Search the vector database
	vectors, _, err := s.store.Search(ctx, queryVector, s.topK, filters)
	if err != nil {
		return fmt.Errorf("VectorSearch failed to search store: %w", err)
	}

	// Convert vectors back to chunks
	var chunks []*core.Chunk
	for _, v := range vectors {
		content := ""
		if v.Metadata != nil {
			if c, ok := v.Metadata["content"].(string); ok {
				content = c
			}
		}
		chunk := &core.Chunk{
			ID:       v.ChunkID,
			Content:  content,
			Metadata: v.Metadata,
			VectorID: v.ID,
		}
		chunks = append(chunks, chunk)
	}

	// Store retrieved chunks in state (RetrievedChunks is [][]*Chunk)
	state.RetrievedChunks = append(state.RetrievedChunks, chunks)
	
	if state.ParallelResults == nil {
		state.ParallelResults = make(map[string][]*core.Chunk)
	}
	state.ParallelResults["vector"] = chunks

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("vector", len(chunks))
	}

	s.logger.Debug("vector search completed", map[string]interface{}{
		"step":    "VectorSearch",
		"query":   state.Query.Text,
		"results": len(chunks),
		"topK":    s.topK,
	})

	return nil
}
