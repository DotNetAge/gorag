package csv

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCSVStreamParser_New(t *testing.T) {
	// Test with custom rows per document
	parser := NewCSVStreamParser(50, true)
	assert.NotNil(t, parser)
	assert.Equal(t, 50, parser.rowsPerDocument)
	assert.True(t, parser.hasHeader)

	// Test with default rows per document (negative value)
	parser = NewCSVStreamParser(-1, false)
	assert.NotNil(t, parser)
	assert.Equal(t, 100, parser.rowsPerDocument) // Default value
	assert.False(t, parser.hasHeader)

	// Test with default rows per document (zero value)
	parser = NewCSVStreamParser(0, true)
	assert.NotNil(t, parser)
	assert.Equal(t, 100, parser.rowsPerDocument) // Default value
	assert.True(t, parser.hasHeader)
}

func TestCSVStreamParser_GetSupportedTypes(t *testing.T) {
	parser := NewCSVStreamParser(100, true)
	supportedTypes := parser.GetSupportedTypes()
	assert.Equal(t, []string{".csv", "text/csv"}, supportedTypes)
}

func TestCSVStreamParser_ParseStream_WithHeader(t *testing.T) {
	// Create test CSV content with header
	csvContent := "name,age,city\nJohn,30,New York\nJane,25,London\n"
	reader := strings.NewReader(csvContent)

	// Create parser with header
	parser := NewCSVStreamParser(100, true)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read from the channel
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.NotEmpty(t, doc.ID)
	assert.Contains(t, doc.Content, "name: John; age: 30; city: New York;")
	assert.Contains(t, doc.Content, "name: Jane; age: 25; city: London;")
	assert.Equal(t, "unknown", doc.Source)
	assert.Equal(t, "text/csv", doc.ContentType)

	// Check metadata
	assert.Equal(t, "CSVStreamParser", doc.Metadata["parser"])
	assert.Equal(t, 0, doc.Metadata["part_index"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)

	// Explicitly use entity package to avoid import error
	_ = core.Document{}
}

func TestCSVStreamParser_ParseStream_WithoutHeader(t *testing.T) {
	// Create test CSV content without header
	csvContent := "John,30,New York\nJane,25,London\n"
	reader := strings.NewReader(csvContent)

	// Create parser without header
	parser := NewCSVStreamParser(100, false)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read from the channel
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.NotEmpty(t, doc.ID)
	assert.Contains(t, doc.Content, "John, 30, New York")
	assert.Contains(t, doc.Content, "Jane, 25, London")
	assert.Equal(t, "unknown", doc.Source)
	assert.Equal(t, "text/csv", doc.ContentType)

	// Check metadata
	assert.Equal(t, "CSVStreamParser", doc.Metadata["parser"])
	assert.Equal(t, 0, doc.Metadata["part_index"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestCSVStreamParser_ParseStream_MultipleDocuments(t *testing.T) {
	// Create test CSV content with multiple rows
	var csvContent strings.Builder
	csvContent.WriteString("name,age\n")
	for i := 1; i <= 5; i++ {
		csvContent.WriteString(fmt.Sprintf("Person%d,%d\n", i, 20+i))
	}
	reader := strings.NewReader(csvContent.String())

	// Create parser with small rows per document
	parser := NewCSVStreamParser(2, true)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read first document
	doc1 := <-docChan
	assert.NotNil(t, doc1)
	assert.Equal(t, 0, doc1.Metadata["part_index"])
	assert.Contains(t, doc1.Content, "Person1")
	assert.Contains(t, doc1.Content, "Person2")

	// Read second document
	doc2 := <-docChan
	assert.NotNil(t, doc2)
	assert.Equal(t, 1, doc2.Metadata["part_index"])
	assert.Contains(t, doc2.Content, "Person3")
	assert.Contains(t, doc2.Content, "Person4")

	// Read third document (remaining row)
	doc3 := <-docChan
	assert.NotNil(t, doc3)
	assert.Equal(t, 2, doc3.Metadata["part_index"])
	assert.Contains(t, doc3.Content, "Person5")

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestCSVStreamParser_ParseStream_EmptyFile(t *testing.T) {
	// Create empty CSV content
	csvContent := ""
	reader := strings.NewReader(csvContent)

	// Create parser
	parser := NewCSVStreamParser(100, true)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// The channel should be closed without any documents
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestCSVStreamParser_ParseStream_WithMetadata(t *testing.T) {
	// Create test CSV content
	csvContent := "name,age\nJohn,30\n"
	reader := strings.NewReader(csvContent)

	// Create metadata
	metadata := map[string]any{
		"source": "test.csv",
		"author": "test",
	}

	// Create parser
	parser := NewCSVStreamParser(100, true)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, metadata)
	assert.NoError(t, err)

	// Read from the channel
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.Equal(t, "test.csv", doc.Source)

	// Check metadata
	assert.Equal(t, "CSVStreamParser", doc.Metadata["parser"])
	assert.Equal(t, "test.csv", doc.Metadata["source"])
	assert.Equal(t, "test", doc.Metadata["author"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestCSVStreamParser_ParseStream_ContextCanceled(t *testing.T) {
	// Create test CSV content with multiple rows
	csvContent := "name,age\nJohn,30\nJane,25\n"
	reader := strings.NewReader(csvContent)

	// Create parser
	parser := NewCSVStreamParser(100, true)

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
