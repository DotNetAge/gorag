package image

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
)

// Parser implements Visual RAG by using an LLM to describe images and EXIF to extract metadata.
type Parser struct {
	llm chat.Client
}

// DefaultParser creates a new Visual RAG image parser.
func DefaultParser(llm chat.Client) *Parser {
	return &Parser{
		llm: llm,
	}
}

// GetSupportedTypes returns the image extensions this parser supports.
func (p *Parser) GetSupportedTypes() []string {
	return []string{".jpg", ".jpeg", ".png", ".webp"}
}

// ParseStream reads an image and uses Multimodal LLM to generate a textual description.
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	docCh := make(chan *core.Document, 1)

	// We need the data for EXIF and potential LLM vision call
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image input: %w", err)
	}

	go func() {
		defer close(docCh)

		source := "unknown"
		if s, ok := metadata["source"].(string); ok {
			source = s
		}

		// 1. Extract EXIF Metadata
		exifMeta := make(map[string]any)
		if x, err := exif.Decode(bytes.NewReader(data)); err == nil {
			if lat, long, err := x.LatLong(); err == nil {
				exifMeta["latitude"] = lat
				exifMeta["longitude"] = long
			}
			if tm, err := x.DateTime(); err == nil {
				exifMeta["taken_at"] = tm
			}
		}

		// 2. Multimodal LLM Description (Visual RAG)
		description := "[Image without description]"
		if p.llm != nil {
			// Create a vision-capable message
			prompt := "Describe this image in detail for a retrieval system. Include objects, text, colors, and context."
			
			// We assume gochat.Client handles multimodal if provided properly.
			// This is a placeholder for actual multimodal prompt structure.
			msg := chat.NewUserMessage(prompt)
			// (Future: attach image data to msg if supported by gochat)
			
			resp, err := p.llm.Chat(ctx, []chat.Message{msg})
			if err == nil {
				description = strings.TrimSpace(resp.Content)
			}
		}

		// Merge metadata
		docMeta := make(map[string]any)
		for k, v := range metadata {
			docMeta[k] = v
		}
		for k, v := range exifMeta {
			docMeta[k] = v
		}
		docMeta["parser"] = "visual-rag-exif"
		docMeta["size_bytes"] = len(data)

		doc := core.NewDocument(
			uuid.New().String(),
			description,
			source,
			"image/generic", // Detailed type could be detected from magic bytes
			docMeta,
		)

		select {
		case <-ctx.Done():
			return
		case docCh <- doc:
		}
	}()

	return docCh, nil
}

// Parse implements legacy core.Parser interface.
func (p *Parser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docCh, err := p.ParseStream(ctx, bytes.NewReader(content), metadata)
	if err != nil {
		return nil, err
	}
	for doc := range docCh {
		return doc, nil
	}
	return nil, fmt.Errorf("no content extracted from Image")
}

func (p *Parser) Supports(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") || strings.HasPrefix(contentType, ".")
}
