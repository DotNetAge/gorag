package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDocument(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		content     string
		source      string
		contentType string
		metadata    map[string]any
	}{
		{
			name:        "basic document",
			id:          "doc-1",
			content:     "This is a test document",
			source:      "/path/to/file.txt",
			contentType: "text/plain",
			metadata:    map[string]any{"author": "test"},
		},
		{
			name:        "nil metadata",
			id:          "doc-2",
			content:     "Another document",
			source:      "/path/to/file2.txt",
			contentType: "text/markdown",
			metadata:    nil,
		},
		{
			name:        "empty content",
			id:          "doc-3",
			content:     "",
			source:      "",
			contentType: "",
			metadata:    map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := NewDocument(tt.id, tt.content, tt.source, tt.contentType, tt.metadata)

			assert.Equal(t, tt.id, doc.ID)
			assert.Equal(t, tt.content, doc.Content)
			assert.Equal(t, tt.source, doc.Source)
			assert.Equal(t, tt.contentType, doc.ContentType)
			assert.Equal(t, tt.metadata, doc.Metadata)
			assert.False(t, doc.CreatedAt.IsZero())
			assert.False(t, doc.UpdatedAt.IsZero())
			assert.True(t, doc.CreatedAt.Equal(doc.UpdatedAt))
		})
	}
}

func TestDocument_Update(t *testing.T) {
	doc := NewDocument("doc-1", "original content", "/path", "text/plain", map[string]any{"key": "value1"})
	originalCreatedAt := doc.CreatedAt

	// Wait a tiny bit to ensure time difference
	time.Sleep(1 * time.Millisecond)

	newMetadata := map[string]any{"key": "value2", "new_key": "new_value"}
	doc.Update("updated content", newMetadata)

	assert.Equal(t, "updated content", doc.Content)
	assert.Equal(t, newMetadata, doc.Metadata)
	assert.True(t, doc.UpdatedAt.After(originalCreatedAt))
	assert.Equal(t, originalCreatedAt, doc.CreatedAt) // CreatedAt should not change
}
