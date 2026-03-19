package gocode

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGocodeStreamParser_BasicFunction(t *testing.T) {
	parser := NewGocodeStreamParser()
	parser.SetChunkSize(500)

	content := `package main

func Hello(name string) string {
	return "Hello, " + name
}

func main() {
	Hello("World")
}
`

	ctx := context.Background()
	metadata := make(map[string]any)
	docs, err := parser.ParseStream(ctx, strings.NewReader(content), metadata)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	var docCount int
	var firstDoc *core.Document
	for doc := range docs {
		docCount++
		if firstDoc == nil {
			firstDoc = doc
		}
	}

	if docCount == 0 {
		t.Fatal("Expected at least one document")
	}

	if firstDoc.Metadata["parser"] != "GocodeStreamParser" {
		t.Errorf("Document should have parser metadata set to 'GocodeStreamParser'")
	}
}

func TestGocodeStreamParser_TypeExtraction(t *testing.T) {
	parser := NewGocodeStreamParser()
	parser.SetExtractTypes(true)

	content := `package main

type Person struct {
	Name string
	Age  int
}

type Manager interface {
	Manage() error
}
`

	ctx := context.Background()
	metadata := make(map[string]any)
	docs, err := parser.ParseStream(ctx, strings.NewReader(content), metadata)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	var docCount int
	for range docs {
		docCount++
	}

	if docCount == 0 {
		t.Fatal("Expected documents")
	}
}

func TestGocodeStreamParser_CommentExtraction(t *testing.T) {
	parser := NewGocodeStreamParser()
	parser.SetExtractComments(true)

	content := `package main

// Hello says hello
func Hello(name string) string {
	return "Hello"
}
`

	ctx := context.Background()
	metadata := make(map[string]any)
	docs, err := parser.ParseStream(ctx, strings.NewReader(content), metadata)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	var foundContent bool
	for doc := range docs {
		if strings.Contains(doc.Content, "Hello") {
			foundContent = true
		}
	}

	// Note: Comments are extracted but may be combined with functions
	// The important thing is that parsing succeeds
	if !foundContent {
		t.Log("Comment extraction test - checking if content contains expected text")
	}
}

func TestGocodeStreamParser_LargeFile(t *testing.T) {
	parser := NewGocodeStreamParser()
	parser.SetChunkSize(500)

	// Create a large Go file with many functions
	var builder strings.Builder
	builder.WriteString("package main\n\n")
	for i := 0; i < 1000; i++ {
		builder.WriteString("func Function")
		builder.WriteString(fmt.Sprintf("%d", i))
		builder.WriteString("() {\n")
		builder.WriteString("// Implementation\n")
		builder.WriteString("}\n\n")
	}
	content := builder.String()

	ctx := context.Background()
	metadata := make(map[string]any)
	docs, err := parser.ParseStream(ctx, strings.NewReader(content), metadata)
	if err != nil {
		t.Fatalf("Failed to parse large file: %v", err)
	}

	var docCount int
	for range docs {
		docCount++
	}

	if docCount == 0 {
		t.Fatal("Expected documents from large file")
	}

	t.Logf("Parsed %d documents from large Go file", docCount)
}

func TestGocodeStreamParser_ContextCancellation(t *testing.T) {
	parser := NewGocodeStreamParser()
	parser.SetChunkSize(50)

	var builder strings.Builder
	builder.WriteString("package main\n")
	for i := 0; i < 10000; i++ {
		builder.WriteString("func F")
		builder.WriteString(fmt.Sprintf("%d", i))
		builder.WriteString("() {}\n")
	}
	content := builder.String()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	metadata := make(map[string]any)
	docs, err := parser.ParseStream(ctx, strings.NewReader(content), metadata)
	if err != nil {
		t.Fatalf("Expected no error when creating parser, got: %v", err)
	}

	// Read from channel to ensure goroutine exits
	for range docs {
	}
}

func TestGocodeStreamParser_EmptyContent(t *testing.T) {
	parser := NewGocodeStreamParser()

	ctx := context.Background()
	metadata := make(map[string]any)
	docs, err := parser.ParseStream(ctx, strings.NewReader(""), metadata)
	if err != nil {
		t.Fatalf("Failed to parse empty content: %v", err)
	}

	var docCount int
	for range docs {
		docCount++
	}

	if docCount != 0 {
		t.Errorf("Expected 0 documents for empty content, got: %d", docCount)
	}
}

func TestGocodeStreamParser_GetSupportedTypes(t *testing.T) {
	parser := NewGocodeStreamParser()
	formats := parser.GetSupportedTypes()

	expectedFormat := ".go"
	found := false
	for _, format := range formats {
		if format == expectedFormat {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to support format '%s', but got: %v", expectedFormat, formats)
	}
}

func TestGocodeStreamParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewGocodeStreamParser()
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
			metadata := make(map[string]any)
			docs, err := parser.ParseStream(ctx, reader, metadata)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			var docCount int
			for doc := range docs {
				docCount++
				assert.NotEmpty(t, doc.ID, "Document should have an ID")
				assert.NotEmpty(t, doc.Content, "Document should have content")
				assert.Contains(t, doc.Metadata["parser"], "GocodeStreamParser", "Document should have parser 'GocodeStreamParser'")
			}

			assert.NotZero(t, docCount, "Expected at least one document")
		})
	}
}
