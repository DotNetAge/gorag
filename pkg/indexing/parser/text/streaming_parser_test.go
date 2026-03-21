package text

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestTextStreamParser_New(t *testing.T) {
	// Test with custom max read bytes
	parser := DefaultTextStreamParser(1024)
	assert.NotNil(t, parser)
	assert.Equal(t, 1024, parser.maxReadBytes)

	// Test with negative max read bytes (should default to 10MB)
	parser = DefaultTextStreamParser(-1)
	assert.NotNil(t, parser)
	assert.Equal(t, 10*1024*1024, parser.maxReadBytes) // Default value

	// Test with zero max read bytes (should default to 10MB)
	parser = DefaultTextStreamParser(0)
	assert.NotNil(t, parser)
	assert.Equal(t, 10*1024*1024, parser.maxReadBytes) // Default value
}

func TestTextStreamParser_GetSupportedTypes(t *testing.T) {
	parser := DefaultTextStreamParser(1024)
	supportedTypes := parser.GetSupportedTypes()
	expectedTypes := []string{".txt", ".md", ".csv", ".log", "text/plain", "text/markdown"}
	assert.Equal(t, expectedTypes, supportedTypes)
}

func TestTextStreamParser_ParseStream_SmallContent(t *testing.T) {
	// Create test text content
	textContent := "This is a small text file with just a few lines.\nIt should be processed as a single document.\n"
	reader := strings.NewReader(textContent)

	// Create parser with large max read bytes
	parser := DefaultTextStreamParser(1024)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read the single document
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.Contains(t, doc.Content, "This is a small text file with just a few lines.")
	assert.Contains(t, doc.Content, "It should be processed as a single document.")
	assert.Equal(t, "unknown", doc.Source)
	assert.Equal(t, "text/plain", doc.ContentType)

	// Check metadata
	assert.Equal(t, "TextStreamParser", doc.Metadata["parser"])
	assert.Equal(t, 0, doc.Metadata["part_index"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)

	// Explicitly use entity package to avoid import error
	_ = core.Document{}
}

func TestTextStreamParser_ParseStream_MultipleParts(t *testing.T) {
	// Create test text content that will be split into multiple parts
	var textContent strings.Builder
	for i := 1; i <= 10; i++ {
		textContent.WriteString(strings.Repeat("x", 200) + "\n") // 201 bytes per line
	}
	// Total content is ~2010 bytes
	reader := strings.NewReader(textContent.String())

	// Create parser with small max read bytes
	parser := DefaultTextStreamParser(1000)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read first document part
	doc1 := <-docChan
	assert.NotNil(t, doc1)

	// Read second document part (remaining content)
	doc2 := <-docChan
	assert.NotNil(t, doc2)

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestTextStreamParser_ParseStream_EmptyFile(t *testing.T) {
	// Create empty text content
	textContent := ""
	reader := strings.NewReader(textContent)

	// Create parser
	parser := DefaultTextStreamParser(1024)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// The channel should be closed without any documents
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestTextStreamParser_ParseStream_WithMetadata(t *testing.T) {
	// Create test text content
	textContent := "This is a test text file.\n"
	reader := strings.NewReader(textContent)

	// Create metadata
	metadata := map[string]any{
		"source": "test.txt",
		"author": "test",
	}

	// Create parser
	parser := DefaultTextStreamParser(1024)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, metadata)
	assert.NoError(t, err)

	// Read from the channel
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.Equal(t, "test.txt", doc.Source)

	// Check metadata
	assert.Equal(t, "TextStreamParser", doc.Metadata["parser"])
	assert.Equal(t, "test.txt", doc.Metadata["source"])
	assert.Equal(t, "test", doc.Metadata["author"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestTextStreamParser_ParseStream_ContextCanceled(t *testing.T) {
	// Create test text content with multiple lines
	textContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"
	reader := strings.NewReader(textContent)

	// Create parser
	parser := DefaultTextStreamParser(100)

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())

	// Test ParseStream
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Cancel the context
	cancel()

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}
