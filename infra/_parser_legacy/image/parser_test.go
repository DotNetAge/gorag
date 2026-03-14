package image

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	// Create a simple image content (PNG header)
	imageContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	parser := New()
	ctx := context.Background()

	chunks, err := parser.Parse(ctx, strings.NewReader(string(imageContent)))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(chunks), 1)
	assert.Equal(t, "[Image content]", chunks[0].Content)
	assert.Equal(t, "image/png", chunks[0].MediaType)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := New()
	formats := parser.SupportedFormats()
	assert.Contains(t, formats, ".jpg")
	assert.Contains(t, formats, ".jpeg")
	assert.Contains(t, formats, ".png")
	assert.Contains(t, formats, ".gif")
	assert.Contains(t, formats, ".webp")
}

func TestParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := New()
	ctx := context.Background()

	// Read all files in .data directory
	files, err := ioutil.ReadDir(dataDir)
	require.NoError(t, err, "Failed to read .data directory")
	if len(files) == 0 {
		t.Skip("No files found in .data directory")
	}

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
			chunks, err := parser.Parse(ctx, reader)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify chunks
			for i, chunk := range chunks {
				assert.NotEmpty(t, chunk.ID, "Chunk %d should have an ID", i)
				assert.NotEmpty(t, chunk.Content, "Chunk %d should have content", i)
				assert.NotEmpty(t, chunk.MediaType, "Chunk %d should have media type", i)
			}
		})
	}
}
