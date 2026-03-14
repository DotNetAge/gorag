package yaml

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_ParseStream(t *testing.T) {
	parser := NewParser()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	yamlContent := []byte(`name: Test
version: "1.0.0"
description: A test YAML file`)

	r := bytes.NewReader(yamlContent)
	docCh, err := parser.ParseStream(ctx, r, nil)
	require.NoError(t, err)

	var docs []*entity.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
	assert.Contains(t, docs[0].Content, "name")
	assert.Contains(t, docs[0].Content, "Test")
	for _, doc := range docs {
		assert.NotEmpty(t, doc.ID)
		assert.Contains(t, doc.Metadata["type"], "yaml")
		assert.Contains(t, doc.Metadata["parser"], "yaml")
	}
}

func TestParser_EmptyYAML(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	// Empty YAML
	yamlContent := []byte(`{}`)
	docCh, err := parser.ParseStream(ctx, bytes.NewReader(yamlContent), nil)
	require.NoError(t, err)

	var docs []*entity.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
}

func TestParser_LargeYAML(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	// Create large YAML
	var sb strings.Builder
	sb.WriteString("items:\n")
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, "  - id: %d\n    name: item%d\n", i, i)
	}

	docCh, err := parser.ParseStream(ctx, strings.NewReader(sb.String()), nil)
	require.NoError(t, err)

	var docs []*entity.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	// Create large YAML
	var sb strings.Builder
	sb.WriteString("config:\n")
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&sb, "  key%d: value%d\n", i, i)
	}

	cancel() // Cancel immediately

	docCh, err := parser.ParseStream(ctx, strings.NewReader(sb.String()), nil)
	require.NoError(t, err)

	var docs []*entity.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	// May be empty due to cancellation
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(200)
	parser.SetChunkOverlap(20)

	assert.Equal(t, 200, parser.chunkSize)
	assert.Equal(t, 20, parser.chunkOverlap)
}

func TestParser_GetSupportedTypes(t *testing.T) {
	parser := NewParser()
	formats := parser.GetSupportedTypes()
	assert.Len(t, formats, 2)
	assert.Contains(t, formats, ".yaml")
	assert.Contains(t, formats, ".yml")
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
				assert.NotEmpty(t, doc.Content, "Document %d should have content", i)
				assert.Contains(t, doc.Metadata["type"], "yaml", "Document %d should have type 'yaml'", i)
				assert.Contains(t, doc.Metadata["parser"], "yaml", "Document %d should have parser 'yaml'", i)
			}
		})
	}
}