package javacode

import (
	"context"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ dataprep.Parser = (*JavacodeStreamParser)(nil)

// JavacodeStreamParser implements the dataprep.Parser interface for Java code
// It wraps the legacy Parser to provide streaming capability

type JavacodeStreamParser struct {
	legacyParser *Parser
}

// NewJavacodeStreamParser creates a new Java code stream parser
func NewJavacodeStreamParser() *JavacodeStreamParser {
	return &JavacodeStreamParser{
		legacyParser: NewParser(),
	}
}

// GetSupportedTypes returns the supported file formats
func (p *JavacodeStreamParser) GetSupportedTypes() []string {
	return p.legacyParser.SupportedFormats()
}

// ParseStream implements the dataprep.Parser interface
func (p *JavacodeStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "JavacodeStreamParser"
	docMeta["type"] = "javacode"

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
func (p *JavacodeStreamParser) SetChunkSize(size int) {
	p.legacyParser.SetChunkSize(size)
}

// SetChunkOverlap sets the chunk overlap
func (p *JavacodeStreamParser) SetChunkOverlap(overlap int) {
	p.legacyParser.SetChunkOverlap(overlap)
}

// SetExtractMethods sets whether to extract methods
func (p *JavacodeStreamParser) SetExtractMethods(extract bool) {
	p.legacyParser.SetExtractMethods(extract)
}

// SetExtractClasses sets whether to extract classes
func (p *JavacodeStreamParser) SetExtractClasses(extract bool) {
	p.legacyParser.SetExtractClasses(extract)
}

// SetExtractComments sets whether to extract comments
func (p *JavacodeStreamParser) SetExtractComments(extract bool) {
	p.legacyParser.SetExtractComments(extract)
}
