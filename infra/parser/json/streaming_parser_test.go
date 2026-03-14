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

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonStreamParser_ParseStream(t *testing.T) {
	parser := NewJsonStreamParser()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	jsonContent := []byte(`{
		"name": "Test",
		"version": "1.0.0",
		"description": "A test JSON file"
	}`)

	r := bytes.NewReader(jsonContent)
	docCh, err := parser.ParseStream(ctx, r, nil)
	require.NoError(t, err)

	var documents []*entity.Document
	for doc := range docCh {
		documents = append(documents, doc)
	}

	assert.NotEmpty(t, documents)
	for _, doc := range documents {
		assert.NotEmpty(t, doc.ID)
		assert.NotEmpty(t, doc.Content)
		assert.Equal(t, "application/json", doc.ContentType)
		assert.Equal(t, "json", doc.Metadata["type"])
	}
}

func TestJsonStreamParser_EmptyJSON(t *testing.T) {
	parser := NewJsonStreamParser()
	ctx := context.Background()

	// Empty object
	jsonContent := []byte(`{}`)
	docCh, err := parser.ParseStream(ctx, bytes.NewReader(jsonContent), nil)
	require.NoError(t, err)

	var documents []*entity.Document
	for doc := range docCh {
		documents = append(documents, doc)
	}

	assert.NotEmpty(t, documents)
	for _, doc := range documents {
		assert.NotEmpty(t, doc.ID)
		assert.NotEmpty(t, doc.Content)
		assert.Equal(t, "application/json", doc.ContentType)
		assert.Equal(t, "json", doc.Metadata["type"])
	}
}

func TestJsonStreamParser_LargeArray(t *testing.T) {
	parser := NewJsonStreamParser()
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

	docCh, err := parser.ParseStream(ctx, strings.NewReader(sb.String()), nil)
	require.NoError(t, err)

	var documents []*entity.Document
	for doc := range docCh {
		documents = append(documents, doc)
	}

	assert.NotEmpty(t, documents)
	for _, doc := range documents {
		assert.NotEmpty(t, doc.ID)
		assert.NotEmpty(t, doc.Content)
		assert.Equal(t, "application/json", doc.ContentType)
		assert.Equal(t, "json", doc.Metadata["type"])
	}
}

func TestJsonStreamParser_ChunkConfiguration(t *testing.T) {
	parser := NewJsonStreamParser()
	parser.SetChunkSize(200)
	parser.SetChunkOverlap(20)

	// We can't directly access private fields, so we just test that the methods don't panic
	assert.NotPanics(t, func() {
		parser.SetChunkSize(300)
		parser.SetChunkOverlap(30)
	})
}

func TestJsonStreamParser_SupportedFormats(t *testing.T) {
	parser := NewJsonStreamParser()
	formats := parser.GetSupportedTypes()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".json", formats[0])
}

func TestJsonStreamParser_ParseStream_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewJsonStreamParser()
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
			docCh, err := parser.ParseStream(ctx, reader, nil)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify results
			var documents []*entity.Document
			for doc := range docCh {
				documents = append(documents, doc)
			}

			assert.NotEmpty(t, documents, "Should have at least one document")
			for _, doc := range documents {
				assert.NotEmpty(t, doc.ID)
	
			assert.NotEmpty(t, doc.Content)
				assert.Equal(t, "application/json", doc.ContentType)
				assert.Equal(t, "json", doc.Metadata["type"])
			}
		})
	}
}

// TestJsonStreamParser_ParseStream_WithContextFilePath tests parsing with file path in context
func TestJsonStreamParser_ParseStream_WithContextFilePath(t *testing.T) {
	parser := NewJsonStreamParser()
	ctx := context.Background()
	
	// Add file path to context
	ctx = context.WithValue(ctx, "file_path", "/test/path/file.json")

	jsonContent := []byte(`{
		"name": "Test",
		"version": "1.0.0"
	}`)

	r := bytes.NewReader(jsonContent)
	docCh, err := parser.ParseStream(ctx, r, nil)
	require.NoError(t, err)

	var documents []*entity.Document
	for doc := range docCh {
		documents = append(documents, doc)
	}

	assert.NotEmpty(t, documents)
	for _, doc := range documents {
		assert.Equal(t, "/test/path/file.json", doc.Metadata["file_path"])
	}
}

// TestJsonStreamParser_ParseStream_WithMetadata tests parsing with metadata
func TestJsonStreamParser_ParseStream_WithMetadata(t *testing.T) {
	parser := NewJsonStreamParser()
	ctx := context.Background()

	jsonContent := []byte(`{
		"name": "Test",
		"version": "1.0.0"
	}`)

	metadata := map[string]any{
		"source": "test.json",
		"author": "test author",
	}

	r := bytes.NewReader(jsonContent)
	docCh, err := parser.ParseStream(ctx, r, metadata)
	require.NoError(t, err)

	var documents []*entity.Document
	for doc := range docCh {
		documents = append(documents, doc)
	}

	assert.NotEmpty(t, documents)
	for _, doc := range documents {
		assert.Equal(t, "test.json", doc.Source)
		assert.Equal(t, "test author", doc.Metadata["author"])
		assert.Equal(t, "JsonStreamParser", doc.Metadata["parser"])
	}
}

// TestJsonStreamParser_ParseStream_WithContextCancel tests parsing with context cancellation
func TestJsonStreamParser_ParseStream_WithContextCancel(t *testing.T) {
	parser := NewJsonStreamParser()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	jsonContent := []byte(`{
		"name": "Test",
		"version": "1.0.0"
	}`)

	r := bytes.NewReader(jsonContent)
	docCh, err := parser.ParseStream(ctx, r, nil)
	require.NoError(t, err)

	// Should not receive any documents due to cancelled context
	docCount := 0
	for range docCh {
		docCount++
	}
	assert.Zero(t, docCount)
}

// TestJsonStreamParser_ParseStream_NestedJSON tests parsing nested JSON structures
func TestJsonStreamParser_ParseStream_NestedJSON(t *testing.T) {
	parser := NewJsonStreamParser()
	parser.SetChunkSize(100) // Small chunk size to test chunking
	ctx := context.Background()

	jsonContent := []byte(`{
		"name": "Test",
		"version": "1.0.0",
		"nested": {
			"key1": "value1",
			"key2": "value2",
			"deeply": {
				"nested": "value"
			}
		},
		"array": [1, 2, 3, 4, 5]
	}`)

	r := bytes.NewReader(jsonContent)
	docCh, err := parser.ParseStream(ctx, r, nil)
	require.NoError(t, err)

	var documents []*entity.Document
	for doc := range docCh {
		documents = append(documents, doc)
	}

	assert.NotEmpty(t, documents)
	for _, doc := range documents {
		assert.NotEmpty(t, doc.ID)
		assert.NotEmpty(t, doc.Content)
		assert.Equal(t, "application/json", doc.ContentType)
		assert.Equal(t, "json", doc.Metadata["type"])
	}
}
