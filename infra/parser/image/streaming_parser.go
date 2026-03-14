package image

import (
	"context"
	"io"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ dataprep.Parser = (*ImageStreamParser)(nil)

// ImageStreamParser implements the dataprep.Parser interface for images
// It wraps the legacy Parser to provide streaming capability

type ImageStreamParser struct {
	legacyParser *Parser
}

// NewImageStreamParser creates a new image stream parser
func NewImageStreamParser() *ImageStreamParser {
	return &ImageStreamParser{
		legacyParser: New(),
	}
}

// GetSupportedTypes returns the supported file formats
func (p *ImageStreamParser) GetSupportedTypes() []string {
	return p.legacyParser.SupportedFormats()
}

// ParseStream implements the dataprep.Parser interface
func (p *ImageStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "ImageStreamParser"
	docMeta["type"] = "image"

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

		// Convert chunks to documents
		for _, chunk := range chunks {
			// Check context
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Get media type from metadata
			mediaType := "image"
			if mt, ok := chunk.Metadata["media_type"].(string); ok {
				mediaType = mt
			}

			// Create document with combined metadata
			documentMeta := make(map[string]any)
			// Add original chunk metadata
			for k, v := range chunk.Metadata {
				documentMeta[k] = v
			}
			// Add parser metadata
			for k, v := range docMeta {
				documentMeta[k] = v
			}

			doc := entity.NewDocument(
				uuid.New().String(),
				chunk.Content,
				source,
				mediaType,
				documentMeta,
			)

			outChan <- doc
		}
	}()

	return outChan, nil
}
