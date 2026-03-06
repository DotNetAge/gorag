package image

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	// Create a simple image content (PNG header)
	imageContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	parser := New()
	ctx := context.Background()

	chunks, err := parser.Parse(ctx, strings.NewReader(string(imageContent)))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(chunks), 1)
	assert.Equal(t, "[Image content]", chunks[0].Content)
	assert.Equal(t, "image/png", chunks[0].MediaType)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := New()
	formats := parser.SupportedFormats()
	assert.Contains(t, formats, ".jpg")
	assert.Contains(t, formats, ".jpeg")
	assert.Contains(t, formats, ".png")
	assert.Contains(t, formats, ".gif")
	assert.Contains(t, formats, ".webp")
}
