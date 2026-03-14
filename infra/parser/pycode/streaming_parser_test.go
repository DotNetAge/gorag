package pycode

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPycodeStreamParser_ParseStream(t *testing.T) {
	parser := NewPycodeStreamParser()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pythonContent := []byte(`def hello(name):
    return f"Hello, {name}!"

class Person:
    def __init__(self, name):
        self.name = name

# This is a comment
print(hello("World"))`)

	r := bytes.NewReader(pythonContent)
	docCh, err := parser.ParseStream(ctx, r, nil)
	require.NoError(t, err)

	var documents []interface{}
	for doc := range docCh {
		documents = append(documents, doc)
	}

	assert.NotEmpty(t, documents)
}

func TestPycodeStreamParser_FunctionExtraction(t *testing.T) {
	parser := NewPycodeStreamParser()
	parser.SetExtractFunctions(true)
	ctx := context.Background()

	pythonContent := []byte(`def greet(name):
    """Say hello"""
    print(f"Hello, {name}!")`)

	docCh, err := parser.ParseStream(ctx, bytes.NewReader(pythonContent), nil)
	require.NoError(t, err)

	var foundGreet bool
	for doc := range docCh {
		if meta, ok := doc.Metadata["element_type"].(string); ok && meta == "function" {
			if funcName, ok := doc.Metadata["function_name"].(string); ok && funcName == "greet" {
				foundGreet = true
			}
		}
	}

	assert.True(t, foundGreet, "Should extract greet function")
}

func TestPycodeStreamParser_ClassExtraction(t *testing.T) {
	parser := NewPycodeStreamParser()
	parser.SetExtractClasses(true)
	ctx := context.Background()

	pythonContent := []byte(`class Calculator:
    def add(self, a, b):
        return a + b`)

	docCh, err := parser.ParseStream(ctx, bytes.NewReader(pythonContent), nil)
	require.NoError(t, err)

	var foundCalculator bool
	for doc := range docCh {
		if meta, ok := doc.Metadata["element_type"].(string); ok && meta == "class" {
			if className, ok := doc.Metadata["class_name"].(string); ok && className == "Calculator" {
				foundCalculator = true
			}
		}
	}

	assert.True(t, foundCalculator, "Should extract Calculator class")
}

func TestPycodeStreamParser_EmptyCode(t *testing.T) {
	parser := NewPycodeStreamParser()
	ctx := context.Background()

	pythonContent := []byte(``)
	docCh, err := parser.ParseStream(ctx, bytes.NewReader(pythonContent), nil)
	require.NoError(t, err)

	var documents []interface{}
	for doc := range docCh {
		documents = append(documents, doc)
	}

	// May be empty
}

func TestPycodeStreamParser_LargeCode(t *testing.T) {
	parser := NewPycodeStreamParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	var sb strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, "def function_%d(x):\n    return x * %d\n\n", i, i)
	}

	docCh, err := parser.ParseStream(ctx, strings.NewReader(sb.String()), nil)
	require.NoError(t, err)

	var documents []interface{}
	for doc := range docCh {
		documents = append(documents, doc)
	}

	assert.NotEmpty(t, documents)
}

func TestPycodeStreamParser_ChunkConfiguration(t *testing.T) {
	parser := NewPycodeStreamParser()
	parser.SetChunkSize(200)
	parser.SetChunkOverlap(20)

	// We can't directly access private fields, so we just test that the methods don't panic
	assert.NotPanics(t, func() {
		parser.SetChunkSize(300)
		parser.SetChunkOverlap(30)
	})
}

func TestPycodeStreamParser_ConfigurationOptions(t *testing.T) {
	parser := NewPycodeStreamParser()

	parser.SetExtractFunctions(false)
	parser.SetExtractClasses(false)
	parser.SetExtractComments(false)

	// We can't directly access private fields, so we just test that the methods don't panic
	assert.NotPanics(t, func() {
		parser.SetExtractFunctions(true)
		parser.SetExtractClasses(true)
		parser.SetExtractComments(true)
	})
}

func TestPycodeStreamParser_SupportedFormats(t *testing.T) {
	parser := NewPycodeStreamParser()
	formats := parser.GetSupportedTypes()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".py", formats[0])
}

func TestPycodeStreamParser_ParseStream_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewPycodeStreamParser()
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
			docCh, err := parser.ParseStream(ctx, reader, nil)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify results
			var documents []interface{}
			for doc := range docCh {
				documents = append(documents, doc)
			}

			// May be empty
		})
	}
}