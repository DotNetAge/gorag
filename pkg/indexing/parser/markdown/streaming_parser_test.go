package markdown

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestMarkdownStreamParser_New(t *testing.T) {
	// Test with valid split level
	parser := DefaultMarkdownStreamParser(2)
	assert.NotNil(t, parser)
	assert.Equal(t, 2, parser.splitOnHeaderLevel)

	// Test with negative split level (should default to 1)
	parser = DefaultMarkdownStreamParser(-1)
	assert.NotNil(t, parser)
	assert.Equal(t, 1, parser.splitOnHeaderLevel)

	// Test with zero split level (should default to 1)
	parser = DefaultMarkdownStreamParser(0)
	assert.NotNil(t, parser)
	assert.Equal(t, 1, parser.splitOnHeaderLevel)

	// Test with split level greater than 6 (should default to 1)
	parser = DefaultMarkdownStreamParser(7)
	assert.NotNil(t, parser)
	assert.Equal(t, 1, parser.splitOnHeaderLevel)

	// Test with maximum valid split level
	parser = DefaultMarkdownStreamParser(6)
	assert.NotNil(t, parser)
	assert.Equal(t, 6, parser.splitOnHeaderLevel)
}

func TestMarkdownStreamParser_GetSupportedTypes(t *testing.T) {
	parser := DefaultMarkdownStreamParser(1)
	supportedTypes := parser.GetSupportedTypes()
	expectedTypes := []string{".md", ".markdown", "text/markdown"}
	assert.Equal(t, expectedTypes, supportedTypes)
}

func TestMarkdownStreamParser_ParseStream_SplitOnH1(t *testing.T) {
	// Create test markdown content with H1 headers
	markdownContent := `# Section 1
This is the content of section 1.

# Section 2
This is the content of section 2.
`
	reader := strings.NewReader(markdownContent)

	// Create parser with split level 1 (H1)
	parser := DefaultMarkdownStreamParser(1)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read first document
	doc1 := <-docChan
	assert.NotNil(t, doc1)
	assert.Contains(t, doc1.Content, "# Section 1")
	assert.Contains(t, doc1.Content, "This is the content of section 1.")
	assert.NotContains(t, doc1.Content, "# Section 2")
	assert.Equal(t, 0, doc1.Metadata["part_index"])

	// Read second document
	doc2 := <-docChan
	assert.NotNil(t, doc2)
	assert.Contains(t, doc2.Content, "# Section 2")
	assert.Contains(t, doc2.Content, "This is the content of section 2.")
	assert.Equal(t, 1, doc2.Metadata["part_index"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestMarkdownStreamParser_ParseStream_SplitOnH2(t *testing.T) {
	// Create test markdown content with H1 and H2 headers
	markdownContent := `# Section 1
This is the content of section 1.

## Subsection 1.1
This is the content of subsection 1.1.

## Subsection 1.2
This is the content of subsection 1.2.

# Section 2
This is the content of section 2.
`
	reader := strings.NewReader(markdownContent)

	// Create parser with split level 2 (H2)
	parser := DefaultMarkdownStreamParser(2)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read first document
	doc1 := <-docChan
	assert.NotNil(t, doc1)
	assert.Contains(t, doc1.Content, "# Section 1")
	assert.Contains(t, doc1.Content, "This is the content of section 1.")
	assert.Equal(t, 0, doc1.Metadata["part_index"])

	// Read second document
	doc2 := <-docChan
	assert.NotNil(t, doc2)
	assert.Contains(t, doc2.Content, "## Subsection 1.1")
	assert.Contains(t, doc2.Content, "This is the content of subsection 1.1.")
	assert.Equal(t, 1, doc2.Metadata["part_index"])

	// Read third document (contains the last subsection and section 2)
	doc3 := <-docChan
	assert.NotNil(t, doc3)
	assert.Contains(t, doc3.Content, "## Subsection 1.2")
	assert.Contains(t, doc3.Content, "This is the content of subsection 1.2.")
	assert.Contains(t, doc3.Content, "# Section 2")
	assert.Contains(t, doc3.Content, "This is the content of section 2.")
	assert.Equal(t, 2, doc3.Metadata["part_index"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestMarkdownStreamParser_ParseStream_NoHeaders(t *testing.T) {
	// Create test markdown content without headers
	markdownContent := `This is a markdown document without headers.
It has multiple lines of text.
`
	reader := strings.NewReader(markdownContent)

	// Create parser with split level 1
	parser := DefaultMarkdownStreamParser(1)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read the single document
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.Contains(t, doc.Content, "This is a markdown document without headers.")
	assert.Contains(t, doc.Content, "It has multiple lines of text.")
	assert.Equal(t, 0, doc.Metadata["part_index"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestMarkdownStreamParser_ParseStream_EmptyFile(t *testing.T) {
	// Create empty markdown content
	markdownContent := ""
	reader := strings.NewReader(markdownContent)

	// Create parser
	parser := DefaultMarkdownStreamParser(1)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// The channel should be closed without any documents
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestMarkdownStreamParser_ParseStream_WithMetadata(t *testing.T) {
	// Create test markdown content
	markdownContent := `# Section 1
This is the content of section 1.
`
	reader := strings.NewReader(markdownContent)

	// Create metadata
	metadata := map[string]any{
		"source": "test.md",
		"author": "test",
	}

	// Create parser
	parser := DefaultMarkdownStreamParser(1)

	// Test ParseStream
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, metadata)
	assert.NoError(t, err)

	// Read from the channel
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.Equal(t, "test.md", doc.Source)

	// Check metadata
	assert.Equal(t, "MarkdownStreamParser", doc.Metadata["parser"])
	assert.Equal(t, "test.md", doc.Metadata["source"])
	assert.Equal(t, "test", doc.Metadata["author"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)

	// Explicitly use entity package to avoid import error
	_ = core.Document{}
}

func TestMarkdownStreamParser_ParseStream_ContextCanceled(t *testing.T) {
	// Create test markdown content with multiple sections
	markdownContent := `# Section 1
This is the content of section 1.

# Section 2
This is the content of section 2.
`
	reader := strings.NewReader(markdownContent)

	// Create parser
	parser := DefaultMarkdownStreamParser(1)

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
