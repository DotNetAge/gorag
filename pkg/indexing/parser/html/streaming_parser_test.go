package html

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestHtmlStreamParser_New(t *testing.T) {
	parser := NewHtmlStreamParser()
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.legacyParser)
}

func TestHtmlStreamParser_GetSupportedTypes(t *testing.T) {
	parser := NewHtmlStreamParser()
	supported := parser.GetSupportedTypes()
	assert.Contains(t, supported, ".html")
	assert.Contains(t, supported, ".htm")
}

func TestHtmlStreamParser_ParseStream_Basic(t *testing.T) {
	parser := NewHtmlStreamParser()

	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
</head>
<body>
	<h1>Hello World</h1>
	<p>This is a test paragraph.</p>
</body>
</html>`
	reader := strings.NewReader(htmlContent)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "Test Page")
	assert.Contains(t, docs[0].Content, "Hello World")
	assert.Contains(t, docs[0].Content, "This is a test paragraph")
	assert.Equal(t, "html", docs[0].Metadata["type"])
	assert.Equal(t, "HtmlStreamParser", docs[0].Metadata["parser"])
}

func TestHtmlStreamParser_ParseStream_WithSourceMetadata(t *testing.T) {
	parser := NewHtmlStreamParser()

	htmlContent := `<html><body><h1>Test</h1></body></html>`
	reader := strings.NewReader(htmlContent)

	metadata := map[string]any{"source": "test.html"}
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, metadata)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Equal(t, "test.html", docs[0].Source)
}

func TestHtmlStreamParser_ParseStream_EmptyFile(t *testing.T) {
	parser := NewHtmlStreamParser()
	reader := strings.NewReader("")

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 0)
}

func TestHtmlStreamParser_ParseStream_Cancellation(t *testing.T) {
	parser := NewHtmlStreamParser()

	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
</head>
<body>
	<h1>Hello World</h1>
	<p>This is a test paragraph.</p>
</body>
</html>`
	reader := strings.NewReader(htmlContent)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Cancel context immediately
	cancel()

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// Should have 0 or 1 documents due to cancellation
	assert.LessOrEqual(t, len(docs), 1)
}

func TestHtmlStreamParser_SetChunkSize(t *testing.T) {
	parser := NewHtmlStreamParser()
	parser.SetChunkSize(1000)
	assert.Equal(t, 1000, parser.legacyParser.chunkSize)
}

func TestHtmlStreamParser_SetChunkOverlap(t *testing.T) {
	parser := NewHtmlStreamParser()
	parser.SetChunkOverlap(100)
	assert.Equal(t, 100, parser.legacyParser.chunkOverlap)
}

func TestHtmlStreamParser_SetCleanScripts(t *testing.T) {
	parser := NewHtmlStreamParser()
	parser.SetCleanScripts(false)
	assert.False(t, parser.legacyParser.cleanScripts)
}

func TestHtmlStreamParser_SetCleanStyles(t *testing.T) {
	parser := NewHtmlStreamParser()
	parser.SetCleanStyles(false)
	assert.False(t, parser.legacyParser.cleanStyles)
}

func TestHtmlStreamParser_SetExtractLinks(t *testing.T) {
	parser := NewHtmlStreamParser()
	parser.SetExtractLinks(true)
	assert.True(t, parser.legacyParser.extractLinks)
}

func TestHtmlStreamParser_ParseStream_WithScriptsAndStyles(t *testing.T) {
	parser := NewHtmlStreamParser()
	// Allow scripts and styles for this test
	parser.SetCleanScripts(false)
	parser.SetCleanStyles(false)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
	<style>
		body { background-color: red; }
	</style>
	<script>
		console.log("Hello from script");
	</script>
</head>
<body>
	<h1>Hello World</h1>
</body>
</html>`
	reader := strings.NewReader(htmlContent)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "Hello World")
	assert.Contains(t, docs[0].Content, "background-color: red")
	assert.Contains(t, docs[0].Content, "console.log(\"Hello from script\")")
}

func TestHtmlStreamParser_ParseStream_WithoutScriptsAndStyles(t *testing.T) {
	parser := NewHtmlStreamParser()
	// Default behavior: remove scripts and styles

	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
	<style>
		body { background-color: red; }
	</style>
	<script>
		console.log("Hello from script");
	</script>
</head>
<body>
	<h1>Hello World</h1>
</body>
</html>`
	reader := strings.NewReader(htmlContent)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "Hello World")
	// Scripts and styles should be removed
	assert.NotContains(t, docs[0].Content, "background-color: red")
	assert.NotContains(t, docs[0].Content, "console.log(\"Hello from script\")")
}
