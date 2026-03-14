package ppt

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

func TestParser_ParseStream(t *testing.T) {
	// Create a simple PPT file content
	pptContent := "Dummy PPT content"

	parser := NewParser()
	ctx := context.Background()

	docCh, err := parser.ParseStream(ctx, strings.NewReader(pptContent), nil)
	require.NoError(t, err)

	var docs []*entity.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	assert.GreaterOrEqual(t, len(docs), 1)
	for _, doc := range docs {
		assert.NotEmpty(t, doc.ID)
		assert.NotEmpty(t, doc.Metadata)
		assert.Equal(t, "ppt", doc.Metadata["type"])
		assert.Equal(t, "ppt", doc.Metadata["parser"])
	}
}

func TestParser_GetSupportedTypes(t *testing.T) {
	parser := NewParser()
	formats := parser.GetSupportedTypes()
	assert.Contains(t, formats, ".pptx")
	assert.Contains(t, formats, ".ppt")
}

func TestParser_ParseStream_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewParser()
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
			docCh, err := parser.ParseStream(ctx, reader, nil)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify documents
			var docs []*entity.Document
			for doc := range docCh {
				docs = append(docs, doc)
			}

			for i, doc := range docs {
				assert.NotEmpty(t, doc.ID, "Document %d should have an ID", i)
				assert.NotEmpty(t, doc.Metadata, "Document %d should have metadata", i)
				assert.Equal(t, "ppt", doc.Metadata["type"], "Document %d should have type 'ppt'", i)
				assert.Equal(t, "ppt", doc.Metadata["parser"], "Document %d should have parser 'ppt'", i)
			}
		})
	}
}