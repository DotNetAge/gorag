package tscode

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

	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tsContent := []byte(`interface User {
    id: number;
    name: string;
}

class UserService {
    getUser(id: number): User {
        return { id, name: "Test" };
    }
}`)

	r := bytes.NewReader(tsContent)
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	tsContent := []byte("function greet(name: string): string {\n    return `Hello, ${name}!`;\n}\n\ninterface Greeting {\n    message: string;\n}")

	var chunkCount int
	var foundFunction bool

	err := parser.ParseWithCallback(ctx, bytes.NewReader(tsContent), func(chunk core.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "tscode")
		if chunk.Metadata["chunk_type"] == "function" {
			foundFunction = true
		}
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
	assert.True(t, foundFunction, "Should find at least one function")
}

func TestParser_InterfaceExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractInterfaces(true)
	ctx := context.Background()

	tsContent := []byte(`export interface ApiResponse<T> {
    data: T;
    status: number;
}`)

	var foundInterface bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(tsContent), func(chunk core.Chunk) error {
		if chunk.Metadata["chunk_type"] == "interface" {
			if chunk.Metadata["interface_name"] == "ApiResponse" {
				foundInterface = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundInterface, "Should extract ApiResponse interface")
}

func TestParser_ClassExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractClasses(true)
	ctx := context.Background()

	tsContent := []byte(`export class DataProcessor {
    process(data: string[]): string[] {
        return data.map(item => item.toUpperCase());
    }
}`)

	var foundClass bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(tsContent), func(chunk core.Chunk) error {
		if chunk.Metadata["chunk_type"] == "class" {
			if chunk.Metadata["class_name"] == "DataProcessor" {
				foundClass = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundClass, "Should find DataProcessor class")
}

func TestParser_TypeAliasExtraction(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	tsContent := []byte(`export type UserID = string | number;`)

	var foundType bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(tsContent), func(chunk core.Chunk) error {
		if chunk.Metadata["chunk_type"] == "type" {
			if chunk.Metadata["type_name"] == "UserID" {
				foundType = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundType, "Should extract UserID type")
}

func TestParser_EmptyCode(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	tsContent := []byte(``)
	chunks, err := parser.Parse(ctx, bytes.NewReader(tsContent))
	require.NoError(t, err)
	_ = chunks
}

func TestParser_LargeCode(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	var sb strings.Builder
	sb.WriteString("export class LargeClass {\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "    method%d(): void {\n        console.log(\"Method %d\");\n    }\n\n", i, i)
	}
	sb.WriteString("}")

	chunks, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	var sb strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&sb, "export class Class%d { method(): void {} }\n", i)
	}

	cancel()
	_, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(20)
	ctx := context.Background()

	tsContent := []byte(`// Comment 1
// Comment 2
const x: number = 10;`)

	err := parser.ParseWithCallback(ctx, bytes.NewReader(tsContent), func(chunk core.Chunk) error {
		return assert.AnError
	})
	assert.Error(t, err)
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(700)
	parser.SetChunkOverlap(70)

	assert.Equal(t, 700, parser.chunkSize)
	assert.Equal(t, 70, parser.chunkOverlap)
}

func TestParser_ConfigurationOptions(t *testing.T) {
	parser := NewParser()

	parser.SetExtractFunctions(false)
	parser.SetExtractClasses(false)
	parser.SetExtractInterfaces(false)
	parser.SetExtractComments(false)

	assert.False(t, parser.extractFunctions)
	assert.False(t, parser.extractClasses)
	assert.False(t, parser.extractInterfaces)
	assert.False(t, parser.extractComments)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 2)
	assert.Contains(t, formats, ".ts")
	assert.Contains(t, formats, ".tsx")
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
				assert.Contains(t, chunk.Metadata["type"], "tscode", "Chunk %d should have type 'tscode'", i)
			}
		})
	}
}
