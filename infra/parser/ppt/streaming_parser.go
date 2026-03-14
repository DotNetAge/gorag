package ppt

import (
	"context"
	"fmt"
	"io"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
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

// ParseStream implements the dataprep.Parser interface
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	docCh := make(chan *entity.Document)

	go func() {
		defer close(docCh)

		// Calculate file size by reading through the reader
		var size int64
		buf := make([]byte, 4096)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := r.Read(buf)
				if err == io.EOF {
					goto endOfFile
				}
				if err != nil {
					return
				}
				size += int64(n)
			}
		}

	endOfFile:

		// For now, we'll just create a simple document with the file size
		// In a real implementation, we would parse the PPT file structure
		// and extract text content from slides
		text := fmt.Sprintf("PPT file with size: %d bytes\n", size)
		text += "Note: PPT parsing is currently basic. In a future version, we'll extract text from slides."

		// Create a single document
		doc := &entity.Document{
			ID:      uuid.New().String(),
			Content: text,
			Metadata: map[string]any{
				"type":     "ppt",
				"position": "0",
				"size":     fmt.Sprintf("%d", size),
				"parser":   "ppt",
			},
		}

		select {
		case <-ctx.Done():
			return
		case docCh <- doc:
		}
	}()

	return docCh, nil
}

// GetSupportedTypes returns supported formats
func (p *Parser) GetSupportedTypes() []string {
	return []string{".pptx", ".ppt"}
}
