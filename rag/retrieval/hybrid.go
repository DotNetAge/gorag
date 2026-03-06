package retrieval

import (
	"context"
	"sort"

	"github.com/DotNetAge/gorag/vectorstore"
)

// HybridRetriever implements hybrid search (vector + keyword)
type HybridRetriever struct {
	vectorStore vectorstore.Store
	keywordStore KeywordStore
	alpha        float32 // Weight between vector (0) and keyword (1) search
}

// KeywordStore defines the interface for keyword search
type KeywordStore interface {
	Search(ctx context.Context, query string, topK int) ([]vectorstore.Result, error)
}

// NewHybridRetriever creates a new hybrid retriever
func NewHybridRetriever(vectorStore vectorstore.Store, keywordStore KeywordStore, alpha float32) *HybridRetriever {
	if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}

	return &HybridRetriever{
		vectorStore:  vectorStore,
		keywordStore: keywordStore,
		alpha:        alpha,
	}
}

// Search performs hybrid search
func (r *HybridRetriever) Search(ctx context.Context, query string, embedding []float32, topK int) ([]vectorstore.Result, error) {
	// Perform vector search
	vectorResults, err := r.vectorStore.Search(ctx, embedding, vectorstore.SearchOptions{
		TopK: topK * 2, // Get more results for reranking
	})
	if err != nil {
		return nil, err
	}

	// Perform keyword search
	keywordResults, err := r.keywordStore.Search(ctx, query, topK*2)
	if err != nil {
		return nil, err
	}

	// Combine and rerank results
	return r.combineResults(vectorResults, keywordResults, topK), nil
}

// combineResults combines vector and keyword search results
func (r *HybridRetriever) combineResults(vectorResults, keywordResults []vectorstore.Result, topK int) []vectorstore.Result {
	// Create a map to store combined results
	resultMap := make(map[string]vectorstore.Result)

	// Add vector results with weight
	for _, result := range vectorResults {
		result.Score = result.Score * (1 - r.alpha)
		resultMap[result.ID] = result
	}

	// Add keyword results with weight
	for _, result := range keywordResults {
		result.Score = result.Score * r.alpha
		if existing, ok := resultMap[result.ID]; ok {
			// Combine scores
			existing.Score += result.Score
			resultMap[result.ID] = existing
		} else {
			resultMap[result.ID] = result
		}
	}

	// Convert map to slice
	var combinedResults []vectorstore.Result
	for _, result := range resultMap {
		combinedResults = append(combinedResults, result)
	}

	// Sort by score
	sort.Slice(combinedResults, func(i, j int) bool {
		return combinedResults[i].Score > combinedResults[j].Score
	})

	// Return top K results
	if len(combinedResults) > topK {
		combinedResults = combinedResults[:topK]
	}

	return combinedResults
}
