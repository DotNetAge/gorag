package chunker

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

func TestDefaultCharacterChunker(t *testing.T) {
	chunker := DefaultCharacterChunker()

	assert.NotNil(t, chunker)
	assert.Equal(t, 1000, chunker.ChunkSize)
	assert.Equal(t, 150, chunker.ChunkOverlap)
	assert.Equal(t, []string{"\n\n", "\n", " ", ""}, chunker.Separators)
}

func TestNewCharacterChunker(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		overlap  int
		wantSize int
	}{
		{"default params", 1000, 150, 1000},
		{"small chunks", 500, 50, 500},
		{"large chunks", 2000, 200, 2000},
		{"zero overlap", 1000, 0, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewCharacterChunker(tt.size, tt.overlap)

			assert.Equal(t, tt.wantSize, chunker.ChunkSize)
			assert.Equal(t, tt.overlap, chunker.ChunkOverlap)
			assert.Equal(t, []string{"\n\n", "\n", " ", ""}, chunker.Separators)
		})
	}
}

func TestCharacterChunker_chunkText(t *testing.T) {
	tests := []struct {
		name         string
		chunker      *CharacterChunker
		text         string
		wantChunks   int
		wantFirstLen int
	}{
		{
			name:         "empty text",
			chunker:      NewCharacterChunker(100, 20),
			text:         "",
			wantChunks:   0,
			wantFirstLen: 0,
		},
		{
			name:         "text smaller than chunk size",
			chunker:      NewCharacterChunker(100, 20),
			text:         "Short text",
			wantChunks:   1,
			wantFirstLen: 10,
		},
		{
			name:         "text exactly one chunk",
			chunker:      NewCharacterChunker(10, 0),
			text:         "1234567890",
			wantChunks:   1,
			wantFirstLen: 10,
		},
		{
			name:         "text with overlap",
			chunker:      NewCharacterChunker(10, 2),
			text:         "12345678901234567890",
			wantChunks:   3, // [1-10], [9-18], [17-20]
			wantFirstLen: 10,
		},
		{
			name:         "unicode text",
			chunker:      NewCharacterChunker(5, 1),
			text:         "你好世界",
			wantChunks:   2,
			wantFirstLen: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := tt.chunker.chunkText(tt.text)

			assert.NoError(t, err)
			assert.Len(t, chunks, tt.wantChunks)

			if tt.wantChunks > 0 {
				assert.Equal(t, tt.wantFirstLen, len([]rune(chunks[0])))
			}
		})
	}
}

func TestCharacterChunker_Chunk(t *testing.T) {
	t.Run("basic document chunking", func(t *testing.T) {
		chunker := NewCharacterChunker(50, 10)
		doc := core.NewDocument(
			"doc-1",
			"This is a test document with enough content to create multiple chunks. "+
				"It should be split into several pieces based on the chunk size and overlap settings.",
			"/path/to/doc.txt",
			"text/plain",
			map[string]any{"author": "test"},
		)

		ctx := context.Background()
		chunks, err := chunker.Chunk(ctx, doc)

		assert.NoError(t, err)
		assert.NotEmpty(t, chunks)

		// Verify chunk properties
		for i, chunk := range chunks {
			assert.NotEmpty(t, chunk.ID)
			assert.Equal(t, "doc-1", chunk.DocumentID)
			assert.NotEmpty(t, chunk.Content)
			assert.Equal(t, "test", chunk.Metadata["author"])
			assert.False(t, chunk.CreatedAt.IsZero())

			// Verify index tracking
			if i > 0 {
				assert.Greater(t, chunk.StartIndex, chunks[i-1].StartIndex)
			}
		}
	})

	t.Run("empty document content", func(t *testing.T) {
		chunker := NewCharacterChunker(100, 20)
		doc := core.NewDocument("doc-2", "", "/path", "text/plain", nil)

		ctx := context.Background()
		chunks, err := chunker.Chunk(ctx, doc)

		assert.NoError(t, err)
		assert.Empty(t, chunks)
	})

	t.Run("single chunk document", func(t *testing.T) {
		chunker := NewCharacterChunker(1000, 100)
		doc := core.NewDocument("doc-3", "Short content", "/path", "text/plain", map[string]any{
			"key": "value",
		})

		ctx := context.Background()
		chunks, err := chunker.Chunk(ctx, doc)

		assert.NoError(t, err)
		assert.Len(t, chunks, 1)
		assert.Equal(t, "Short content", chunks[0].Content)
		assert.Equal(t, "value", chunks[0].Metadata["key"])
	})

	t.Run("metadata inheritance", func(t *testing.T) {
		chunker := NewCharacterChunker(20, 5)
		doc := core.NewDocument("doc-4", "This is a longer text to test metadata inheritance across multiple chunks", "/path", "text/plain", map[string]any{
			"source": "test",
			"page":   1,
			"tags":   []string{"tag1", "tag2"},
		})

		ctx := context.Background()
		chunks, err := chunker.Chunk(ctx, doc)

		assert.NoError(t, err)
		assert.NotEmpty(t, chunks)

		// All chunks should inherit metadata
		for _, chunk := range chunks {
			assert.Equal(t, "test", chunk.Metadata["source"])
			assert.Equal(t, 1, chunk.Metadata["page"])
			assert.Equal(t, []string{"tag1", "tag2"}, chunk.Metadata["tags"])
		}
	})

	t.Run("trimmed whitespace", func(t *testing.T) {
		chunker := NewCharacterChunker(50, 10)
		doc := core.NewDocument("doc-5", "  Text with leading and trailing spaces  ", "/path", "text/plain", nil)

		ctx := context.Background()
		chunks, err := chunker.Chunk(ctx, doc)

		assert.NoError(t, err)
		assert.NotEmpty(t, chunks)
		// Content should be trimmed
		assert.Equal(t, "Text with leading and trailing spaces", chunks[0].Content)
	})

	t.Run("context cancellation", func(t *testing.T) {
		chunker := NewCharacterChunker(100, 20)
		doc := core.NewDocument("doc-6", "Test content", "/path", "text/plain", nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Chunk should still work as it doesn't use context for I/O
		chunks, err := chunker.Chunk(ctx, doc)
		assert.NoError(t, err)
		assert.Len(t, chunks, 1)
	})
}

func TestCharacterChunker_InterfaceCompliance(t *testing.T) {
	// Verify that CharacterChunker implements core.Chunker interface
	var _ core.Chunker = (*CharacterChunker)(nil)

	chunker := NewCharacterChunker(100, 20)
	assert.Implements(t, (*core.Chunker)(nil), chunker)
}
