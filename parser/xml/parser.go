package xml

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
)

// Parser implements an XML parser using SAX-style parsing
type Parser struct {
	chunkSize        int
	chunkOverlap     int
	preserveComments bool
}

// NewParser creates a new XML parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:        500,
		chunkOverlap:     50,
		preserveComments: false,
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

// SetPreserveComments sets whether to preserve XML comments
func (p *Parser) SetPreserveComments(preserve bool) {
	p.preserveComments = preserve
}

// Parse parses XML into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses XML and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	decoder := xml.NewDecoder(r)

	var buffer strings.Builder
	var position int
	var currentDepth int
	var skipElement bool

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		token, err := decoder.Token()
		if err == io.EOF {
			// Process remaining content
			if buffer.Len() > 0 {
				chunk := p.createChunk(buffer.String(), position)
				if err := callback(chunk); err != nil {
					return err
				}
			}
			return nil
		}

		if err != nil {
			return fmt.Errorf("XML parsing error: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			currentDepth++
			// Check if we should skip this element (e.g., comments)
			if t.Name.Local == "comment" && !p.preserveComments {
				skipElement = true
			}

		case xml.EndElement:
			if currentDepth > 0 {
				currentDepth--
			}
			if t.Name.Local == "comment" {
				skipElement = false
			}

		case xml.CharData:
			// Skip whitespace-only text at root level
			text := strings.TrimSpace(string(t))
			if text == "" || skipElement {
				continue
			}

			buffer.WriteString(text)
			buffer.WriteString(" ")

			// Check if we have enough content for a chunk
			if buffer.Len() >= p.chunkSize {
				chunkText := strings.TrimSpace(buffer.String())
				chunk := p.createChunk(chunkText, position)

				if err := callback(chunk); err != nil {
					return err
				}

				// Keep overlap for next chunk
				if p.chunkOverlap > 0 && buffer.Len() > p.chunkOverlap {
					remaining := buffer.String()[len(buffer.String())-p.chunkOverlap:]
					buffer.Reset()
					buffer.WriteString(remaining)
				} else {
					buffer.Reset()
				}

				position++
			}

		case xml.Comment:
			if p.preserveComments {
				comment := string(t)
				buffer.WriteString(comment)
				buffer.WriteString(" ")
			}
			// Otherwise skip comments
		}
	}
}

// createChunk creates a new chunk with metadata
func (p *Parser) createChunk(content string, position int) core.Chunk {
	return core.Chunk{
		ID:      uuid.New().String(),
		Content: strings.TrimSpace(content),
		Metadata: map[string]string{
			"type":     "xml",
			"position": fmt.Sprintf("%d", position),
		},
	}
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".xml"}
}
