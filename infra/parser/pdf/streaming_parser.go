package pdf

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/google/uuid"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	filePathKey contextKey = "file_path"
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

// ParseStream implements the dataprep.Parser interface
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	docCh := make(chan *entity.Document)

	go func() {
		defer close(docCh)

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
				return
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
					if path, ok := ctx.Value(filePathKey).(string); ok {
						filePath = path
					}

					// Create document
					doc := &entity.Document{
						ID:      uuid.New().String(),
						Content: strings.TrimSpace(chunkText),
						Metadata: map[string]any{
							"type":      "pdf",
							"position":  fmt.Sprintf("%d", position),
							"file_path": filePath,
							"parser":    "pdf",
						},
					}

					select {
					case <-ctx.Done():
						return
					case docCh <- doc:
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
			return
		}

		// Process remaining content
		if len(buffer) > 0 {
			// 获取文件路径
			filePath := ""
			if path, ok := ctx.Value(filePathKey).(string); ok {
				filePath = path
			}

			doc := &entity.Document{
				ID:      uuid.New().String(),
				Content: strings.TrimSpace(string(buffer)),
				Metadata: map[string]any{
					"type":      "pdf",
					"position":  fmt.Sprintf("%d", position),
					"file_path": filePath,
					"parser":    "pdf",
				},
			}

			select {
			case <-ctx.Done():
				return
			case docCh <- doc:
			}
		}
	}()

	return docCh, nil
}

// GetSupportedTypes returns supported formats
func (p *Parser) GetSupportedTypes() []string {
	return []string{".pdf"}
}
