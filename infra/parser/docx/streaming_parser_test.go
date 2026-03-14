package docx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDocxStreamParser(t *testing.T) {
	p := NewDocxStreamParser()
	require.NotNil(t, p)
	assert.Equal(t, 500, p.chunkSize)
	assert.Equal(t, 50, p.chunkOverlap)
}

func TestDocxStreamParser_ParseStream(t *testing.T) {
	p := NewDocxStreamParser()

	// Test with empty reader
	reader := strings.NewReader("")
	docChan, err := p.ParseStream(context.Background(), reader, nil)
	require.NoError(t, err)

	docs := make([]*entity.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// For empty input, the parser returns an empty slice
	assert.Empty(t, docs)
}

func TestDocxStreamParser_ParseStream_WithContent(t *testing.T) {
	p := NewDocxStreamParser()
	p.chunkSize = 100 // Small chunk size for testing

	// Test with sample content
	sampleContent := "This is a sample DOCX content. It contains multiple lines of text to test the parsing functionality."
	reader := strings.NewReader(sampleContent)
	docChan, err := p.ParseStream(context.Background(), reader, nil)
	require.NoError(t, err)

	docs := make([]*entity.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
		assert.NotEmpty(t, doc.ID)
		assert.NotEmpty(t, doc.Metadata)
		assert.Equal(t, "docx", doc.Metadata["type"])
	}

	assert.NotEmpty(t, docs)
}

func TestDocxStreamParser_GetSupportedTypes(t *testing.T) {
	p := NewDocxStreamParser()
	formats := p.GetSupportedTypes()
	assert.Len(t, formats, 1)
	assert.Contains(t, formats, ".docx")
}

func TestDocxStreamParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	p := NewDocxStreamParser()
	ctx := context.Background()

	// Read all files in .data directory
	files, err := os.ReadDir(dataDir)
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
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "Failed to read test file: %s", filePath)

			// Create reader from file content
			reader := bytes.NewReader(content)

			// Parse the file
			docChan, err := p.ParseStream(ctx, reader, nil)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify documents
			docCount := 0
			for doc := range docChan {
				docCount++
				assert.NotEmpty(t, doc.ID, "Document should have an ID")
				assert.NotEmpty(t, doc.Metadata, "Document should have metadata")
				assert.Equal(t, "docx", doc.Metadata["type"], "Document should have type 'docx'")
			}
			assert.Greater(t, docCount, 0, "Should have at least one document")
		})
	}
}
