package semantic

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockEmbeddingProvider is a mock implementation of embedding.Provider
type MockEmbeddingProvider struct {
	dimension int
}

func (m *MockEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var embeddings [][]float32
	for _, text := range texts {
		// Return different embeddings for different sentences
		switch text {
		case "This is the first sentence.":
			embeddings = append(embeddings, []float32{0.1, 0.2, 0.3})
		case "This is the second sentence.":
			embeddings = append(embeddings, []float32{0.4, 0.5, 0.6})
		case "This is the third sentence.":
			embeddings = append(embeddings, []float32{0.7, 0.8, 0.9})
		default:
			embeddings = append(embeddings, []float32{0.1, 0.2, 0.3})
		}
	}
	return embeddings, nil
}

func (m *MockEmbeddingProvider) Dimension() int {
	if m.dimension > 0 {
		return m.dimension
	}
	// Return a default dimension
	return 3
}

func TestSemanticChunker_Chunk(t *testing.T) {
	// Create a mock embedding provider
	mockEmbedder := &MockEmbeddingProvider{
		dimension: 3,
	}

	// Create a semantic chunker
	chunker := NewSemanticChunker(mockEmbedder, 10, 100, 0.5)

	// Create a test document
	doc := &entity.Document{
		ID:      "test-doc",
		Content: "This is the first sentence. This is the second sentence. This is the third sentence.",
		Metadata: map[string]any{
			"author": "test",
		},
	}

	// Test Chunk method
	ctx := context.Background()
	chunks, err := chunker.Chunk(ctx, doc)

	// Check for errors
	assert.NoError(t, err)

	// Check that chunks were created
	assert.Greater(t, len(chunks), 0)

	// Check that metadata was inherited
	for _, chunk := range chunks {
		assert.Equal(t, "test", chunk.Metadata["author"])
	}
}

func TestSemanticChunker_HierarchicalChunk(t *testing.T) {
	// Create a mock embedding provider
	mockEmbedder := &MockEmbeddingProvider{
		dimension: 3,
	}

	// Create a semantic chunker
	chunker := NewSemanticChunker(mockEmbedder, 10, 100, 0.5)

	// Create a test document
	doc := &entity.Document{
		ID:      "test-doc",
		Content: "This is the first sentence. This is the second sentence. This is the third sentence.",
	}

	// Test HierarchicalChunk method
	ctx := context.Background()
	parents, children, err := chunker.HierarchicalChunk(ctx, doc)

	// Check for errors
	assert.NoError(t, err)

	// Check that parent and child chunks were created
	assert.Greater(t, len(parents), 0)
	assert.Greater(t, len(children), 0)

	// Check that parent and child relationships are set
	for _, child := range children {
		assert.NotEmpty(t, child.ParentID)
		assert.Equal(t, 2, child.Level)
	}

	for _, parent := range parents {
		assert.Equal(t, 1, parent.Level)
	}
}

func TestSemanticChunker_ContextualChunk(t *testing.T) {
	// Create a mock embedding provider
	mockEmbedder := &MockEmbeddingProvider{
		dimension: 3,
	}

	// Create a semantic chunker
	chunker := NewSemanticChunker(mockEmbedder, 10, 100, 0.5)

	// Create a test document
	doc := &entity.Document{
		ID:      "test-doc",
		Content: "This is the first sentence. This is the second sentence.",
	}

	// Test ContextualChunk method
	ctx := context.Background()
	docSummary := "This is a test document summary."
	chunks, err := chunker.ContextualChunk(ctx, doc, docSummary)

	// Check for errors
	assert.NoError(t, err)

	// Check that chunks were created
	assert.Greater(t, len(chunks), 0)

	// Check that context was injected
	for _, chunk := range chunks {
		assert.Contains(t, chunk.Content, "Document Context: This is a test document summary.")
		assert.Contains(t, chunk.Content, "Chunk Content:")
		assert.True(t, chunk.Metadata["is_contextual"].(bool))
	}
}

func TestSemanticChunker_splitIntoSentences(t *testing.T) {
	// Create a mock embedding provider
	mockEmbedder := &MockEmbeddingProvider{
		dimension: 3,
	}

	// Create a semantic chunker
	chunker := NewSemanticChunker(mockEmbedder, 10, 100, 0.5)

	// Test splitIntoSentences method with different punctuation
	text := "Hello world! How are you? I'm fine."
	sentences := chunker.splitIntoSentences(text)

	// Check that sentences were split correctly
	assert.Len(t, sentences, 3)
	assert.Equal(t, "Hello world!", sentences[0])
	assert.Equal(t, "How are you?", sentences[1])
	assert.Equal(t, "I'm fine.", sentences[2])
}

func TestCosineSimilarity(t *testing.T) {
	// Test cosine similarity with identical vectors
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0, 3.0}
	sim := cosineSimilarity(a, b)
	assert.InDelta(t, float32(1.0), sim, 0.000001)

	// Test cosine similarity with different vectors
	a = []float32{1.0, 0.0, 0.0}
	b = []float32{0.0, 1.0, 0.0}
	sim = cosineSimilarity(a, b)
	assert.InDelta(t, float32(0.0), sim, 0.000001)

	// Test cosine similarity with empty vectors
	a = []float32{}
	b = []float32{}
	sim = cosineSimilarity(a, b)
	assert.Equal(t, float32(0.0), sim)

	// Test cosine similarity with vectors of different lengths
	a = []float32{1.0, 2.0}
	b = []float32{1.0, 2.0, 3.0}
	sim = cosineSimilarity(a, b)
	assert.Equal(t, float32(0.0), sim)
}
