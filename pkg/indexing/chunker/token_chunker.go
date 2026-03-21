package chunker

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
	"github.com/pkoukk/tiktoken-go"
)

var _ core.Chunker = (*TokenChunker)(nil)

// TokenChunker chunks text based on Token count instead of character count.
// This ensures that generated chunks strictly adhere to LLM/Embedding context limits.
type TokenChunker struct {
	ChunkSize    int
	ChunkOverlap int
	Encoding     *tiktoken.Tiktoken
}

// DefaultTokenChunker returns a TokenChunker with optimal default parameters
// (Size: 500, Overlap: 50, Model: cl100k_base). Ideal for OpenAI models.
func DefaultTokenChunker() (*TokenChunker, error) {
	return NewTokenChunker(500, 50, "cl100k_base")
}

// NewTokenChunker creates a new TokenChunker using the specified encoding model.
// Common models: "cl100k_base" (for text-embedding-3, gpt-4), "cl100k_base" (gpt-3.5)
func NewTokenChunker(size, overlap int, model string) (*TokenChunker, error) {
	if model == "" {
		model = "cl100k_base" // Default to the most common OpenAI encoding
	}

	tkm, err := tiktoken.GetEncoding(model)
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding for model %s: %w", model, err)
	}

	return &TokenChunker{
		ChunkSize:    size,
		ChunkOverlap: overlap,
		Encoding:     tkm,
	}, nil
}

// chunkTokens provides raw token splitting logic
func (c *TokenChunker) chunkTokens(text string) ([]string, error) {
	if text == "" {
		return []string{}, nil
	}

	tokens := c.Encoding.Encode(text, nil, nil)
	var chunks []string

	if len(tokens) == 0 {
		return chunks, nil
	}

	step := c.ChunkSize - c.ChunkOverlap
	if step <= 0 {
		step = c.ChunkSize // Prevent infinite loop if overlap >= size
	}

	for i := 0; i < len(tokens); i += step {
		end := i + c.ChunkSize
		if end > len(tokens) {
			end = len(tokens)
		}

		// Decode back to string
		chunkText := c.Encoding.Decode(tokens[i:end])
		chunks = append(chunks, chunkText)

		if end == len(tokens) {
			break
		}
	}

	return chunks, nil
}

// Chunk satisfies the core.Chunker pipeline interface
func (c *TokenChunker) Chunk(ctx context.Context, doc *core.Document) ([]*core.Chunk, error) {
	texts, err := c.chunkTokens(doc.Content)
	if err != nil {
		return nil, err
	}

	var chunks []*core.Chunk

	for i, text := range texts {
		// Inherit document metadata cleanly
		metadata := make(map[string]any)
		for k, v := range doc.Metadata {
			metadata[k] = v
		}

		// Note: Accurate StartIndex/EndIndex based on runes requires a complex mapping from tokens back to string positions.
		// For simplicity and efficiency, we store the chunk index.
		metadata["chunk_index"] = i

		chunk := core.NewChunk(
			uuid.New().String(),
			doc.ID,
			strings.TrimSpace(text),
			0, // We omit precise rune indices for token chunking to keep performance high
			0,
			metadata,
		)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}
