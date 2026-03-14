package base

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestGenericStreamWrapper_New(t *testing.T) {
	// Create a test extractor function
	extractor := func(ctx context.Context, r io.Reader) (string, error) {
		return "Test content", nil
	}

	// Test creating a new GenericStreamWrapper
	supportedTypes := []string{"txt", "md"}
	wrapper := NewGenericStreamWrapper("test-parser", supportedTypes, extractor)

	// Check that the wrapper is created correctly
	assert.NotNil(t, wrapper)
	assert.Equal(t, "test-parser", wrapper.parserName)
	assert.Equal(t, supportedTypes, wrapper.supportedTypes)
	assert.NotNil(t, wrapper.extractor)
}

func TestGenericStreamWrapper_GetSupportedTypes(t *testing.T) {
	// Create a test extractor function
	extractor := func(ctx context.Context, r io.Reader) (string, error) {
		return "Test content", nil
	}

	// Create a GenericStreamWrapper
	supportedTypes := []string{"txt", "md"}
	wrapper := NewGenericStreamWrapper("test-parser", supportedTypes, extractor)

	// Test GetSupportedTypes
	types := wrapper.GetSupportedTypes()
	assert.Equal(t, supportedTypes, types)
}

func TestGenericStreamWrapper_ParseStream(t *testing.T) {
	// Create a test extractor function that returns test content
	extractor := func(ctx context.Context, r io.Reader) (string, error) {
		// Read content from the reader
		buf := make([]byte, 1024)
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		return string(buf[:n]), nil
	}

	// Create a GenericStreamWrapper
	supportedTypes := []string{"txt"}
	wrapper := NewGenericStreamWrapper("test-parser", supportedTypes, extractor)

	// Create test content and reader
	testContent := "This is a test content"
	reader := strings.NewReader(testContent)

	// Create metadata
	metadata := map[string]any{
		"source": "test-file.txt",
		"author": "test-author",
	}

	// Test ParseStream
	ctx := context.Background()
	docChan, err := wrapper.ParseStream(ctx, reader, metadata)
	assert.NoError(t, err)

	// Read from the channel
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.NotEmpty(t, doc.ID)
	assert.Equal(t, testContent, doc.Content)
	assert.Equal(t, "test-file.txt", doc.Source)
	assert.Equal(t, "text/plain", doc.ContentType)

	// Check metadata
	assert.Equal(t, "test-parser", doc.Metadata["parser"])
	assert.Equal(t, "test-file.txt", doc.Metadata["source"])
	assert.Equal(t, "test-author", doc.Metadata["author"])

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)

	// Explicitly use entity package to avoid import error
	_ = entity.Document{}
}

func TestGenericStreamWrapper_ParseStream_ExtractorError(t *testing.T) {
	// Create a test extractor function that returns an error
	extractor := func(ctx context.Context, r io.Reader) (string, error) {
		return "", errors.New("extractor error")
	}

	// Create a GenericStreamWrapper
	supportedTypes := []string{"txt"}
	wrapper := NewGenericStreamWrapper("test-parser", supportedTypes, extractor)

	// Create test reader
	reader := strings.NewReader("test content")

	// Test ParseStream with extractor error
	ctx := context.Background()
	docChan, err := wrapper.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// The channel should be closed without any documents
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestGenericStreamWrapper_ParseStream_ContextCanceled(t *testing.T) {
	// Create a test extractor function that waits for context cancellation
	extractor := func(ctx context.Context, r io.Reader) (string, error) {
		// Wait for context to be canceled
		<-ctx.Done()
		return "", ctx.Err()
	}

	// Create a GenericStreamWrapper
	supportedTypes := []string{"txt"}
	wrapper := NewGenericStreamWrapper("test-parser", supportedTypes, extractor)

	// Create test reader
	reader := strings.NewReader("test content")

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())

	// Test ParseStream
	docChan, err := wrapper.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Cancel the context
	cancel()

	// The channel should be closed without any documents
	_, ok := <-docChan
	assert.False(t, ok)
}

func TestGenericStreamWrapper_ParseStream_NoSourceMetadata(t *testing.T) {
	// Create a test extractor function
	extractor := func(ctx context.Context, r io.Reader) (string, error) {
		return "Test content", nil
	}

	// Create a GenericStreamWrapper
	supportedTypes := []string{"txt"}
	wrapper := NewGenericStreamWrapper("test-parser", supportedTypes, extractor)

	// Create test reader
	reader := strings.NewReader("Test content")

	// Test ParseStream without source metadata
	ctx := context.Background()
	docChan, err := wrapper.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	// Read from the channel
	doc := <-docChan
	assert.NotNil(t, doc)
	assert.Equal(t, "unknown", doc.Source)

	// The channel should be closed
	_, ok := <-docChan
	assert.False(t, ok)
}
