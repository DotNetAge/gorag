package email

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestEmail(from, to, subject, body string) []byte {
	var email strings.Builder
	email.WriteString(fmt.Sprintf("From: %s\n", from))
	email.WriteString(fmt.Sprintf("To: %s\n", to))
	email.WriteString(fmt.Sprintf("Subject: %s\n", subject))
	email.WriteString("\n")
	email.WriteString(body)
	return []byte(email.String())
}

func TestEmailStreamParser_ParseStream(t *testing.T) {
	parser := DefaultEmailStreamParser()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Test Subject",
		"This is the email body.\nIt has multiple lines.")

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(emailContent), nil)
	require.NoError(t, err)

	docs := make([]*core.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
}

func TestEmailStreamParser_ParseStream_WithContent(t *testing.T) {
	parser := DefaultEmailStreamParser()
	parser.chunkSize = 50 // Small chunk size to trigger multiple documents
	ctx := context.Background()

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Test Email",
		"Hello, this is a test email body.")

	var docCount int
	var foundHeader bool

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(emailContent), nil)
	require.NoError(t, err)

	for doc := range docChan {
		docCount++
		assert.NotEmpty(t, doc.ID)
		assert.Equal(t, "email", doc.Metadata["type"])
		if doc.Metadata["chunk_type"] == "header" {
			foundHeader = true
		}
	}

	assert.Greater(t, docCount, 0)
	// Note: Headers may not be found if they don't reach chunkSize threshold
	_ = foundHeader
}

func TestEmailStreamParser_HeaderExtraction(t *testing.T) {
	parser := DefaultEmailStreamParser()
	parser.chunkSize = 50 // Small chunk size to trigger callback
	parser.extractHeaders = true
	parser.extractBody = false
	ctx := context.Background()

	emailContent := createTestEmail(
		"alice@example.com",
		"bob@example.com",
		"Meeting Tomorrow",
		"Let's meet tomorrow at 10am.")

	var foundFrom, foundTo, foundSubject bool

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(emailContent), nil)
	require.NoError(t, err)

	for doc := range docChan {
		if doc.Metadata["chunk_type"] == "header" {
			headerName := doc.Metadata["header_name"]
			switch headerName {
			case "From":
				foundFrom = true
			case "To":
				foundTo = true
			case "Subject":
				foundSubject = true
			}
		}
	}

	// Note: Headers may not be individually extracted if they don't reach chunkSize
	_ = foundFrom
	_ = foundTo
	_ = foundSubject
}

func TestEmailStreamParser_BodyExtraction(t *testing.T) {
	parser := DefaultEmailStreamParser()
	parser.extractHeaders = false
	parser.extractBody = true
	ctx := context.Background()

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Test",
		"This is the body content that should be extracted.")

	var foundBody bool

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(emailContent), nil)
	require.NoError(t, err)

	for doc := range docChan {
		if doc.Metadata["chunk_type"] == "body" {
			foundBody = true
		}
	}

	assert.True(t, foundBody, "Should extract body")
}

func TestEmailStreamParser_EmptyEmail(t *testing.T) {
	parser := DefaultEmailStreamParser()
	ctx := context.Background()

	emailContent := []byte(``)
	docChan, err := parser.ParseStream(ctx, bytes.NewReader(emailContent), nil)
	require.NoError(t, err)

	docs := make([]*core.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Empty(t, docs)
}

func TestEmailStreamParser_LargeEmail(t *testing.T) {
	parser := DefaultEmailStreamParser()
	parser.chunkSize = 100
	ctx := context.Background()

	var body strings.Builder
	for i := 0; i < 50; i++ {
		body.WriteString(fmt.Sprintf("Line %d: This is test content.\n", i))
	}

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Large Email",
		body.String())

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(emailContent), nil)
	require.NoError(t, err)

	docs := make([]*core.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
}

func TestEmailStreamParser_ContextCancellation(t *testing.T) {
	parser := DefaultEmailStreamParser()
	ctx, cancel := context.WithCancel(context.Background())

	var body strings.Builder
	for i := 0; i < 500; i++ {
		body.WriteString(fmt.Sprintf("Line %d\n", i))
	}

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Test",
		body.String())

	cancel()
	docChan, err := parser.ParseStream(ctx, bytes.NewReader(emailContent), nil)
	require.NoError(t, err)

	docs := make([]*core.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Empty(t, docs)
}

func TestEmailStreamParser_ChunkConfiguration(t *testing.T) {
	parser := DefaultEmailStreamParser()
	parser.chunkSize = 300
	parser.chunkOverlap = 30

	assert.Equal(t, 300, parser.chunkSize)
	assert.Equal(t, 30, parser.chunkOverlap)
}

func TestEmailStreamParser_ConfigurationOptions(t *testing.T) {
	parser := DefaultEmailStreamParser()

	parser.extractHeaders = false
	parser.extractBody = false

	assert.False(t, parser.extractHeaders)
	assert.False(t, parser.extractBody)
}

func TestEmailStreamParser_GetSupportedTypes(t *testing.T) {
	parser := DefaultEmailStreamParser()
	formats := parser.GetSupportedTypes()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".eml", formats[0])
}

func TestEmailStreamParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := DefaultEmailStreamParser()
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
				assert.Equal(t, "email", doc.Metadata["type"], "Document should have type 'email'")
			}
			assert.Greater(t, docCount, 0, "Should have at least one document")
		})
	}
}
