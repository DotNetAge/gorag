package memory

import (
	"context"
	"math"
	"sync"

	"github.com/DotNetAge/gorag/vectorstore"
)

// Store implements an in-memory vector store
type Store struct {
	mu         sync.RWMutex
	documents  map[string]vectorstore.Chunk
	embeddings map[string][]float32
	// Precomputed norms for faster similarity calculation
	norms map[string]float32
}

// NewStore creates a new in-memory store
func NewStore() *Store {
	return &Store{
		documents:  make(map[string]vectorstore.Chunk),
		embeddings: make(map[string][]float32),
		norms:      make(map[string]float32),
	}
}

// Add adds chunks to the store
func (s *Store) Add(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, chunk := range chunks {
		s.documents[chunk.ID] = chunk
		s.embeddings[chunk.ID] = embeddings[i]
		s.norms[chunk.ID] = computeNorm(embeddings[i])
	}

	return nil
}

// Search performs similarity search
func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Precompute query norm for faster calculation
	queryNorm := computeNorm(query)

	// Use a fixed-size slice for results to reduce allocations
	capacity := len(s.embeddings)
	if capacity > 1000 { // Limit capacity for large stores
		capacity = 1000
	}
	results := make([]vectorstore.Result, 0, capacity)

	for id, embedding := range s.embeddings {
		score := cosineSimilarity(query, embedding, queryNorm, s.norms[id])
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
		delete(s.norms, id)
	}

	return nil
}

// computeNorm calculates the L2 norm of a vector
func computeNorm(v []float32) float32 {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	return float32(math.Sqrt(float64(sum)))
}

// cosineSimilarity calculates cosine similarity using precomputed norms
func cosineSimilarity(a, b []float32, normA, normB float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	var dotProduct float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
	}

	return dotProduct / (normA * normB)
}

// topK returns top K results by score using a more efficient algorithm
func topK(results []vectorstore.Result, k int) []vectorstore.Result {
	if k <= 0 || len(results) == 0 {
		return []vectorstore.Result{}
	}

	if k >= len(results) {
		// Sort the entire slice if k is larger than the number of results
		quickSort(results, 0, len(results)-1)
		return results
	}

	// Use quickselect to find the top k results
	quickSelect(results, 0, len(results)-1, k)
	// Sort the top k results
	quickSort(results[:k], 0, k-1)
	return results[:k]
}

// quickSort sorts results by score in descending order
func quickSort(results []vectorstore.Result, low, high int) {
	if low < high {
		pivot := partition(results, low, high)
		quickSort(results, low, pivot-1)
		quickSort(results, pivot+1, high)
	}
}

// partition is used by quickSort
func partition(results []vectorstore.Result, low, high int) int {
	pivot := results[high].Score
	i := low - 1
	for j := low; j < high; j++ {
		if results[j].Score >= pivot {
			i++
			results[i], results[j] = results[j], results[i]
		}
	}
	results[i+1], results[high] = results[high], results[i+1]
	return i + 1
}

// quickSelect finds the k-th largest element
func quickSelect(results []vectorstore.Result, low, high, k int) {
	if low < high {
		pivot := partition(results, low, high)
		if pivot == k-1 {
			return
		} else if pivot > k-1 {
			quickSelect(results, low, pivot-1, k)
		} else {
			quickSelect(results, pivot+1, high, k)
		}
	}
}
