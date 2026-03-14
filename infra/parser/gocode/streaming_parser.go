package gocode

import (
	"context"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ dataprep.Parser = (*GocodeStreamParser)(nil)

// GocodeStreamParser implements the dataprep.Parser interface for Go code
// It wraps the legacy Parser to provide streaming capability

type GocodeStreamParser struct {
	legacyParser *Parser
}

// NewGocodeStreamParser creates a new Go code stream parser
func NewGocodeStreamParser() *GocodeStreamParser {
	return &GocodeStreamParser{
		legacyParser: NewParser(),
	}
}

// GetSupportedTypes returns the supported file formats
func (p *GocodeStreamParser) GetSupportedTypes() []string {
	return p.legacyParser.SupportedFormats()
}

// ParseStream implements the dataprep.Parser interface
func (p *GocodeStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "GocodeStreamParser"
	docMeta["type"] = "gocode"

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		// Check context before parsing
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Parse using the legacy parser
		chunks, err := p.legacyParser.Parse(ctx, r)
		if err != nil {
			return
		}

		// Convert chunks to a single document
		var contentBuilder strings.Builder
		for _, chunk := range chunks {
			contentBuilder.WriteString(chunk.Content)
			contentBuilder.WriteString("\n\n")
		}

		content := strings.TrimSpace(contentBuilder.String())

		// Check if content is empty
		if content == "" {
			return
		}

		// Check context again
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Create document
		doc := entity.NewDocument(
			uuid.New().String(),
			content,
			source,
			"text/plain",
			docMeta,
		)

		outChan <- doc
	}()

	return outChan, nil
}

// SetChunkSize sets the chunk size
func (p *GocodeStreamParser) SetChunkSize(size int) {
	p.legacyParser.SetChunkSize(size)
}

// SetChunkOverlap sets the chunk overlap
func (p *GocodeStreamParser) SetChunkOverlap(overlap int) {
	p.legacyParser.SetChunkOverlap(overlap)
}

// SetExtractFunctions enables or disables function extraction
func (p *GocodeStreamParser) SetExtractFunctions(enabled bool) {
	p.legacyParser.SetExtractFunctions(enabled)
}

// SetExtractTypes enables or disables type extraction
func (p *GocodeStreamParser) SetExtractTypes(enabled bool) {
	p.legacyParser.SetExtractTypes(enabled)
}

// SetExtractComments enables or disables comment extraction
func (p *GocodeStreamParser) SetExtractComments(enabled bool) {
	p.legacyParser.SetExtractComments(enabled)
}
