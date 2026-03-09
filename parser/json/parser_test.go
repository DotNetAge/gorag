package json

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	jsonContent := []byte(`{
		"name": "Test",
		"version": "1.0.0",
		"description": "A test JSON file"
	}`)

	r := bytes.NewReader(jsonContent)
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
	assert.Contains(t, chunks[0].Content, "name")
	assert.Contains(t, chunks[0].Content, "Test")
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	jsonContent := []byte(`{"key": "value", "nested": {"a": 1}}`)
	var chunkCount int

	err := parser.ParseWithCallback(ctx, bytes.NewReader(jsonContent), func(chunk core.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "json")
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
}

func TestParser_EmptyJSON(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	// Empty object
	jsonContent := []byte(`{}`)
	chunks, err := parser.Parse(ctx, bytes.NewReader(jsonContent))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_LargeArray(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	// Create large array
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id":%d,"name":"item%d"}`, i, i)
	}
	sb.WriteString("]")

	chunks, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	// Create large JSON
	var sb strings.Builder
	sb.WriteString("{")
	for i := 0; i < 1000; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `"key%d":"value%d"`, i, i)
	}
	sb.WriteString("}")

	cancel() // Cancel immediately

	_, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	jsonContent := []byte(`{"test": true}`)

	err := parser.ParseWithCallback(ctx, bytes.NewReader(jsonContent), func(chunk core.Chunk) error {
		return assert.AnError
	})

	assert.Error(t, err)
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(200)
	parser.SetChunkOverlap(20)

	assert.Equal(t, 200, parser.chunkSize)
	assert.Equal(t, 20, parser.chunkOverlap)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".json", formats[0])
}

func TestParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewParser()
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
		// Skip files that are not valid JSON
		if file.Name() == "tsconfig.json" {
			continue
		}
		// Skip non-JSON files
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if ext != ".json" {
			continue
		}

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
				assert.Contains(t, chunk.Metadata["type"], "json", "Chunk %d should have type 'json'", i)
			}
		})
	}
}
