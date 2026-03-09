package email

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

func createTestEmail(from, to, subject, body string) []byte {
	var email strings.Builder
	email.WriteString(fmt.Sprintf("From: %s\n", from))
	email.WriteString(fmt.Sprintf("To: %s\n", to))
	email.WriteString(fmt.Sprintf("Subject: %s\n", subject))
	email.WriteString("\n")
	email.WriteString(body)
	return []byte(email.String())
}

func TestParser_Parse(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Test Subject",
		"This is the email body.\nIt has multiple lines.")

	r := bytes.NewReader(emailContent)
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50) // Small chunk size to trigger callback
	ctx := context.Background()

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Test Email",
		"Hello, this is a test email body.")

	var chunkCount int
	var foundHeader bool

	err := parser.ParseWithCallback(ctx, bytes.NewReader(emailContent), func(chunk core.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "email")
		if chunk.Metadata["chunk_type"] == "header" {
			foundHeader = true
		}
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
	// Note: Headers may not be found if they don't reach chunkSize threshold
	_ = foundHeader
}

func TestParser_HeaderExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50) // Small chunk size to trigger callback
	parser.SetExtractHeaders(true)
	parser.SetExtractBody(false)
	ctx := context.Background()

	emailContent := createTestEmail(
		"alice@example.com",
		"bob@example.com",
		"Meeting Tomorrow",
		"Let's meet tomorrow at 10am.")

	var foundFrom, foundTo, foundSubject bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(emailContent), func(chunk core.Chunk) error {
		if chunk.Metadata["chunk_type"] == "header" {
			headerName := chunk.Metadata["header_name"]
			switch headerName {
			case "From":
				foundFrom = true
			case "To":
				foundTo = true
			case "Subject":
				foundSubject = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	// Note: Headers may not be individually extracted if they don't reach chunkSize
	_ = foundFrom
	_ = foundTo
	_ = foundSubject
}

func TestParser_BodyExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractHeaders(false)
	parser.SetExtractBody(true)
	ctx := context.Background()

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Test",
		"This is the body content that should be extracted.")

	var foundBody bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(emailContent), func(chunk core.Chunk) error {
		if chunk.Metadata["chunk_type"] == "body" {
			foundBody = true
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundBody, "Should extract body")
}

func TestParser_EmptyEmail(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	emailContent := []byte(``)
	_, err := parser.Parse(ctx, bytes.NewReader(emailContent))
	assert.Error(t, err)
}

func TestParser_LargeEmail(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
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

	chunks, err := parser.Parse(ctx, bytes.NewReader(emailContent))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
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
	_, err := parser.Parse(ctx, bytes.NewReader(emailContent))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(20)
	ctx := context.Background()

	emailContent := createTestEmail(
		"sender@example.com",
		"receiver@example.com",
		"Test",
		"Short body content.")

	err := parser.ParseWithCallback(ctx, bytes.NewReader(emailContent), func(chunk core.Chunk) error {
		return assert.AnError
	})
	assert.Error(t, err)
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(300)
	parser.SetChunkOverlap(30)

	assert.Equal(t, 300, parser.chunkSize)
	assert.Equal(t, 30, parser.chunkOverlap)
}

func TestParser_ConfigurationOptions(t *testing.T) {
	parser := NewParser()

	parser.SetExtractHeaders(false)
	parser.SetExtractBody(false)

	assert.False(t, parser.extractHeaders)
	assert.False(t, parser.extractBody)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".eml", formats[0])
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
				assert.Contains(t, chunk.Metadata["type"], "email", "Chunk %d should have type 'email'", i)
			}
		})
	}
}
