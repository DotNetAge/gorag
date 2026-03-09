package pdf

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
)

// Parser implements a PDF document parser
type Parser struct {
	chunkSize    int
	chunkOverlap int
}

// NewParser creates a new PDF parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// Parse parses PDF into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses PDF and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	// For simplicity, we'll skip the actual implementation for now
	// In a real implementation, you would use the pdfcpu library to parse PDF
	// and process it in a streaming manner

	// Simulate parsing with a simple scanner
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024) // 10MB buffer for large lines

	buffer := make([]byte, 0, p.chunkSize*2) // Preallocate buffer
	var position int

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line := scanner.Bytes()
			buffer = append(buffer, line...)
			buffer = append(buffer, '\n')

			// Check if we have enough content for a chunk
			if len(buffer) >= p.chunkSize {
				// Create chunk with overlap
				chunkText := string(buffer[:p.chunkSize])

				// 获取文件路径
				filePath := ""
				if path, ok := ctx.Value("file_path").(string); ok {
					filePath = path
				}
				
				// Create chunk
				chunk := core.Chunk{
					ID:      uuid.New().String(),
					Content: strings.TrimSpace(chunkText),
					Metadata: map[string]string{
						"type":      "pdf",
						"position":  fmt.Sprintf("%d", position),
						"file_path": filePath,
					},
				}

				// Call callback
				if err := callback(chunk); err != nil {
					return err
				}

				// Keep overlap for next chunk
				if p.chunkOverlap > 0 && len(buffer) > p.chunkOverlap {
					remaining := buffer[p.chunkSize-p.chunkOverlap:]
					buffer = make([]byte, len(remaining))
					copy(buffer, remaining)
				} else {
					buffer = buffer[:0]
				}

				position++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Process remaining content
	if len(buffer) > 0 {
		// 获取文件路径
		filePath := ""
		if path, ok := ctx.Value("file_path").(string); ok {
			filePath = path
		}
		
		chunk := core.Chunk{
			ID:      uuid.New().String(),
			Content: strings.TrimSpace(string(buffer)),
			Metadata: map[string]string{
				"type":      "pdf",
				"position":  fmt.Sprintf("%d", position),
				"file_path": filePath,
			},
		}

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".pdf"}
}

