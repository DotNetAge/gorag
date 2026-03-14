package json

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/google/uuid"
)

// Parser implements a JSON parser
type Parser struct {
	chunkSize    int
	chunkOverlap int
}

// NewParser creates a new JSON parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// SetChunkSize sets the chunk size
func (p *Parser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *Parser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// Parse parses JSON into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]model.Chunk, error) {
	var chunks []model.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk model.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses JSON and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(model.Chunk) error) error {
	decoder := json.NewDecoder(r)

	// Read opening token
	token, err := decoder.Token()
	if err != nil {
		return err
	}

	var buffer strings.Builder
	var position int

	// Process JSON tokens
	for decoder.More() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Encode token back to JSON string
			tokenBytes, err := json.Marshal(token)
			if err != nil {
				return err
			}
			buffer.WriteString(string(tokenBytes))

			// Check if we have enough content for a chunk
			if buffer.Len() >= p.chunkSize {
				// Create chunk with overlap
				chunkText := buffer.String()
				if len(chunkText) > p.chunkSize {
					chunkText = chunkText[:p.chunkSize]
				}

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
						"type":      "json",
						"position":  fmt.Sprintf("%d", position),
						"file_path": filePath,
					},
				}

				// Call callback
				if err := callback(chunk); err != nil {
					return err
				}

				// Keep overlap for next chunk
				if p.chunkOverlap > 0 && buffer.Len() > p.chunkOverlap {
					remaining := buffer.String()[p.chunkSize-p.chunkOverlap:]
					buffer.Reset()
					buffer.WriteString(remaining)
				} else {
					buffer.Reset()
				}

				position++
			}

			// Read next token
			token, err = decoder.Token()
			if err != nil {
				return err
			}
		}
	}

	// Process last token
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return err
	}
	buffer.WriteString(string(tokenBytes))

	// Process remaining content
	if buffer.Len() > 0 {
		// 获取文件路径
		filePath := ""
		if path, ok := ctx.Value("file_path").(string); ok {
			filePath = path
		}

		chunk := model.Chunk{
			ID:      uuid.New().String(),
			Content: strings.TrimSpace(buffer.String()),
			Metadata: map[string]string{
				"type":      "json",
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
	return []string{".json"}
}
