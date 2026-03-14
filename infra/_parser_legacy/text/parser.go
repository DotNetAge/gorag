package text

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/google/uuid"
)

// Parser implements a simple text parser
type Parser struct {
	chunkSize    int
	chunkOverlap int
}

// NewParser creates a new text parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// Parse parses text into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]model.Chunk, error) {
	var chunks []model.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk model.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses text and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(model.Chunk) error) error {
	// Read all content first for accurate chunking
	content, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	text := string(content)
	var position int

	// Split text into chunks with overlap
	for i := 0; i < len(text); i += p.chunkSize - p.chunkOverlap {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			end := i + p.chunkSize
			if end > len(text) {
				end = len(text)
			}

			chunkText := text[i:end]

			// 获取文件路径
			filePath := ""
			if path, ok := ctx.Value("file_path").(string); ok {
				filePath = path
			}

			// Create chunk
			chunk := model.Chunk{
				ID:      uuid.New().String(),
				Content: strings.TrimSpace(chunkText),
				Metadata: map[string]string{
					"type":      "text",
					"position":  fmt.Sprintf("%d", position),
					"file_path": filePath,
				},
			}

			// Call callback
			if err := callback(chunk); err != nil {
				return err
			}

			position++

			// If we've reached the end, break
			if end >= len(text) {
				break
			}
		}
	}

	// Handle empty text
	if position == 0 {
		// 获取文件路径
		filePath := ""
		if path, ok := ctx.Value("file_path").(string); ok {
			filePath = path
		}

		chunk := model.Chunk{
			ID:      uuid.New().String(),
			Content: "",
			Metadata: map[string]string{
				"type":      "text",
				"position":  "0",
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
	return []string{".txt", ".md"}
}
