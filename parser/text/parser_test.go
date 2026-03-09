package text

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

func TestParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	p := NewParser()
	ctx := context.Background()

	// Read all files in .data directory
	files, err := ioutil.ReadDir(dataDir)
	require.NoError(t, err, "Failed to read .data directory")
	require.NotEmpty(t, files, "No files found in .data directory")

	// Test each file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(dataDir, file.Name())
		t.Run(file.Name(), func(t *testing.T) {
			// Read file content
			content, err := ioutil.ReadFile(filePath)
			require.NoError(t, err, "Failed to read test file: %s", filePath)

			// Create reader from file content
			reader := bytes.NewReader(content)

			// Parse the file
			chunks, err := p.Parse(ctx, reader)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify chunks
			for i, chunk := range chunks {
				assert.NotEmpty(t, chunk.ID, "Chunk %d should have an ID", i)
				// Allow empty content for empty files
				// assert.NotEmpty(t, chunk.Content, "Chunk %d should have content", i)
				assert.Equal(t, "text", chunk.Metadata["type"], "Chunk %d should have type 'text'", i)
				assert.Contains(t, chunk.Metadata, "position", "Chunk %d should have position metadata", i)
			}
		})
	}
}
