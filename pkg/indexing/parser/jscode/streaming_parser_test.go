package jscode

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestJscodeStreamParser_New(t *testing.T) {
	parser := NewJscodeStreamParser()
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.legacyParser)
}

func TestJscodeStreamParser_GetSupportedTypes(t *testing.T) {
	parser := NewJscodeStreamParser()
	supported := parser.GetSupportedTypes()
	assert.Contains(t, supported, ".js")
	assert.Contains(t, supported, ".jsx")
	assert.Contains(t, supported, ".mjs")
}

func TestJscodeStreamParser_ParseStream_Basic(t *testing.T) {
	parser := NewJscodeStreamParser()

	jsCode := `function testFunction() {
	console.log("Hello World");
}

class TestClass {
	constructor() {
		this.value = 42;
	}
}`
	reader := strings.NewReader(jsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "testFunction")
	assert.Contains(t, docs[0].Content, "TestClass")
	assert.Equal(t, "jscode", docs[0].Metadata["type"])
	assert.Equal(t, "JscodeStreamParser", docs[0].Metadata["parser"])
}

func TestJscodeStreamParser_ParseStream_WithSourceMetadata(t *testing.T) {
	parser := NewJscodeStreamParser()

	jsCode := `function testFunction() {
	console.log("Hello World");
}`
	reader := strings.NewReader(jsCode)

	metadata := map[string]any{"source": "test.js"}
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, metadata)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Equal(t, "test.js", docs[0].Source)
}

func TestJscodeStreamParser_ParseStream_EmptyFile(t *testing.T) {
	parser := NewJscodeStreamParser()
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

func TestJscodeStreamParser_ParseStream_Cancellation(t *testing.T) {
	parser := NewJscodeStreamParser()

	jsCode := `function testFunction() {
	console.log("Hello World");
}

class TestClass {
	constructor() {
		this.value = 42;
	}
}`
	reader := strings.NewReader(jsCode)

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

func TestJscodeStreamParser_SetChunkSize(t *testing.T) {
	parser := NewJscodeStreamParser()
	parser.SetChunkSize(1000)
	assert.Equal(t, 1000, parser.legacyParser.chunkSize)
}

func TestJscodeStreamParser_SetChunkOverlap(t *testing.T) {
	parser := NewJscodeStreamParser()
	parser.SetChunkOverlap(100)
	assert.Equal(t, 100, parser.legacyParser.chunkOverlap)
}

func TestJscodeStreamParser_SetExtractFunctions(t *testing.T) {
	parser := NewJscodeStreamParser()
	parser.SetExtractFunctions(false)
	assert.False(t, parser.legacyParser.extractFunctions)
}

func TestJscodeStreamParser_SetExtractClasses(t *testing.T) {
	parser := NewJscodeStreamParser()
	parser.SetExtractClasses(false)
	assert.False(t, parser.legacyParser.extractClasses)
}

func TestJscodeStreamParser_SetExtractComments(t *testing.T) {
	parser := NewJscodeStreamParser()
	parser.SetExtractComments(false)
	assert.False(t, parser.legacyParser.extractComments)
}

func TestJscodeStreamParser_ParseStream_WithComments(t *testing.T) {
	parser := NewJscodeStreamParser()

	jsCode := `// This is a function comment
function testFunction() {
	/* This is a
	multi-line comment */
	console.log("Hello World");
}

// This is a class comment
class TestClass {
	constructor() {
		this.value = 42;
	}
}`
	reader := strings.NewReader(jsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "testFunction")
	assert.Contains(t, docs[0].Content, "TestClass")
	assert.Contains(t, docs[0].Content, "Hello World")
	// Comments are extracted separately and appended
	assert.Contains(t, docs[0].Content, "This is a function comment")
}

func TestJscodeStreamParser_ParseStream_WithoutFunctions(t *testing.T) {
	parser := NewJscodeStreamParser()
	parser.SetExtractFunctions(false)

	jsCode := `function testFunction() {
	console.log("Hello World");
}

class TestClass {
	constructor() {
		this.value = 42;
	}
}`
	reader := strings.NewReader(jsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	// Functions should not be extracted separately
}

func TestJscodeStreamParser_ParseStream_WithoutClasses(t *testing.T) {
	parser := NewJscodeStreamParser()
	parser.SetExtractClasses(false)

	jsCode := `function testFunction() {
	console.log("Hello World");
}

class TestClass {
	constructor() {
		this.value = 42;
	}
}`
	reader := strings.NewReader(jsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	// Classes should not be extracted separately
}

func TestJscodeStreamParser_ParseStream_ArrowFunctions(t *testing.T) {
	parser := NewJscodeStreamParser()

	jsCode := `const arrowFunction = () => {
	console.log("Arrow function");
};

const asyncArrow = async () => {
	await new Promise(resolve => setTimeout(resolve, 100));
	console.log("Async arrow function");
};`
	reader := strings.NewReader(jsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "arrowFunction")
	assert.Contains(t, docs[0].Content, "asyncArrow")
}
