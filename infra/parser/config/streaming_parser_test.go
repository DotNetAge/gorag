package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigStreamParser_Basic(t *testing.T) {
	parser := NewConfigStreamParser()

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
	var firstDoc *entity.Document

	docChan, err := parser.ParseStream(ctx, strings.NewReader(content), nil)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for doc := range docChan {
		chunkCount++
		if firstDoc == nil {
			firstDoc = doc
		}
	}

	if chunkCount == 0 {
		t.Fatal("Expected at least one document")
	}

	// Verify metadata
	if firstDoc.Metadata["streaming"] != "true" {
		t.Errorf("Document should be marked as streaming")
	}
	if firstDoc.Metadata["parser"] != "ConfigStreamParser" {
		t.Errorf("Document has wrong parser: %v", firstDoc.Metadata["parser"])
	}
}

func TestConfigStreamParser_SecretMasking(t *testing.T) {
	parser := NewConfigStreamParser()

	content := `# Database config
 db.password = mysecretpassword
 api_key = sk-1234567890
 normal_key = normal_value
`

	ctx := context.Background()
	var foundPassword bool
	var foundAPIKey bool

	docChan, err := parser.ParseStream(ctx, strings.NewReader(content), nil)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for doc := range docChan {
		if strings.Contains(doc.Content, "password") && strings.Contains(doc.Content, "***MASKED***") {
			foundPassword = true
		}
		if strings.Contains(doc.Content, "api_key") && strings.Contains(doc.Content, "***MASKED***") {
			foundAPIKey = true
		}
	}

	if !foundPassword {
		t.Error("Expected password to be masked")
	}
	if !foundAPIKey {
		t.Error("Expected API key to be masked")
	}
}

func TestConfigStreamParser_EnvExpansion(t *testing.T) {
	parser := NewConfigStreamParser()
	parser.expandEnv = true // Enable env expansion

	// Set a test environment variable
	t.Setenv("TEST_PORT", "9090")

	content := `server.port = ${TEST_PORT}
 server.host = localhost
`

	ctx := context.Background()
	var foundExpanded bool

	docChan, err := parser.ParseStream(ctx, strings.NewReader(content), nil)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for doc := range docChan {
		if strings.Contains(doc.Content, "server.port = 9090") {
			foundExpanded = true
		}
	}

	if !foundExpanded {
		t.Error("Expected environment variable to be expanded")
	}
}

func TestConfigStreamParser_ContextCancellation(t *testing.T) {
	parser := NewConfigStreamParser()

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

	docChan, err := parser.ParseStream(ctx, strings.NewReader(content), nil)
	if err != nil {
		t.Fatalf("Failed to create parser stream: %v", err)
	}

	// Read from channel - should close immediately
	docCount := 0
	for range docChan {
		docCount++
	}

	if docCount > 0 {
		t.Errorf("Expected no documents due to context cancellation, got: %d", docCount)
	}
}

func TestConfigStreamParser_EmptyContent(t *testing.T) {
	parser := NewConfigStreamParser()

	ctx := context.Background()

docChan, err := parser.ParseStream(ctx, strings.NewReader(""), nil)
	if err != nil {
		t.Fatalf("Failed to parse empty content: %v", err)
	}

	// Read from channel - should close immediately
	docCount := 0
	for range docChan {
		docCount++
	}

	if docCount != 0 {
		t.Errorf("Expected 0 documents for empty content, got: %d", docCount)
	}
}

func TestConfigStreamParser_ChunkSizing(t *testing.T) {
	parser := NewConfigStreamParser()

	content := strings.Repeat("key = value\n", 100)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, strings.NewReader(content), nil)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	docs := make([]*entity.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		t.Fatal("Expected documents")
	}

	// Verify document sizes
	for i, doc := range docs {
		if len(doc.Content) > 1000 { // Reasonable size limit
			t.Errorf("Document %d exceeds size limit: %d bytes", i, len(doc.Content))
		}
	}
}

func TestConfigStreamParser_MetadataCompleteness(t *testing.T) {
	parser := NewConfigStreamParser()
	parser.chunkSize = 10 // Reduce chunk size for test

	content := `# Test
key = value
another_key = another_value
third_key = third_value
`

	ctx := context.Background()
	var firstDoc *entity.Document

	docChan, err := parser.ParseStream(ctx, strings.NewReader(content), nil)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for doc := range docChan {
		if firstDoc == nil {
			firstDoc = doc
		}
	}

	if firstDoc == nil {
		t.Fatal("Expected at least one document")
	}

	// Verify metadata
	assert.Equal(t, "ConfigStreamParser", firstDoc.Metadata["parser"])
	assert.Equal(t, "true", firstDoc.Metadata["streaming"])
	assert.NotNil(t, firstDoc.Metadata["masked"])
	assert.NotNil(t, firstDoc.Metadata["env_expanded"])
}

func TestConfigStreamParser_DisableMasking(t *testing.T) {
	parser := NewConfigStreamParser()
	parser.maskSecrets = false // Disable masking

	content := "password = secret123\n"

	ctx := context.Background()
	var foundUnmasked bool

	docChan, err := parser.ParseStream(ctx, strings.NewReader(content), nil)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for doc := range docChan {
		if strings.Contains(doc.Content, "password = secret123") {
			foundUnmasked = true
		}
	}

	if !foundUnmasked {
		t.Error("Expected password to remain unmasked when masking is disabled")
	}
}

func TestConfigStreamParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewConfigStreamParser()
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
			docChan, err := parser.ParseStream(ctx, reader, nil)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify documents
			docCount := 0
			for doc := range docChan {
				docCount++
				assert.NotEmpty(t, doc.ID, "Document should have an ID")
				assert.NotEmpty(t, doc.Content, "Document should have content")
				assert.Equal(t, "ConfigStreamParser", doc.Metadata["parser"], "Document should have correct parser")
			}
			assert.Greater(t, docCount, 0, "Should have at least one document")
		})
	}
}
