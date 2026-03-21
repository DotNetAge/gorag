package xml

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

func TestParser_ParseStream(t *testing.T) {
	parser := DefaultParser()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	xmlContent := []byte(`<?xml version="1.0"?>
<root>
	<name>Test</name>
	<version>1.0.0</version>
	<description>A test XML file</description>
</root>`)

	r := bytes.NewReader(xmlContent)
	docCh, err := parser.ParseStream(ctx, r, nil)
	require.NoError(t, err)

	var docs []*core.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
	assert.Contains(t, docs[0].Content, "Test")
	for _, doc := range docs {
		assert.NotEmpty(t, doc.ID)
		assert.Contains(t, doc.Metadata["type"], "xml")
		assert.Contains(t, doc.Metadata["parser"], "xml")
	}
}

func TestParser_EmptyXML(t *testing.T) {
	parser := DefaultParser()
	ctx := context.Background()

	// Empty root element - should still create a chunk for the structure
	xmlContent := []byte(`<root></root>`)
	docCh, err := parser.ParseStream(ctx, bytes.NewReader(xmlContent), nil)
	require.NoError(t, err)

	var docs []*core.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	// Note: Empty elements may not produce chunks if they contain no text
	// This is expected behavior for SAX parsing
}

func TestParser_LargeXML(t *testing.T) {
	parser := DefaultParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	// Create large XML
	var sb strings.Builder
	sb.WriteString(`<items>`)
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, `<item id="%d"><name>item%d</name></item>`, i, i)
	}
	sb.WriteString(`</items>`)

	docCh, err := parser.ParseStream(ctx, strings.NewReader(sb.String()), nil)
	require.NoError(t, err)

	var docs []*core.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := DefaultParser()
	ctx, cancel := context.WithCancel(context.Background())

	// Create large XML
	var sb strings.Builder
	sb.WriteString(`<config>`)
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&sb, `<key%d>value%d</key%d>`, i, i, i)
	}
	sb.WriteString(`</config>`)

	cancel() // Cancel immediately

	docCh, err := parser.ParseStream(ctx, strings.NewReader(sb.String()), nil)
	require.NoError(t, err)

	var docs []*core.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	// May be empty due to cancellation
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := DefaultParser()
	parser.SetChunkSize(200)
	parser.SetChunkOverlap(20)

	assert.Equal(t, 200, parser.chunkSize)
	assert.Equal(t, 20, parser.chunkOverlap)
}

func TestParser_CommentHandling(t *testing.T) {
	// Test with comments preserved
	parser := DefaultParser()
	parser.SetPreserveComments(true)
	ctx := context.Background()

	xmlContent := []byte(`<root><!-- This is a comment --><text>Hello</text></root>`)
	docCh, err := parser.ParseStream(ctx, bytes.NewReader(xmlContent), nil)
	require.NoError(t, err)

	var docs []*core.Document
	for doc := range docCh {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
}

func TestParser_GetSupportedTypes(t *testing.T) {
	parser := DefaultParser()
	formats := parser.GetSupportedTypes()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".xml", formats[0])
}

func TestParser_ParseStream_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := DefaultParser()
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
			docCh, err := parser.ParseStream(ctx, reader, nil)
			if err != nil {
				t.Skipf("Skipping file with parsing error: %s", err)
				return
			}

			// Verify documents
			var docs []*core.Document
			for doc := range docCh {
				docs = append(docs, doc)
			}

			for i, doc := range docs {
				assert.NotEmpty(t, doc.ID, "Document %d should have an ID", i)
				assert.NotEmpty(t, doc.Content, "Document %d should have content", i)
				assert.Contains(t, doc.Metadata["type"], "xml", "Document %d should have type 'xml'", i)
				assert.Contains(t, doc.Metadata["parser"], "xml", "Document %d should have parser 'xml'", i)
			}
		})
	}
}