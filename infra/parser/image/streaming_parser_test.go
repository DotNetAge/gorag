package image

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageStreamParser_BasicFunction(t *testing.T) {
	// Create a simple image content (PNG header)
	imageContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	parser := NewImageStreamParser()
	ctx := context.Background()
	metadata := make(map[string]any)

	docs, err := parser.ParseStream(ctx, strings.NewReader(string(imageContent)), metadata)
	require.NoError(t, err)

	var docCount int
	var firstDoc *entity.Document
	for doc := range docs {
		docCount++
		if firstDoc == nil {
			firstDoc = doc
		}
	}

	assert.GreaterOrEqual(t, docCount, 1)
	assert.Equal(t, "[Image content]", firstDoc.Content)
	assert.Equal(t, "image/png", firstDoc.ContentType)
	assert.NotNil(t, firstDoc.Metadata["media_data"])
	assert.Equal(t, "ImageStreamParser", firstDoc.Metadata["parser"])
}

func TestImageStreamParser_EmptyContent(t *testing.T) {
	parser := NewImageStreamParser()

	ctx := context.Background()
	metadata := make(map[string]any)
	docs, err := parser.ParseStream(ctx, strings.NewReader(""), metadata)
	if err != nil {
		t.Fatalf("Failed to parse empty content: %v", err)
	}

	var docCount int
	for range docs {
		docCount++
	}

	if docCount != 0 {
		t.Errorf("Expected 0 documents for empty content, got: %d", docCount)
	}
}

func TestImageStreamParser_GetSupportedTypes(t *testing.T) {
	parser := NewImageStreamParser()
	formats := parser.GetSupportedTypes()

	expectedFormats := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"}
	for _, expected := range expectedFormats {
		found := false
		for _, format := range formats {
			if format == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to support format '%s', but got: %v", expected, formats)
		}
	}
}

func TestImageStreamParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewImageStreamParser()
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
			metadata := make(map[string]any)
			docs, err := parser.ParseStream(ctx, reader, metadata)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify documents
			var docCount int
			for doc := range docs {
				docCount++
				assert.NotEmpty(t, doc.ID, "Document should have an ID")
				assert.NotEmpty(t, doc.Content, "Document should have content")
				assert.NotEmpty(t, doc.ContentType, "Document should have media type")
				assert.NotNil(t, doc.Metadata["media_data"], "Document should have media data")
				assert.Contains(t, doc.Metadata["parser"], "ImageStreamParser", "Document should have parser 'ImageStreamParser'")
			}

			assert.NotZero(t, docCount, "Expected at least one document")
		})
	}
}
