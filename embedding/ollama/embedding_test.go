package ollama

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_Embed(t *testing.T) {
	provider, err := New(Config{
		Model: "qllama/bge-small-zh-v1.5:latest",
		Dimension: 512, // bge-small-zh-v1.5 has 512 dimensions
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	texts := []string{
		"Hello, world!",
	}

	embeddings, err := provider.Embed(ctx, texts)
	require.NoError(t, err)
	assert.Len(t, embeddings, 1)
	assert.Len(t, embeddings[0], provider.Dimension())
}

func TestProvider_Dimension(t *testing.T) {
	// Test with custom dimension
	provider, err := New(Config{
		Model:     "qllama/bge-small-zh-v1.5:latest",
		Dimension: 512,
	})
	require.NoError(t, err)
	assert.Equal(t, 512, provider.Dimension())

	// Test with default dimension
	provider, err = New(Config{
		Model: "qllama/bge-small-zh-v1.5:latest",
	})
	require.NoError(t, err)
	assert.Equal(t, 1536, provider.Dimension())
}

func TestProvider_Embed_EmptyTexts(t *testing.T) {
	provider, err := New(Config{
		Model: "qllama/bge-small-zh-v1.5:latest",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	embeddings, err := provider.Embed(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, embeddings)
}
