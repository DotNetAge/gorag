package pdf

import (
	"context"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestParser_New(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
}

func TestParser_ParseStream_Basic(t *testing.T) {
	parser := NewParser()

	pdfContent := "This is a PDF document\nwith multiple lines\nof content"
	reader := strings.NewReader(pdfContent)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "This is a PDF document")
	assert.Contains(t, docs[0].Content, "of content")
	assert.Equal(t, "pdf", docs[0].Metadata["type"])
	assert.Equal(t, "0", docs[0].Metadata["position"])
	assert.Equal(t, "pdf", docs[0].Metadata["parser"])
}

func TestParser_ParseStream_WithFilepathContext(t *testing.T) {
	parser := NewParser()

	pdfContent := "PDF content with filepath"
	reader := strings.NewReader(pdfContent)

	ctx := context.WithValue(context.Background(), filePathKey, "/path/to/test.pdf")
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Equal(t, "/path/to/test.pdf", docs[0].Metadata["file_path"])
}

func TestParser_ParseStream_EmptyFile(t *testing.T) {
	parser := NewParser()
	reader := strings.NewReader("")

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 0)
}

func TestParser_ParseStream_Cancellation(t *testing.T) {
	parser := NewParser()

	// Create a large content to ensure parser is working when we cancel
	largeContent := strings.Repeat("Line "+strings.Repeat("x", 100)+"\n", 100)
	reader := strings.NewReader(largeContent)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Cancel context immediately
	cancel()

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// Should have 0 or 1 documents due to cancellation
	assert.LessOrEqual(t, len(docs), 1)
}

func TestParser_GetSupportedTypes(t *testing.T) {
	parser := NewParser()
	supported := parser.GetSupportedTypes()
	assert.Contains(t, supported, ".pdf")
}
