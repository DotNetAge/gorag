package chunker

import (
	"context"
	"strings"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ TextSplitter = (*CharacterSplitter)(nil)
var _ core.Chunker = (*CharacterSplitter)(nil)

// CharacterSplitter splits text recursively using a list of separators,
// similar to RecursiveCharacterTextSplitter in LangChain.
type CharacterSplitter struct {
	ChunkSize    int
	ChunkOverlap int
	Separators   []string
}

// NewCharacterSplitter creates a chunker splitting by runes and logical breaks
func NewCharacterSplitter(size, overlap int) *CharacterSplitter {
	return &CharacterSplitter{
		ChunkSize:    size,
		ChunkOverlap: overlap,
		Separators:   []string{"\n\n", "\n", " ", ""}, // Try paragraphs, then lines, words, chars
	}
}

// SplitText provides raw splitting logic
func (c *CharacterSplitter) SplitText(text string) ([]string, error) {
	// Simple implementation (for foundational completeness): fallback to hard rune limit.
	// A production-grade chunker would recursively split across 'c.Separators'.
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

// SplitDocument converts a document into interconnected core.Chunk units (similar to LlamaIndex Nodes).
func (c *CharacterSplitter) SplitDocument(ctx context.Context, doc *core.Document) ([]*core.Chunk, error) {
	return c.Chunk(ctx, doc)
}

// Chunk satisfies the core.Chunker pipeline interface
func (c *CharacterSplitter) Chunk(ctx context.Context, doc *core.Document) ([]*core.Chunk, error) {
	texts, err := c.SplitText(doc.Content)
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
