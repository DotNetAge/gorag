package xml

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
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

// ParseStream implements the core.Parser interface
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	docCh := make(chan *core.Document)

	go func() {
		defer close(docCh)

		decoder := xml.NewDecoder(r)

		var buffer strings.Builder
		var position int
		var currentDepth int
		var skipElement bool

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			token, err := decoder.Token()
			if err == io.EOF {
				// Process remaining content
				if buffer.Len() > 0 {
					doc := p.createDocument(buffer.String(), position)
					select {
					case <-ctx.Done():
						return
					case docCh <- doc:
					}
				}
				return
			}

			if err != nil {
				return
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
					doc := p.createDocument(chunkText, position)

					select {
					case <-ctx.Done():
						return
					case docCh <- doc:
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
	}()

	return docCh, nil
}

// createDocument creates a new document with metadata
func (p *Parser) createDocument(content string, position int) *core.Document {
	return &core.Document{
		ID:      uuid.New().String(),
		Content: strings.TrimSpace(content),
		Metadata: map[string]any{
			"type":     "xml",
			"position": fmt.Sprintf("%d", position),
			"parser":   "xml",
		},
	}
}

// GetSupportedTypes returns supported formats
func (p *Parser) GetSupportedTypes() []string {
	return []string{".xml"}
}

// Supports checks if the content type is supported
func (p *Parser) Supports(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return contentType == ".xml" || contentType == "text/xml" || contentType == "application/xml"
}

// Parse implements the core.Parser interface
func (p *Parser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docChan, err := p.ParseStream(ctx, strings.NewReader(string(content)), metadata)
	if err != nil {
		return nil, err
	}

	var firstDoc *core.Document
	for doc := range docChan {
		if firstDoc == nil {
			firstDoc = doc
		}
	}

	if firstDoc == nil {
		return nil, fmt.Errorf("no document parsed")
	}

	return firstDoc, nil
}
