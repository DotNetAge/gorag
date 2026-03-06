package ppt

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	// Create a simple PPT file content
	pptContent := "Dummy PPT content"

	parser := NewParser()
	ctx := context.Background()

	chunks, err := parser.Parse(ctx, strings.NewReader(pptContent))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Contains(t, formats, ".pptx")
	assert.Contains(t, formats, ".ppt")
}
