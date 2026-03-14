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

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()
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
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	pythonContent := []byte(`def add(a, b):
    return a + b

def subtract(a, b):
    return a - b`)
	var chunkCount int
	var foundFunction bool

	err := parser.ParseWithCallback(ctx, bytes.NewReader(pythonContent), func(chunk model.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "pycode")
		if chunk.Metadata["element_type"] == "function" {
			foundFunction = true
		}
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
	assert.True(t, foundFunction, "Should find at least one function")
}

func TestParser_FunctionExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractFunctions(true)
	ctx := context.Background()

	pythonContent := []byte(`def greet(name):
    """Say hello"""
    print(f"Hello, {name}!")`)

	var foundGreet bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(pythonContent), func(chunk model.Chunk) error {
		if chunk.Metadata["element_type"] == "function" {
			if chunk.Metadata["function_name"] == "greet" {
				foundGreet = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundGreet, "Should extract greet function")
}

func TestParser_ClassExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractClasses(true)
	ctx := context.Background()

	pythonContent := []byte(`class Calculator:
    def add(self, a, b):
        return a + b`)

	var foundCalculator bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(pythonContent), func(chunk model.Chunk) error {
		if chunk.Metadata["element_type"] == "class" {
			if chunk.Metadata["class_name"] == "Calculator" {
				foundCalculator = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundCalculator, "Should extract Calculator class")
}

func TestParser_EmptyCode(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	pythonContent := []byte(``)
	chunks, err := parser.Parse(ctx, bytes.NewReader(pythonContent))
	require.NoError(t, err)
	_ = chunks
}

func TestParser_LargeCode(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	var sb strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, "def function_%d(x):\n    return x * %d\n\n", i, i)
	}

	chunks, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	var sb strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&sb, "def func_%d(): pass\n", i)
	}

	cancel()
	_, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(10)
	ctx := context.Background()

	// Use code that will produce multiple chunks
	pythonContent := []byte(`# Comment line 1
# Comment line 2
# Comment line 3
print("test")`)
	err := parser.ParseWithCallback(ctx, bytes.NewReader(pythonContent), func(chunk model.Chunk) error {
		return assert.AnError
	})
	assert.Error(t, err)
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(200)
	parser.SetChunkOverlap(20)

	assert.Equal(t, 200, parser.chunkSize)
	assert.Equal(t, 20, parser.chunkOverlap)
}

func TestParser_ConfigurationOptions(t *testing.T) {
	parser := NewParser()

	parser.SetExtractFunctions(false)
	parser.SetExtractClasses(false)
	parser.SetExtractComments(false)

	assert.False(t, parser.extractFunctions)
	assert.False(t, parser.extractClasses)
	assert.False(t, parser.extractComments)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".py", formats[0])
}

func TestParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewParser()
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
			chunks, err := parser.Parse(ctx, reader)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify chunks
			for i, chunk := range chunks {
				assert.NotEmpty(t, chunk.ID, "Chunk %d should have an ID", i)
				assert.NotEmpty(t, chunk.Content, "Chunk %d should have content", i)
				assert.Contains(t, chunk.Metadata["type"], "pycode", "Chunk %d should have type 'pycode'", i)
			}
		})
	}
}
