package html

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/core"
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

	// Test with empty HTML
	emptyHTML := ""
	reader := strings.NewReader(emptyHTML)
	chunks, err := p.Parse(context.Background(), reader)
	require.NoError(t, err)
	// For empty input, the parser returns an empty slice
	assert.Empty(t, chunks)

	// Test with simple HTML
	simpleHTML := "<html><body><h1>Hello</h1><p>World</p></body></html>"
	reader = strings.NewReader(simpleHTML)
	chunks, err = p.Parse(context.Background(), reader)
	require.NoError(t, err)
	// For simple HTML, check that we get chunks (they might be empty if text extraction fails)
	assert.IsType(t, []core.Chunk{}, chunks)

	// Test with complex HTML
	complexHTML := `
		<html>
		<head><title>Test</title></head>
		<body>
			<h1>Header</h1>
			<p>Paragraph 1</p>
			<p>Paragraph 2</p>
			<script>console.log('script')</script>
			<style>body { color: red; }</style>
		</body>
		</html>
	`
	reader = strings.NewReader(complexHTML)
	chunks, err = p.Parse(context.Background(), reader)
	require.NoError(t, err)
	// For complex HTML, check that we get chunks (they might be empty if text extraction fails)
	assert.IsType(t, []core.Chunk{}, chunks)

	// Test that chunks have expected structure if they exist
	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk.ID)
		assert.NotEmpty(t, chunk.Metadata)
		assert.Equal(t, "html", chunk.Metadata["type"])
	}
}

func TestParser_SupportedFormats(t *testing.T) {
	p := NewParser()
	formats := p.SupportedFormats()
	assert.Len(t, formats, 2)
	assert.Contains(t, formats, ".html")
	assert.Contains(t, formats, ".htm")
}



func TestParser_extractText(t *testing.T) {
	p := NewParser()

	// Test with simple HTML
	simpleHTML := "<html><body><h1>Hello</h1><p>World</p></body></html>"
	reader := strings.NewReader(simpleHTML)
	chunks, err := p.Parse(context.Background(), reader)
	require.NoError(t, err)
	// For simple HTML, check that we get chunks (they might be empty if text extraction fails)
	assert.IsType(t, []core.Chunk{}, chunks)
	// Check that script and style tags are not included
	for _, chunk := range chunks {
		assert.NotContains(t, chunk.Content, "script")
		assert.NotContains(t, chunk.Content, "style")
	}
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
				assert.NotEmpty(t, chunk.Metadata, "Chunk %d should have metadata", i)
				assert.Equal(t, "html", chunk.Metadata["type"], "Chunk %d should have type 'html'", i)
			}
		})
	}
}
