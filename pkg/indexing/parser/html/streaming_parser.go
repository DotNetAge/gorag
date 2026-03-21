package html

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
var _ core.Parser = (*HtmlStreamParser)(nil)

// HtmlStreamParser implements the core.Parser interface for HTML
// It wraps the legacy Parser to provide streaming capability

type HtmlStreamParser struct {
	legacyParser *Parser
}

// DefaultHtmlStreamParser creates a new HTML stream parser
func DefaultHtmlStreamParser() *HtmlStreamParser {
	return &HtmlStreamParser{
		legacyParser: DefaultParser(),
	}
}

// GetSupportedTypes returns the supported file formats
func (p *HtmlStreamParser) GetSupportedTypes() []string {
	return p.legacyParser.SupportedFormats()
}

// ParseStream implements the core.Parser interface
func (p *HtmlStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	outChan := make(chan *core.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "HtmlStreamParser"
	docMeta["type"] = "html"

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
func (p *HtmlStreamParser) SetChunkSize(size int) {
	p.legacyParser.SetChunkSize(size)
}

// SetChunkOverlap sets the chunk overlap
func (p *HtmlStreamParser) SetChunkOverlap(overlap int) {
	p.legacyParser.SetChunkOverlap(overlap)
}

// SetCleanScripts sets whether to remove <script> tags
func (p *HtmlStreamParser) SetCleanScripts(clean bool) {
	p.legacyParser.SetCleanScripts(clean)
}

// SetCleanStyles sets whether to remove <style> tags
func (p *HtmlStreamParser) SetCleanStyles(clean bool) {
	p.legacyParser.SetCleanStyles(clean)
}

// SetExtractLinks sets whether to extract links
func (p *HtmlStreamParser) SetExtractLinks(extract bool) {
	p.legacyParser.SetExtractLinks(extract)
}


// Parse implements core.Parser interface.
func (p *HtmlStreamParser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docChan, err := p.ParseStream(ctx, bytes.NewReader(content), metadata)
	if err != nil {
		return nil, err
	}
	for doc := range docChan {
		return doc, nil
	}
	return nil, fmt.Errorf("no document produced")
}

func (p *HtmlStreamParser) Supports(contentType string) bool { return true }
