package graph

import (
	"context"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// HybridSearch combines local graph traversal, global community search, and vector search.
// Best for: complex queries needing both specific facts and broader context.
// Following Microsoft GraphRAG: fuses multiple retrieval signals.
type HybridSearch struct {
	graphStore  core.GraphStore
	vectorStore core.VectorStore
	embedder    embedding.Provider
	topK        int
	depth       int
	logger      logging.Logger
}

type HybridSearchOption func(*HybridSearch)

func WithHybridTopK(topK int) HybridSearchOption {
	return func(s *HybridSearch) {
		s.topK = topK
	}
}

func WithHybridDepth(depth int) HybridSearchOption {
	return func(s *HybridSearch) {
		s.depth = depth
	}
}

// NewHybridSearch creates a hybrid search step combining multiple retrieval strategies.
func NewHybridSearch(graphStore core.GraphStore, vectorStore core.VectorStore, embedder embedding.Provider, opts ...HybridSearchOption) pipeline.Step[*core.RetrievalContext] {
	s := &HybridSearch{
		graphStore:  graphStore,
		vectorStore: vectorStore,
		embedder:    embedder,
		topK:        5,
		depth:       2,
		logger:      logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *HybridSearch) Name() string {
	return "HybridSearch"
}

func (s *HybridSearch) Execute(ctx context.Context, rctx *core.RetrievalContext) error {
	s.logger.Info("Starting hybrid search", map[string]any{
		"query": rctx.Query.Text,
		"topK":  s.topK,
	})

	// 1. Local search (if entities extracted)
	localChunks := s.localSearch(ctx, rctx)

	// 2. Global search (community summaries)
	globalChunks := s.globalSearch(ctx, rctx)

	// 3. Vector search
	vectorChunks := s.vectorSearch(ctx, rctx)

	// 4. Fuse results
	allChunks := s.fuseResults(localChunks, globalChunks, vectorChunks)

	// Store in context
	rctx.RetrievedChunks = append(rctx.RetrievedChunks, allChunks)

	s.logger.Info("Hybrid search completed", map[string]any{
		"local_chunks":  len(localChunks),
		"global_chunks": len(globalChunks),
		"vector_chunks": len(vectorChunks),
		"total_chunks":  len(allChunks),
	})

	return nil
}

func (s *HybridSearch) localSearch(ctx context.Context, rctx *core.RetrievalContext) []*core.Chunk {
	entities, ok := rctx.Custom["extracted_entities"].([]string)
	if !ok || len(entities) == 0 || s.graphStore == nil {
		return nil
	}

	var chunks []*core.Chunk
	chunkSet := make(map[string]bool)

	for _, entity := range entities {
		nodes, _, err := s.graphStore.GetNeighbors(ctx, entity, s.depth, s.topK)
		if err != nil {
			continue
		}

		for _, node := range nodes {
			for _, chunkID := range node.SourceChunkIDs {
				if !chunkSet[chunkID] {
					chunkSet[chunkID] = true
					// Create a placeholder chunk - actual content will be enriched later
					chunks = append(chunks, &core.Chunk{
						ID: chunkID,
					})
				}
			}
		}
	}

	return chunks
}

func (s *HybridSearch) globalSearch(ctx context.Context, rctx *core.RetrievalContext) []*core.Chunk {
	if s.graphStore == nil {
		return nil
	}

	// Get community summaries
	communities, err := s.graphStore.GetCommunitySummaries(ctx, 0)
	if err != nil || len(communities) == 0 {
		return nil
	}

	var chunks []*core.Chunk

	// Match communities to query (simplified keyword matching)
	queryLower := rctx.Query.Text
	for _, community := range communities {
		summary, ok := community["summary"].(string)
		if !ok || summary == "" {
			continue
		}

		// Check if summary is relevant
		if s.isRelevant(queryLower, summary) {
			// Get source chunks from community
			if chunkIDs, ok := community["chunks"].([]string); ok {
				for _, chunkID := range chunkIDs {
					chunks = append(chunks, &core.Chunk{
						ID: chunkID,
					})
				}
			}
		}
	}

	return chunks
}

func (s *HybridSearch) vectorSearch(ctx context.Context, rctx *core.RetrievalContext) []*core.Chunk {
	if s.vectorStore == nil || s.embedder == nil {
		return nil
	}

	// Embed query
	queryVecs, err := s.embedder.Embed(ctx, []string{rctx.Query.Text})
	if err != nil {
		s.logger.Warn("Failed to embed query for vector search", map[string]any{"error": err.Error()})
		return nil
	}
	queryVec := queryVecs[0]

	// Search vectors
	vectors, scores, err := s.vectorStore.Search(ctx, queryVec, s.topK, nil)
	if err != nil {
		s.logger.Warn("Vector search failed", map[string]any{"error": err.Error()})
		return nil
	}

	var chunks []*core.Chunk
	for i, vec := range vectors {
		chunks = append(chunks, &core.Chunk{
			ID:       vec.ChunkID,
			VectorID: vec.ID,
			Metadata: map[string]any{"vector_score": scores[i]},
		})
	}

	return chunks
}

func (s *HybridSearch) fuseResults(local, global, vector []*core.Chunk) []*core.Chunk {
	// Deduplicate by chunk ID
	chunkMap := make(map[string]*core.Chunk)

	// Add local results (highest priority)
	for _, chunk := range local {
		if chunk.ID != "" {
			chunkMap[chunk.ID] = chunk
		}
	}

	// Add global results
	for _, chunk := range global {
		if chunk.ID != "" && chunkMap[chunk.ID] == nil {
			chunkMap[chunk.ID] = chunk
		}
	}

	// Add vector results
	for _, chunk := range vector {
		if chunk.ID != "" && chunkMap[chunk.ID] == nil {
			chunkMap[chunk.ID] = chunk
		}
	}

	// Convert to slice
	var result []*core.Chunk
	for _, chunk := range chunkMap {
		result = append(result, chunk)
	}

	return result
}

func (s *HybridSearch) isRelevant(query, text string) bool {
	// Simple relevance check - can be enhanced
	queryWords := splitWords(query)
	textLower := text
	matches := 0

	for _, word := range queryWords {
		if len(word) > 3 && containsWord(textLower, word) {
			matches++
		}
	}

	return matches >= 2 // At least 2 word matches
}

func splitWords(s string) []string {
	var words []string
	current := ""
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}

func containsWord(text, word string) bool {
	if len(text) < len(word) {
		return false
	}
	if text == word {
		return true
	}
	if text[:len(word)] == word {
		return true
	}
	if text[len(text)-len(word):] == word {
		return true
	}
	return findSubstring(text, word)
}

func findSubstring(text, word string) bool {
	for i := 0; i <= len(text)-len(word); i++ {
		if text[i:i+len(word)] == word {
			return true
		}
	}
	return false
}
