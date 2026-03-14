package ppt

import (
	"context"
	"fmt"
	"io"

	"github.com/DotNetAge/gorag/domain/model"
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
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]model.Chunk, error) {
	var chunks []model.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk model.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses PPT and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(model.Chunk) error) error {
	// Calculate file size by reading through the reader
	var size int64
	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := r.Read(buf)
			if err == io.EOF {
				goto endOfFile
			}
			if err != nil {
				return fmt.Errorf("failed to read PPT file: %w", err)
			}
			size += int64(n)
		}
	}

endOfFile:

	// For now, we'll just create a simple chunk with the file size
	// In a real implementation, we would parse the PPT file structure
	// and extract text content from slides
	text := fmt.Sprintf("PPT file with size: %d bytes\n", size)
	text += "Note: PPT parsing is currently basic. In a future version, we'll extract text from slides."

	// Create a single chunk
	chunk := model.Chunk{
		ID:      uuid.New().String(),
		Content: text,
		Metadata: map[string]string{
			"type":     "ppt",
			"position": "0",
			"size":     fmt.Sprintf("%d", size),
		},
	}

	// Call callback
	if err := callback(chunk); err != nil {
		return err
	}

	return nil
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".pptx", ".ppt"}
}
