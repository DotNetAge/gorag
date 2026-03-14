package javacode

import (
	"context"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestJavacodeStreamParser_New(t *testing.T) {
	parser := NewJavacodeStreamParser()
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.legacyParser)
}

func TestJavacodeStreamParser_GetSupportedTypes(t *testing.T) {
	parser := NewJavacodeStreamParser()
	supported := parser.GetSupportedTypes()
	assert.Contains(t, supported, ".java")
}

func TestJavacodeStreamParser_ParseStream_Basic(t *testing.T) {
	parser := NewJavacodeStreamParser()

	javaCode := `public class TestClass {
	public void testMethod() {
		System.out.println("Hello World");
	}
}`
	reader := strings.NewReader(javaCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "TestClass")
	assert.Contains(t, docs[0].Content, "testMethod")
	assert.Equal(t, "javacode", docs[0].Metadata["type"])
	assert.Equal(t, "JavacodeStreamParser", docs[0].Metadata["parser"])
}

func TestJavacodeStreamParser_ParseStream_WithSourceMetadata(t *testing.T) {
	parser := NewJavacodeStreamParser()

	javaCode := `public class TestClass {
	public void testMethod() {
		System.out.println("Hello World");
	}
}`
	reader := strings.NewReader(javaCode)

	metadata := map[string]any{"source": "Test.java"}
	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, metadata)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Equal(t, "Test.java", docs[0].Source)
}

func TestJavacodeStreamParser_ParseStream_EmptyFile(t *testing.T) {
	parser := NewJavacodeStreamParser()
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

func TestJavacodeStreamParser_ParseStream_Cancellation(t *testing.T) {
	parser := NewJavacodeStreamParser()

	javaCode := `public class TestClass {
	public void testMethod() {
		System.out.println("Hello World");
	}
}`
	reader := strings.NewReader(javaCode)

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

func TestJavacodeStreamParser_SetChunkSize(t *testing.T) {
	parser := NewJavacodeStreamParser()
	parser.SetChunkSize(1000)
	assert.Equal(t, 1000, parser.legacyParser.chunkSize)
}

func TestJavacodeStreamParser_SetChunkOverlap(t *testing.T) {
	parser := NewJavacodeStreamParser()
	parser.SetChunkOverlap(100)
	assert.Equal(t, 100, parser.legacyParser.chunkOverlap)
}

func TestJavacodeStreamParser_SetExtractMethods(t *testing.T) {
	parser := NewJavacodeStreamParser()
	parser.SetExtractMethods(false)
	assert.False(t, parser.legacyParser.extractMethods)
}

func TestJavacodeStreamParser_SetExtractClasses(t *testing.T) {
	parser := NewJavacodeStreamParser()
	parser.SetExtractClasses(false)
	assert.False(t, parser.legacyParser.extractClasses)
}

func TestJavacodeStreamParser_SetExtractComments(t *testing.T) {
	parser := NewJavacodeStreamParser()
	parser.SetExtractComments(false)
	assert.False(t, parser.legacyParser.extractComments)
}

func TestJavacodeStreamParser_ParseStream_WithComments(t *testing.T) {
	parser := NewJavacodeStreamParser()

	javaCode := `// This is a class comment
public class TestClass {
	// This is a method comment
	public void testMethod() {
		/* This is a
		multi-line comment */
		System.out.println("Hello World");
	}
}`
	reader := strings.NewReader(javaCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "TestClass")
	assert.Contains(t, docs[0].Content, "testMethod")
	assert.Contains(t, docs[0].Content, "Hello World")
	// Comments are extracted separately and appended
	assert.Contains(t, docs[0].Content, "This is a class comment")
}

func TestJavacodeStreamParser_ParseStream_WithoutMethods(t *testing.T) {
	parser := NewJavacodeStreamParser()
	parser.SetExtractMethods(false)

	javaCode := `public class TestClass {
	public void testMethod() {
		System.out.println("Hello World");
	}
}`
	reader := strings.NewReader(javaCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "TestClass")
	// Method should not be extracted separately
}

func TestJavacodeStreamParser_ParseStream_WithoutClasses(t *testing.T) {
	parser := NewJavacodeStreamParser()
	parser.SetExtractClasses(false)

	javaCode := `public class TestClass {
	public void testMethod() {
		System.out.println("Hello World");
	}
}`
	reader := strings.NewReader(javaCode)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	// Class should not be extracted separately
}
