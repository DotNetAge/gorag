package base

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"io"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ Parser = (*GenericStreamWrapper)(nil)

// ContentExtractor is a simple function signature that takes an io.Reader and returns the full text.
// This is used for legacy parsers or file types that cannot be easily streamed (like Docx/PDF).
type ContentExtractor func(ctx context.Context, r io.Reader) (string, error)

// GenericStreamWrapper wraps a full-read extractor into a core.Parser (streaming interface).
// It's a bridge for legacy parsers.
type GenericStreamWrapper struct {
	extractor      ContentExtractor
	supportedTypes []string
	parserName     string
}

func NewGenericStreamWrapper(name string, types []string, extractor ContentExtractor) *GenericStreamWrapper {
	return &GenericStreamWrapper{
		extractor:      extractor,
		supportedTypes: types,
		parserName:     name,
	}
}

func (w *GenericStreamWrapper) GetSupportedTypes() []string {
	return w.supportedTypes
}

func (w *GenericStreamWrapper) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	outChan := make(chan *core.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = w.parserName

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		// Check context before heavy extraction
		select {
		case <-ctx.Done():
			return
		default:
		}

		content, err := w.extractor(ctx, r)
		if err != nil {
			// In a real system, you might want to return this error through an error channel,
			// but for now, we just close the output channel (terminating the stream).
			return
		}

		// Check context again after heavy extraction
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Since we extracted the full text, we yield it as a single Document.
		// If the text is massive, we could split it here, but that's what the Chunker is for.
		doc := core.NewDocument(
			uuid.New().String(),
			content,
			source,
			"text/plain", // Defaulting to plain text once extracted
			docMeta,
		)

		outChan <- doc
	}()

	return outChan, nil
}
