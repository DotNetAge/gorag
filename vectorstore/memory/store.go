package memory

import (
	"context"
	"sync"

	"github.com/raya-dev/gorag/vectorstore"
)

// Store implements an in-memory vector store
type Store struct {
	mu         sync.RWMutex
	documents  map[string]vectorstore.Chunk
	embeddings map[string][]float32
}

// NewStore creates a new in-memory store
func NewStore() *Store {
	return &Store{
		documents:  make(map[string]vectorstore.Chunk),
		embeddings: make(map[string][]float32),
	}
}

// Add adds chunks to the store
func (s *Store) Add(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, chunk := range chunks {
		s.documents[chunk.ID] = chunk
		s.embeddings[chunk.ID] = embeddings[i]
	}

	return nil
}

// Search performs similarity search
func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []vectorstore.Result

	for id, embedding := range s.embeddings {
		score := cosineSimilarity(query, embedding)
		if score >= opts.MinScore {
			results = append(results, vectorstore.Result{
				Chunk: s.documents[id],
				Score: score,
			})
		}
	}

	return topK(results, opts.TopK), nil
}

// Delete removes chunks from the store
func (s *Store) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.documents, id)
		delete(s.embeddings, id)
	}

	return nil
}

// cosineSimilarity calculates cosine similarity
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float32) float32 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// topK returns top K results by score
func topK(results []vectorstore.Result, k int) []vectorstore.Result {
	if k >= len(results) {
		return results
	}

	for i := 0; i < k; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results[:k]
}
