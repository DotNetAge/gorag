package yaml

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/parser"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Parser implements a YAML parser
type Parser struct {
	chunkSize    int
	chunkOverlap int
}

// NewParser creates a new YAML parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// Parse parses YAML into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]parser.Chunk, error) {
	var data interface{}
	err := yaml.NewDecoder(r).Decode(&data)
	if err != nil {
		return nil, err
	}

	// Convert to YAML string
	content, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}

	text := string(content)
	chunks := p.splitText(text)

	result := make([]parser.Chunk, len(chunks))
	for i, chunk := range chunks {
		result[i] = parser.Chunk{
			ID:      uuid.New().String(),
			Content: chunk,
			Metadata: map[string]string{
				"type":     "yaml",
				"position": fmt.Sprintf("%d", i),
			},
		}
	}

	return result, nil
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".yaml", ".yml"}
}

// splitText splits text into chunks with overlap
func (p *Parser) splitText(text string) []string {
	var chunks []string

	// Handle empty text
	if len(text) == 0 {
		chunks = append(chunks, "")
		return chunks
	}

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
