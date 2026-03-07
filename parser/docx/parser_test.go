package docx

import (
	"context"
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

	// Test with empty reader
	reader := strings.NewReader("")
	chunks, err := p.Parse(context.Background(), reader)
	require.NoError(t, err)
	// For empty input, the parser returns an empty slice
	assert.Empty(t, chunks)

	// Test that chunks have expected structure
	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk.ID)
		assert.NotEmpty(t, chunk.Metadata)
		assert.Equal(t, "docx", chunk.Metadata["type"])
	}
}

func TestParser_SupportedFormats(t *testing.T) {
	p := NewParser()
	formats := p.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Contains(t, formats, ".docx")
}
