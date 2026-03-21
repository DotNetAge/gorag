package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

// Parser implements a high-quality PDF document parser using ledongthuc/pdf.
type Parser struct {
	chunkSize    int // characters per document chunk in the stream
}

// DefaultParser creates a new PDF parser instance.
func DefaultParser() *Parser {
	return &Parser{
		chunkSize: 2000, // Reasonable text block size before streaming
	}
}

// GetSupportedTypes returns the file extensions this parser supports.
func (p *Parser) GetSupportedTypes() []string {
	return []string{".pdf"}
}

// ParseStream reads from an io.Reader (which must be seekable or fully read) 
// and streams Document objects representing pages or logical sections.
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	docCh := make(chan *core.Document, 1)

	// PDF parsing usually requires a ReaderAt or seeking. 
	// For maximum compatibility in a streaming API, we read to a buffer if not seekable.
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF input: %w", err)
	}

	readerAt := bytes.NewReader(data)
	pdfReader, err := pdf.NewReader(readerAt, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF reader: %w", err)
	}

	go func() {
		defer close(docCh)

		source := "unknown"
		if s, ok := metadata["source"].(string); ok {
			source = s
		}

		numPages := pdfReader.NumPage()
		for i := 1; i <= numPages; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				page := pdfReader.Page(i)
				if page.V.IsNull() {
					continue
				}

				content, err := page.GetPlainText(nil)
				if err != nil {
					continue // Log or skip malformed pages
				}

				content = strings.TrimSpace(content)
				if content == "" {
					continue
				}

				// Combine existing metadata with page-specific info
				docMeta := make(map[string]any)
				for k, v := range metadata {
					docMeta[k] = v
				}
				docMeta["page_number"] = i
				docMeta["total_pages"] = numPages
				docMeta["parser"] = "pdf-ledongthuc"

				doc := core.NewDocument(
					uuid.New().String(),
					content,
					source,
					"application/pdf",
					docMeta,
				)

				select {
				case <-ctx.Done():
					return
				case docCh <- doc:
				}
			}
		}
	}()

	return docCh, nil
}

// Parse is the legacy non-streaming interface wrapper.
func (p *Parser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docCh, err := p.ParseStream(ctx, bytes.NewReader(content), metadata)
	if err != nil {
		return nil, err
	}
	// Return first page as primary document
	for doc := range docCh {
		return doc, nil
	}
	return nil, fmt.Errorf("no content extracted from PDF")
}

func (p *Parser) Supports(contentType string) bool {
	return contentType == "application/pdf" || contentType == ".pdf"
}
