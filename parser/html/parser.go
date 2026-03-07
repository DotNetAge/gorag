package html

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/parser"
	"github.com/google/uuid"
	"golang.org/x/net/html"
)

// Parser implements an HTML document parser
type Parser struct {
	chunkSize    int
	chunkOverlap int
}

// NewParser creates a new HTML parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// Parse parses HTML into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]parser.Chunk, error) {
	var chunks []parser.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk parser.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses HTML and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(parser.Chunk) error) error {
	// Create HTML tokenizer
	tokenizer := html.NewTokenizer(r)

	var buffer strings.Builder
	var position int

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			tokenType := tokenizer.Next()

			switch tokenType {
			case html.ErrorToken:
				// End of document
				if tokenizer.Err() != io.EOF {
					return tokenizer.Err()
				}
				// Process remaining content
				if buffer.Len() > 0 {
					chunk := parser.Chunk{
						ID:      uuid.New().String(),
						Content: strings.TrimSpace(buffer.String()),
						Metadata: map[string]string{
							"type":     "html",
							"position": fmt.Sprintf("%d", position),
						},
					}

					if err := callback(chunk); err != nil {
						return err
					}
				}
				return nil

			case html.TextToken:
				// Extract text content
				text := string(tokenizer.Text())
				buffer.WriteString(text)

				// Check if we have enough content for a chunk
				if buffer.Len() >= p.chunkSize {
					// Create chunk with overlap
					chunkText := buffer.String()
					if len(chunkText) > p.chunkSize {
						chunkText = chunkText[:p.chunkSize]
					}

					// Create chunk
					chunk := parser.Chunk{
						ID:      uuid.New().String(),
						Content: strings.TrimSpace(chunkText),
						Metadata: map[string]string{
							"type":     "html",
							"position": fmt.Sprintf("%d", position),
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

			// Ignore other token types
			default:
				// Do nothing
			}
		}
	}
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".html", ".htm"}
}

