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
	// Parse HTML document
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract text from HTML
	var buf strings.Builder
	p.extractText(doc, &buf)

	text := buf.String()
	chunks := p.splitText(text)

	result := make([]parser.Chunk, len(chunks))
	for i, chunk := range chunks {
		result[i] = parser.Chunk{
			ID:      uuid.New().String(),
			Content: chunk,
			Metadata: map[string]string{
				"type":     "html",
				"position": fmt.Sprintf("%d", i),
			},
		}
	}

	return result, nil
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".html", ".htm"}
}

// splitText splits text into chunks with overlap
func (p *Parser) splitText(text string) []string {
	var chunks []string

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

// extractText extracts text from HTML node
func (p *Parser) extractText(n *html.Node, buf *strings.Builder) {
	if n.Type == html.TextNode {
		buf.WriteString(n.Data)
	} else if n.Type == html.ElementNode && n.Data != "script" && n.Data != "style" {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			p.extractText(c, buf)
		}
	}
}
