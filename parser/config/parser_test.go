package config

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Basic(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(500)

	content := `# Configuration
database.host = localhost
database.port = 5432
database.password = secret123

[server]
port = 8080
host = 0.0.0.0
`

	ctx := context.Background()
	var chunkCount int
	var firstChunk *core.Chunk

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		chunkCount++
		if firstChunk == nil {
			firstChunk = &chunk
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if chunkCount == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// Verify metadata
	if firstChunk.Metadata["streaming"] != "true" {
		t.Errorf("Chunk should be marked as streaming")
	}
	if firstChunk.Metadata["type"] != "config" {
		t.Errorf("Chunk has wrong type: %s", firstChunk.Metadata["type"])
	}
}

func TestParser_SecretMasking(t *testing.T) {
	parser := NewParser()
	parser.SetMaskSecrets(true)
	parser.SetChunkSize(500)

	content := `# Database config
db.password = mysecretpassword
api_key = sk-1234567890
normal_key = normal_value
`

	ctx := context.Background()
	var foundPassword bool
	var foundAPIKey bool

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		if strings.Contains(chunk.Content, "password") && strings.Contains(chunk.Content, "***MASKED***") {
			foundPassword = true
		}
		if strings.Contains(chunk.Content, "api_key") && strings.Contains(chunk.Content, "***MASKED***") {
			foundAPIKey = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if !foundPassword {
		t.Error("Expected password to be masked")
	}
	if !foundAPIKey {
		t.Error("Expected API key to be masked")
	}
}

func TestParser_EnvExpansion(t *testing.T) {
	parser := NewParser()
	parser.SetExpandEnv(true)
	parser.SetChunkSize(500)

	// Set a test environment variable
	t.Setenv("TEST_PORT", "9090")

	content := `server.port = ${TEST_PORT}
server.host = localhost
`

	ctx := context.Background()
	var foundExpanded bool

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		if strings.Contains(chunk.Content, "server.port = 9090") {
			foundExpanded = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if !foundExpanded {
		t.Error("Expected environment variable to be expanded")
	}
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50)

	// Create large content
	var builder strings.Builder
	for i := 0; i < 10000; i++ {
		builder.WriteString("key")
		builder.WriteString(string(rune(i)))
		builder.WriteString(" = value\n")
	}
	content := builder.String()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := parser.Parse(ctx, strings.NewReader(content))
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestParser_EmptyContent(t *testing.T) {
	parser := NewParser()

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(""))
	if err != nil {
		t.Fatalf("Failed to parse empty content: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got: %d", len(chunks))
	}
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()

	content := "# Config\nkey = value\n"

	ctx := context.Background()
	expectedErr := "callback error"

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		return &testError{msg: expectedErr}
	})

	if err == nil || !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected callback error, got: %v", err)
	}
}

func TestParser_ChunkSizing(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50)
	parser.SetChunkOverlap(10)

	content := strings.Repeat("key = value\n", 100)

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks")
	}

	// Verify chunk sizes
	for i, chunk := range chunks {
		if len(chunk.Content) > 100 { // chunkSize + overlap tolerance
			t.Errorf("Chunk %d exceeds size limit: %d bytes", i, len(chunk.Content))
		}
	}
}

func TestParser_MetadataCompleteness(t *testing.T) {
	parser := NewParser()

	content := "# Test\nkey = value\n"

	ctx := context.Background()
	var firstChunk *core.Chunk

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		if firstChunk == nil {
			firstChunk = &chunk
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if firstChunk == nil {
		t.Fatal("Expected at least one chunk")
	}

	requiredFields := []string{"type", "position", "format", "streaming", "masked", "env_expanded"}
	for _, field := range requiredFields {
		if _, ok := firstChunk.Metadata[field]; !ok {
			t.Errorf("Missing required metadata field: %s", field)
		}
	}
}

func TestParser_DisableMasking(t *testing.T) {
	parser := NewParser()
	parser.SetMaskSecrets(false)

	content := "password = secret123\n"

	ctx := context.Background()
	var foundUnmasked bool

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		if strings.Contains(chunk.Content, "password = secret123") {
			foundUnmasked = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if !foundUnmasked {
		t.Error("Expected password to remain unmasked when masking is disabled")
	}
}

// Helper type
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
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
				assert.Equal(t, "config", chunk.Metadata["type"], "Chunk %d should have type 'config'", i)
			}
		})
	}
}
