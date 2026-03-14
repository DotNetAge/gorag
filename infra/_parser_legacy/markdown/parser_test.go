package markdown

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Basic(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)

	content := `# Test Document

This is a test markdown document for streaming parser.

## Section 1

Some content here to make the document longer.

## Section 2

More content to test the streaming functionality.
`

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// Verify chunks have correct metadata
	for i, chunk := range chunks {
		if chunk.ID == "" {
			t.Errorf("Chunk %d has empty ID", i)
		}
		if chunk.Content == "" {
			t.Errorf("Chunk %d has empty content", i)
		}
		if chunk.Metadata["type"] != "markdown" {
			t.Errorf("Chunk %d has wrong type: %s", i, chunk.Metadata["type"])
		}
		if chunk.Metadata["streaming"] != "true" {
			t.Errorf("Chunk %d should be marked as streaming", i)
		}
	}
}

func TestParser_Frontmatter(t *testing.T) {
	parser := NewParser()
	parser.SetParseFrontmatter(true)

	content := `---
title: Test Document
author: Test Author
---

# Main Content

This is the main content.
`

	ctx := context.Background()
	var chunkCount int
	var firstChunkType string

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk model.Chunk) error {
		chunkCount++
		if chunkCount == 1 {
			firstChunkType = chunk.Metadata["type"]
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if chunkCount < 1 {
		t.Fatal("Expected at least one chunk")
	}

	if firstChunkType != "markdown_frontmatter" {
		t.Errorf("First chunk should be frontmatter, got: %s", firstChunkType)
	}
}

func TestParser_LargeFile(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(500)
	parser.SetChunkOverlap(50)

	// Create a large document (100KB)
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString("# Header\n")
		builder.WriteString("This is paragraph number ")
		builder.WriteString(string(rune(i)))
		builder.WriteString("\n\n")
	}
	content := builder.String()

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse large file: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks from large file")
	}

	t.Logf("Parsed %d chunks from large file", len(chunks))

	// Verify memory efficiency - chunks should be reasonable size
	for i, chunk := range chunks {
		if len(chunk.Content) > 2000 { // Should not exceed chunk size by much
			t.Errorf("Chunk %d too large: %d bytes", i, len(chunk.Content))
		}
	}
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50)

	// Create content that takes time to process
	var builder strings.Builder
	for i := 0; i < 100000; i++ {
		builder.WriteString("Line ")
		builder.WriteString(string(rune(i)))
		builder.WriteString("\n")
	}
	content := builder.String()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := parser.Parse(ctx, strings.NewReader(content))
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestParser_EmptyContent(t *testing.T) {
	parser := NewParser()

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(""))
	if err != nil {
		t.Fatalf("Failed to parse empty content: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got: %d", len(chunks))
	}
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()

	content := "# Test\n\nSome content here.\n"

	ctx := context.Background()
	expectedErr := "callback error"

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk model.Chunk) error {
		return &testError{msg: expectedErr}
	})

	if err == nil || !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected callback error, got: %v", err)
	}
}

func TestParser_ChunkSizing(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50)
	parser.SetChunkOverlap(10)

	content := strings.Repeat("A B C D E F G H I J K L M N O P Q R S T U V W X Y Z\n", 100)

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks")
	}

	// Verify chunk sizes respect configuration
	for i, chunk := range chunks {
		if len(chunk.Content) > 100 { // chunkSize + overlap tolerance
			t.Errorf("Chunk %d exceeds size limit: %d bytes", i, len(chunk.Content))
		}
	}
}

func TestParser_MetadataCompleteness(t *testing.T) {
	parser := NewParser()

	content := `---
title: Metadata Test
---

# Content

Test content.
`

	ctx := context.Background()
	var firstChunk *model.Chunk

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk model.Chunk) error {
		if firstChunk == nil {
			firstChunk = &chunk
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if firstChunk == nil {
		t.Fatal("Expected at least one chunk")
	}

	requiredFields := []string{"type", "position", "has_frontmatter", "streaming"}
	for _, field := range requiredFields {
		if _, ok := firstChunk.Metadata[field]; !ok {
			t.Errorf("Missing required metadata field: %s", field)
		}
	}
}

// Helper types
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
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
				assert.Contains(t, chunk.Metadata["type"], "markdown", "Chunk %d should have type 'markdown'", i)
			}
		})
	}
}
