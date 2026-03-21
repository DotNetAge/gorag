package jscode

import (
	"fmt"
	"bytes"
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"io"
	"strings"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ core.Parser = (*JscodeStreamParser)(nil)

// JscodeStreamParser implements the core.Parser interface for JavaScript code
// It wraps the legacy Parser to provide streaming capability

type JscodeStreamParser struct {
	legacyParser *Parser
}

// DefaultJscodeStreamParser creates a new JavaScript code stream parser
func DefaultJscodeStreamParser() *JscodeStreamParser {
	return &JscodeStreamParser{
		legacyParser: DefaultParser(),
	}
}

// GetSupportedTypes returns the supported file formats
func (p *JscodeStreamParser) GetSupportedTypes() []string {
	return p.legacyParser.SupportedFormats()
}

// ParseStream implements the core.Parser interface
func (p *JscodeStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	outChan := make(chan *core.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "JscodeStreamParser"
	docMeta["type"] = "jscode"

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
		doc := core.NewDocument(
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
func (p *JscodeStreamParser) SetChunkSize(size int) {
	p.legacyParser.SetChunkSize(size)
}

// SetChunkOverlap sets the chunk overlap
func (p *JscodeStreamParser) SetChunkOverlap(overlap int) {
	p.legacyParser.SetChunkOverlap(overlap)
}

// SetExtractFunctions sets whether to extract functions
func (p *JscodeStreamParser) SetExtractFunctions(extract bool) {
	p.legacyParser.SetExtractFunctions(extract)
}

// SetExtractClasses sets whether to extract classes
func (p *JscodeStreamParser) SetExtractClasses(extract bool) {
	p.legacyParser.SetExtractClasses(extract)
}

// SetExtractComments sets whether to extract comments
func (p *JscodeStreamParser) SetExtractComments(extract bool) {
	p.legacyParser.SetExtractComments(extract)
}


// Parse implements core.Parser interface.
func (p *JscodeStreamParser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docChan, err := p.ParseStream(ctx, bytes.NewReader(content), metadata)
	if err != nil {
		return nil, err
	}
	for doc := range docChan {
		return doc, nil
	}
	return nil, fmt.Errorf("no document produced")
}

func (p *JscodeStreamParser) Supports(contentType string) bool { return true }
