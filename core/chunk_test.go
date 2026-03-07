package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChunk(t *testing.T) {
	// Test Chunk creation
	chunk := Chunk{
		ID:      "1",
		Content: "Test content",
		Metadata: map[string]string{
			"Source":    "test.txt",
			"Page":      "1",
			"Position":  "0",
			"Type":      "text",
			"CreatedAt": time.Now().String(),
		},
	}

	assert.Equal(t, "1", chunk.ID)
	assert.Equal(t, "Test content", chunk.Content)
	assert.Equal(t, "test.txt", chunk.Metadata["Source"])
	assert.Equal(t, "1", chunk.Metadata["Page"])
	assert.Equal(t, "0", chunk.Metadata["Position"])
	assert.Equal(t, "text", chunk.Metadata["Type"])
	assert.NotEmpty(t, chunk.Metadata["CreatedAt"])
}

func TestResult(t *testing.T) {
	// Test Result creation
	result := Result{
		Chunk: Chunk{
			ID:      "1",
			Content: "Test content",
			Metadata: map[string]string{
				"Source":    "test.txt",
				"Page":      "1",
				"Position":  "0",
				"Type":      "text",
				"CreatedAt": time.Now().String(),
			},
		},
		Score: 0.95,
	}

	assert.Equal(t, "1", result.ID)
	assert.Equal(t, "Test content", result.Content)
	assert.Equal(t, "test.txt", result.Metadata["Source"])
	assert.Equal(t, float32(0.95), result.Score)
}

func TestSearchOptions(t *testing.T) {
	// Test SearchOptions creation
	options := SearchOptions{
		TopK:     10,
		Filter:   map[string]interface{}{"type": "text"},
		MinScore: 0.7,
	}

	assert.Equal(t, 10, options.TopK)
	assert.Equal(t, "text", options.Filter["type"])
	assert.Equal(t, float32(0.7), options.MinScore)
}

func TestSource(t *testing.T) {
	// Test Source creation
	source := Source{
		Type:    "text",
		Path:    "test.txt",
		Content: "Test content",
		Reader:  nil,
	}

	assert.Equal(t, "text", source.Type)
	assert.Equal(t, "test.txt", source.Path)
	assert.Equal(t, "Test content", source.Content)
	assert.Nil(t, source.Reader)
}

func TestMetadata_Defaults(t *testing.T) {
	// Test Metadata with default values
	metadata := map[string]string{}

	assert.Empty(t, metadata["Source"])
	assert.Empty(t, metadata["Page"])
	assert.Empty(t, metadata["Position"])
	assert.Empty(t, metadata["Type"])
	assert.Empty(t, metadata["CreatedAt"])
}

func TestSearchOptions_Defaults(t *testing.T) {
	// Test SearchOptions with default values
	options := SearchOptions{}

	assert.Zero(t, options.TopK)
	assert.Nil(t, options.Filter)
	assert.Zero(t, options.MinScore)
}
