package json

import (
	"bytes"
	"context"
	"testing"
	"time"

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

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".json", formats[0])
}
