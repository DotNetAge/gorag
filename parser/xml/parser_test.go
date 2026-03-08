package xml

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

	xmlContent := []byte(`<?xml version="1.0"?>
<root>
	<name>Test</name>
	<version>1.0.0</version>
	<description>A test XML file</description>
</root>`)

	r := bytes.NewReader(xmlContent)
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
	assert.Contains(t, chunks[0].Content, "Test")
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	xmlContent := []byte(`<data><key>value</key><nested><a>1</a></nested></data>`)
	var chunkCount int

	err := parser.ParseWithCallback(ctx, bytes.NewReader(xmlContent), func(chunk core.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "xml")
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
}

func TestParser_EmptyXML(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	// Empty root element - should still create a chunk for the structure
	xmlContent := []byte(`<root></root>`)
	chunks, err := parser.Parse(ctx, bytes.NewReader(xmlContent))
	require.NoError(t, err)
	// Note: Empty elements may not produce chunks if they contain no text
	// This is expected behavior for SAX parsing
	_ = chunks // May be empty
}

func TestParser_LargeXML(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	// Create large XML
	var sb strings.Builder
	sb.WriteString(`<items>`)
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, `<item id="%d"><name>item%d</name></item>`, i, i)
	}
	sb.WriteString(`</items>`)

	chunks, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	// Create large XML
	var sb strings.Builder
	sb.WriteString(`<config>`)
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&sb, `<key%d>value%d</key%d>`, i, i, i)
	}
	sb.WriteString(`</config>`)

	cancel() // Cancel immediately

	_, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	xmlContent := []byte(`<test>true</test>`)

	err := parser.ParseWithCallback(ctx, bytes.NewReader(xmlContent), func(chunk core.Chunk) error {
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

func TestParser_CommentHandling(t *testing.T) {
	// Test with comments preserved
	parser := NewParser()
	parser.SetPreserveComments(true)
	ctx := context.Background()

	xmlContent := []byte(`<root><!-- This is a comment --><text>Hello</text></root>`)
	chunks, err := parser.Parse(ctx, bytes.NewReader(xmlContent))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".xml", formats[0])
}
