package tscode

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestParser_New(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
	assert.Equal(t, 500, parser.chunkSize)
	assert.Equal(t, 50, parser.chunkOverlap)
	assert.True(t, parser.extractFunctions)
	assert.True(t, parser.extractClasses)
	assert.True(t, parser.extractInterfaces)
	assert.True(t, parser.extractComments)
}

func TestParser_GetSupportedTypes(t *testing.T) {
	parser := NewParser()
	supported := parser.GetSupportedTypes()
	assert.Contains(t, supported, ".ts")
	assert.Contains(t, supported, ".tsx")
}

func TestParser_ParseStream_Basic(t *testing.T) {
	parser := NewParser()

	tsCode := `function testFunction(): string {
	return "Hello World";
}

class TestClass {
	private value: number;

	constructor(value: number) {
		this.value = value;
	}
}

interface TestInterface {
	id: number;
	name: string;
}

type TestType = {
	id: number;
	name: string;
};`
	reader := strings.NewReader(tsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// Check that all elements were extracted
	functionFound := false
	classFound := false
	interfaceFound := false
	typeFound := false

	for _, doc := range docs {
		if strings.Contains(doc.Content, "testFunction") {
			functionFound = true
			assert.Equal(t, "function", doc.Metadata["chunk_type"])
			assert.Equal(t, "testFunction", doc.Metadata["function_name"])
		} else if strings.Contains(doc.Content, "TestClass") {
			classFound = true
			assert.Equal(t, "class", doc.Metadata["chunk_type"])
			assert.Equal(t, "TestClass", doc.Metadata["class_name"])
		} else if strings.Contains(doc.Content, "TestInterface") {
			interfaceFound = true
			assert.Equal(t, "interface", doc.Metadata["chunk_type"])
			assert.Equal(t, "TestInterface", doc.Metadata["interface_name"])
		} else if strings.Contains(doc.Content, "TestType") {
			typeFound = true
			assert.Equal(t, "type", doc.Metadata["chunk_type"])
			assert.Equal(t, "TestType", doc.Metadata["type_name"])
		}
	}

	assert.True(t, functionFound)
	assert.True(t, classFound)
	assert.True(t, interfaceFound)
	assert.True(t, typeFound)
}

func TestParser_ParseStream_WithComments(t *testing.T) {
	parser := NewParser()

	tsCode := `// This is a function comment
function testFunction(): string {
	/* This is a
	multi-line comment */
	return "Hello World";
}

// This is a class comment
class TestClass {
	constructor() {}
}`
	reader := strings.NewReader(tsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// Should have at least 2 documents (function and class)
	assert.GreaterOrEqual(t, len(docs), 2)
	
	// Check that function and class are present
	functionFound := false
	classFound := false
	commentFound := false

	for _, doc := range docs {
		if strings.Contains(doc.Content, "testFunction") {
			functionFound = true
			assert.Equal(t, "function", doc.Metadata["chunk_type"])
		} else if strings.Contains(doc.Content, "TestClass") {
			classFound = true
			assert.Equal(t, "class", doc.Metadata["chunk_type"])
		} else if strings.Contains(doc.Content, "This is a function comment") || strings.Contains(doc.Content, "This is a class comment") {
			commentFound = true
		}
	}

	assert.True(t, functionFound)
	assert.True(t, classFound)
	assert.True(t, commentFound)
}

func TestParser_ParseStream_EmptyFile(t *testing.T) {
	parser := NewParser()
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

func TestParser_ParseStream_Cancellation(t *testing.T) {
	parser := NewParser()

	tsCode := `function testFunction(): string {
	return "Hello World";
}

class TestClass {
	constructor() {}
}`
	reader := strings.NewReader(tsCode)

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

func TestParser_SetChunkSize(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(1000)
	assert.Equal(t, 1000, parser.chunkSize)
}

func TestParser_SetChunkOverlap(t *testing.T) {
	parser := NewParser()
	parser.SetChunkOverlap(100)
	assert.Equal(t, 100, parser.chunkOverlap)
}

func TestParser_SetExtractFunctions(t *testing.T) {
	parser := NewParser()
	parser.SetExtractFunctions(false)
	assert.False(t, parser.extractFunctions)
}

func TestParser_SetExtractClasses(t *testing.T) {
	parser := NewParser()
	parser.SetExtractClasses(false)
	assert.False(t, parser.extractClasses)
}

func TestParser_SetExtractInterfaces(t *testing.T) {
	parser := NewParser()
	parser.SetExtractInterfaces(false)
	assert.False(t, parser.extractInterfaces)
}

func TestParser_SetExtractComments(t *testing.T) {
	parser := NewParser()
	parser.SetExtractComments(false)
	assert.False(t, parser.extractComments)
}

func TestParser_ParseStream_WithoutFunctions(t *testing.T) {
	parser := NewParser()
	parser.SetExtractFunctions(false)

	tsCode := `function testFunction(): string {
	return "Hello World";
}

class TestClass {
	constructor() {}
}`
	reader := strings.NewReader(tsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// Check that class is extracted
	classFound := false

	for _, doc := range docs {
		if strings.Contains(doc.Content, "TestClass") {
			classFound = true
			assert.Equal(t, "class", doc.Metadata["chunk_type"])
		}
	}

	assert.True(t, classFound)
}

func TestParser_ParseStream_WithoutClasses(t *testing.T) {
	parser := NewParser()
	parser.SetExtractClasses(false)

	tsCode := `function testFunction(): string {
	return "Hello World";
}

class TestClass {
	constructor() {}
}`
	reader := strings.NewReader(tsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// Check that function is extracted
	functionFound := false

	for _, doc := range docs {
		if strings.Contains(doc.Content, "testFunction") {
			functionFound = true
			assert.Equal(t, "function", doc.Metadata["chunk_type"])
		}
	}

	assert.True(t, functionFound)
}

func TestParser_ParseStream_WithoutInterfaces(t *testing.T) {
	parser := NewParser()
	parser.SetExtractInterfaces(false)

	tsCode := `interface TestInterface {
	id: number;
}

type TestType = {
	id: number;
};`
	reader := strings.NewReader(tsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// No interfaces or types should be extracted
	assert.Len(t, docs, 1)
}

func TestParser_ParseStream_ArrowFunctions(t *testing.T) {
	parser := NewParser()

	tsCode := `const arrowFunction: () => string = () => {
	return "Arrow function";
};

const asyncArrow: () => Promise<void> = async () => {
	await new Promise(resolve => setTimeout(resolve, 100));
	console.log("Async arrow function");
};`
	reader := strings.NewReader(tsCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*core.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// Check that both arrow functions are extracted
	arrowFunctionFound := false
	asyncArrowFound := false

	for _, doc := range docs {
		if strings.Contains(doc.Content, "arrowFunction") {
			arrowFunctionFound = true
		} else if strings.Contains(doc.Content, "asyncArrow") {
			asyncArrowFound = true
		}
	}

	assert.True(t, arrowFunctionFound)
	assert.True(t, asyncArrowFound)
}
