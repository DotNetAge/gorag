package retrieval

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockKeywordStore struct {
	results []vectorstore.Result
}

func (m *mockKeywordStore) Search(ctx context.Context, query string, topK int) ([]vectorstore.Result, error) {
	return m.results, nil
}

type mockVectorStore struct {
	results []vectorstore.Result
}

func (m *mockVectorStore) Add(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
	return m.results, nil
}

func (m *mockVectorStore) Delete(ctx context.Context, ids []string) error {
	return nil
}

func TestHybridRetriever_Search(t *testing.T) {
	vectorResults := []vectorstore.Result{
		{
			Chunk: vectorstore.Chunk{
				ID:      "vec1",
				Content: "Vector result 1",
			},
			Score: 0.9,
		},
		{
			Chunk: vectorstore.Chunk{
				ID:      "vec2",
				Content: "Vector result 2",
			},
			Score: 0.8,
		},
	}

	keywordResults := []vectorstore.Result{
		{
			Chunk: vectorstore.Chunk{
				ID:      "key1",
				Content: "Keyword result 1",
			},
			Score: 0.7,
		},
		{
			Chunk: vectorstore.Chunk{
				ID:      "vec1", // Same as vector result
				Content: "Vector result 1",
			},
			Score: 0.6,
		},
	}

	vectorStore := &mockVectorStore{results: vectorResults}
	keywordStore := &mockKeywordStore{results: keywordResults}

	tests := []struct {
		name     string
		alpha    float32
		topK     int
		expected int
	}{
		{
			name:     "balanced weights",
			alpha:    0.5,
			topK:     3,
			expected: 3,
		},
		{
			name:     "vector heavy",
			alpha:    0.2,
			topK:     2,
			expected: 2,
		},
		{
			name:     "keyword heavy",
			alpha:    0.8,
			topK:     2,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retriever := NewHybridRetriever(vectorStore, keywordStore, tt.alpha)
			results, err := retriever.Search(context.Background(), "test query", []float32{0.1, 0.2, 0.3}, tt.topK)
			require.NoError(t, err)
			assert.Len(t, results, tt.expected)
			assert.True(t, results[0].Score > 0)
		})
	}
}

func TestHybridRetriever_CombineResults(t *testing.T) {
	vectorResults := []vectorstore.Result{
		{
			Chunk: vectorstore.Chunk{
				ID:      "doc1",
				Content: "Document 1",
			},
			Score: 0.9,
		},
	}

	keywordResults := []vectorstore.Result{
		{
			Chunk: vectorstore.Chunk{
				ID:      "doc1",
				Content: "Document 1",
			},
			Score: 0.7,
		},
		{
			Chunk: vectorstore.Chunk{
				ID:      "doc2",
				Content: "Document 2",
			},
			Score: 0.8,
		},
	}

	vectorStore := &mockVectorStore{results: vectorResults}
	keywordStore := &mockKeywordStore{results: keywordResults}

	retriever := NewHybridRetriever(vectorStore, keywordStore, 0.5)
	results := retriever.combineResults(vectorResults, keywordResults, 2)

	assert.Len(t, results, 2)
	assert.Equal(t, "doc1", results[0].ID) // Should have higher combined score
	assert.Equal(t, "doc2", results[1].ID)
}
