package embedding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmbedder is a mock implementation of Provider for testing
type mockEmbedder struct {
	dimension int
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// Return empty embeddings for testing
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = make([]float32, m.dimension)
	}
	return embeddings, nil
}

func (m *mockEmbedder) Dimension() int {
	return m.dimension
}

func TestProviderInterface(t *testing.T) {
	// Test that mockEmbedder implements the Provider interface
	var provider Provider = &mockEmbedder{dimension: 1536}
	require.NotNil(t, provider)

	// Test Dimension method
	dimension := provider.Dimension()
	assert.Equal(t, 1536, dimension)

	// Test Embed method
	texts := []string{"Hello, world!", "Test embedding"}
	embeddings, err := provider.Embed(context.Background(), texts)
	require.NoError(t, err)
	assert.Len(t, embeddings, 2)
	assert.Len(t, embeddings[0], 1536)
	assert.Len(t, embeddings[1], 1536)
}

func TestProvider_EmptyTexts(t *testing.T) {
	provider := &mockEmbedder{dimension: 1536}

	// Test with empty texts
	embeddings, err := provider.Embed(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, embeddings)
}

func TestProvider_SingleText(t *testing.T) {
	provider := &mockEmbedder{dimension: 768}

	// Test with single text
	texts := []string{"Single text embedding"}
	embeddings, err := provider.Embed(context.Background(), texts)
	require.NoError(t, err)
	assert.Len(t, embeddings, 1)
	assert.Len(t, embeddings[0], 768)
}

func TestProvider_MultipleTexts(t *testing.T) {
	provider := &mockEmbedder{dimension: 1024}

	// Test with multiple texts
	texts := []string{"First text", "Second text", "Third text"}
	embeddings, err := provider.Embed(context.Background(), texts)
	require.NoError(t, err)
	assert.Len(t, embeddings, 3)
	for _, embedding := range embeddings {
		assert.Len(t, embedding, 1024)
	}
}
