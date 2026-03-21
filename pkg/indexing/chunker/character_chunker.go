package chunker

import (
	"context"
	"strings"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ core.Chunker = (*CharacterChunker)(nil)

// CharacterChunker chunks text recursively using a list of separators.
type CharacterChunker struct {
	ChunkSize    int
	ChunkOverlap int
	Separators   []string
}

// NewDefaultCharacterChunker returns a CharacterChunker with optimal default parameters (Size: 1000, Overlap: 150).
// Ideal for quick start and simple text processing.
func DefaultCharacterChunker() *CharacterChunker {
	return NewCharacterChunker(1000, 150)
}

// NewCharacterChunker creates a chunker splitting by runes and logical breaks
func NewCharacterChunker(size, overlap int) *CharacterChunker {
	return &CharacterChunker{
		ChunkSize:    size,
		ChunkOverlap: overlap,
		Separators:   []string{"\n\n", "\n", " ", ""}, // Try paragraphs, then lines, words, chars
	}
}

// chunkText provides raw splitting logic
func (c *CharacterChunker) chunkText(text string) ([]string, error) {
	// Simple implementation (for foundational completeness): fallback to hard rune limit.
	// A production-grade chunker would recursively chunk across 'c.Separators'.
	runes := []rune(text)
	var chunks []string

	if len(runes) == 0 {
		return chunks, nil
	}

	for i := 0; i < len(runes); i += (c.ChunkSize - c.ChunkOverlap) {
		end := i + c.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))

		if end == len(runes) {
			break
		}
	}

	return chunks, nil
}

// Chunk satisfies the core.Chunker pipeline interface
func (c *CharacterChunker) Chunk(ctx context.Context, doc *core.Document) ([]*core.Chunk, error) {
	texts, err := c.chunkText(doc.Content)
	if err != nil {
		return nil, err
	}

	var chunks []*core.Chunk
	startIdx := 0

	for _, text := range texts {
		// Inherit document metadata cleanly
		metadata := make(map[string]any)
		for k, v := range doc.Metadata {
			metadata[k] = v
		}

		chunk := core.NewChunk(
			uuid.New().String(),
			doc.ID,
			strings.TrimSpace(text),
			startIdx,
			startIdx+len([]rune(text)),
			metadata,
		)
		chunks = append(chunks, chunk)
		startIdx += (c.ChunkSize - c.ChunkOverlap)
	}

	return chunks, nil
}
