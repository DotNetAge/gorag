package ppt

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/parser"
	"github.com/google/uuid"
)

// Parser implements a PPT parser
 type Parser struct {
	chunkSize    int
	chunkOverlap int
}

// NewParser creates a new PPT parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// Parse parses PPT file into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]parser.Chunk, error) {
	// Read the PPT file content
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read PPT file: %w", err)
	}

	// For now, we'll just create a simple chunk with the file size
	// In a real implementation, we would parse the PPT file structure
	// and extract text content from slides
	text := fmt.Sprintf("PPT file with size: %d bytes\n", len(content))
	text += "Note: PPT parsing is currently basic. In a future version, we'll extract text from slides."

	chunks := p.splitText(text)

	result := make([]parser.Chunk, len(chunks))
	for i, chunk := range chunks {
		result[i] = parser.Chunk{
			ID:      uuid.New().String(),
			Content: chunk,
			Metadata: map[string]string{
				"type":     "ppt",
				"position": fmt.Sprintf("%d", i),
			},
		}
	}

	return result, nil
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".pptx", ".ppt"}
}

// splitText splits text into chunks with overlap
func (p *Parser) splitText(text string) []string {
	var chunks []string

	// Handle empty text
	if len(text) == 0 {
		chunks = append(chunks, "")
		return chunks
	}

	for i := 0; i < len(text); i += p.chunkSize - p.chunkOverlap {
		end := i + p.chunkSize
		if end > len(text) {
			end = len(text)
		}

		chunk := text[i:end]
		chunks = append(chunks, strings.TrimSpace(chunk))

		if end >= len(text) {
			break
		}
	}

	return chunks
}
