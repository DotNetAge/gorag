package json

import (
	"bytes"
	"context"
	"fmt"
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

	jsonContent := []byte(`{
		"name": "Test",
		"version": "1.0.0",
		"description": "A test JSON file"
	}`)

	r := bytes.NewReader(jsonContent)
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
	assert.Contains(t, chunks[0].Content, "name")
	assert.Contains(t, chunks[0].Content, "Test")
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	jsonContent := []byte(`{"key": "value", "nested": {"a": 1}}`)
	var chunkCount int

	err := parser.ParseWithCallback(ctx, bytes.NewReader(jsonContent), func(chunk core.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "json")
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
}

func TestParser_EmptyJSON(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	// Empty object
	jsonContent := []byte(`{}`)
	chunks, err := parser.Parse(ctx, bytes.NewReader(jsonContent))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_LargeArray(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	// Create large array
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id":%d,"name":"item%d"}`, i, i)
	}
	sb.WriteString("]")

	chunks, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	// Create large JSON
	var sb strings.Builder
	sb.WriteString("{")
	for i := 0; i < 1000; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `"key%d":"value%d"`, i, i)
	}
	sb.WriteString("}")

	cancel() // Cancel immediately

	_, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	jsonContent := []byte(`{"test": true}`)

	err := parser.ParseWithCallback(ctx, bytes.NewReader(jsonContent), func(chunk core.Chunk) error {
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

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".json", formats[0])
}
