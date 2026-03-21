package memory

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"math"
	"sync"
)

// ensure interface implementation
var _ core.VectorStore = (*Store)(nil)

type scoredVector struct {
	vector *core.Vector
	score  float32
}

// Store implements an in-memory vector store
type Store struct {
	mu         sync.RWMutex
	vectors    map[string]*core.Vector
	// Precomputed norms for faster similarity calculation
	norms map[string]float32
}

// NewStore creates a new in-memory store
func NewStore() core.VectorStore {
	return &Store{
		vectors: make(map[string]*core.Vector),
		norms:   make(map[string]float32),
	}
}

// Upsert adds multiple vectors to the store in batch.
func (s *Store) Upsert(ctx context.Context, vectors []*core.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range vectors {
		if v == nil || len(v.Values) == 0 {
			continue
		}
		s.vectors[v.ID] = v
		s.norms[v.ID] = computeNorm(v.Values)
	}

	return nil
}

// Search searches for similar vectors based on the query vector.
func (s *Store) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if topK <= 0 {
		topK = 5
	}

	// Precompute query norm for faster calculation
	queryNorm := computeNorm(query)

	var results []scoredVector

	for id, vec := range s.vectors {
		// Apply metadata filters if provided
		if !s.matchesMetadata(vec.Metadata, filters) {
			continue
		}

		score := cosineSimilarity(query, vec.Values, queryNorm, s.norms[id])
		results = append(results, scoredVector{
			vector: vec,
			score:  score,
		})
	}

	// Sort and extract TopK
	topKResults := getTopK(results, topK)
	
	var outVectors []*core.Vector
	var outScores []float32
	
	for _, res := range topKResults {
		outVectors = append(outVectors, res.vector)
		outScores = append(outScores, res.score)
	}

	return outVectors, outScores, nil
}

// matchesMetadata checks if vector metadata matches the filters
func (s *Store) matchesMetadata(vectorMeta map[string]any, filters map[string]any) bool {
	if len(filters) == 0 {
		return true
	}
	if len(vectorMeta) == 0 {
		return false
	}

	for key, filterVal := range filters {
		vecVal, exists := vectorMeta[key]
		if !exists || vecVal != filterVal {
			return false
		}
	}

	return true
}

// Delete deletes a vector from the store.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.vectors, id)
	delete(s.norms, id)

	return nil
}

func (s *Store) Close(ctx context.Context) error {
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

// getTopK returns top K results by score
func getTopK(results []scoredVector, k int) []scoredVector {
	if k <= 0 || len(results) == 0 {
		return nil
	}

	if k >= len(results) {
		quickSort(results, 0, len(results)-1)
		return results
	}

	quickSelect(results, 0, len(results)-1, k)
	quickSort(results[:k], 0, k-1)
	return results[:k]
}

func quickSort(results []scoredVector, low, high int) {
	if low < high {
		pivot := partition(results, low, high)
		quickSort(results, low, pivot-1)
		quickSort(results, pivot+1, high)
	}
}

func partition(results []scoredVector, low, high int) int {
	pivot := results[high].score
	i := low - 1
	for j := low; j < high; j++ {
		if results[j].score >= pivot {
			i++
			results[i], results[j] = results[j], results[i]
		}
	}
	results[i+1], results[high] = results[high], results[i+1]
	return i + 1
}

func quickSelect(results []scoredVector, low, high, k int) {
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
