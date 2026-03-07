package text

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	require.NotNil(t, p)
	assert.Equal(t, 500, p.chunkSize)
	assert.Equal(t, 50, p.chunkOverlap)
}

func TestParser_Parse(t *testing.T) {
	p := NewParser()
	ctx := context.Background()

	// Test with simple text
	text := "Hello, world! This is a test."
	reader := bytes.NewReader([]byte(text))

	chunks, err := p.Parse(ctx, reader)
	require.NoError(t, err)
	require.Len(t, chunks, 1)

	assert.NotEmpty(t, chunks[0].ID)
	assert.Equal(t, text, chunks[0].Content)
	assert.Equal(t, "text", chunks[0].Metadata["type"])
	assert.Equal(t, "0", chunks[0].Metadata["position"])
}

func TestParser_Parse_LargeText(t *testing.T) {
	p := NewParser()
	ctx := context.Background()

	// Create a large text that will be split into multiple chunks
	text := strings.Repeat("a", 1200)
	reader := bytes.NewReader([]byte(text))

	chunks, err := p.Parse(ctx, reader)
	require.NoError(t, err)
	// Should be split into 3 chunks (500 + 500 + 300 with overlap)
	assert.Len(t, chunks, 3)

	// Verify chunks have IDs and metadata
	for i, chunk := range chunks {
		assert.NotEmpty(t, chunk.ID)
		assert.NotEmpty(t, chunk.Content)
		assert.Equal(t, "text", chunk.Metadata["type"])
		assert.Equal(t, fmt.Sprintf("%d", i), chunk.Metadata["position"])
	}
}

func TestParser_Parse_EmptyText(t *testing.T) {
	p := NewParser()
	ctx := context.Background()

	// Test with empty text
	reader := bytes.NewReader([]byte(""))

	chunks, err := p.Parse(ctx, reader)
	require.NoError(t, err)
	// Should return one empty chunk
	assert.Len(t, chunks, 1)
	assert.NotEmpty(t, chunks[0].ID)
	assert.Empty(t, chunks[0].Content)
	assert.Equal(t, "text", chunks[0].Metadata["type"])
	assert.Equal(t, "0", chunks[0].Metadata["position"])
}

func TestParser_SupportedFormats(t *testing.T) {
	p := NewParser()
	formats := p.SupportedFormats()
	assert.Len(t, formats, 2)
	assert.Contains(t, formats, ".txt")
	assert.Contains(t, formats, ".md")
}


