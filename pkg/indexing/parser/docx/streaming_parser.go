package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
	"github.com/nguyenthenguyen/docx"
)

// Parser implements a high-quality DOCX document parser.
type Parser struct{}

// NewParser creates a new DOCX parser instance.
func NewParser() *Parser {
	return &Parser{}
}

// GetSupportedTypes returns the file extensions this parser supports.
func (p *Parser) GetSupportedTypes() []string {
	return []string{".docx"}
}

// ParseStream reads from an io.Reader (as a zip archive) and streams document contents.
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	docCh := make(chan *core.Document, 1)

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read DOCX input: %w", err)
	}

	readerAt := bytes.NewReader(data)
	zipReader, err := zip.NewReader(readerAt, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open DOCX zip: %w", err)
	}

	go func() {
		defer close(docCh)

		source := "unknown"
		if s, ok := metadata["source"].(string); ok {
			source = s
		}

		// bytes.NewReader implements io.ReaderAt
		d, err := docx.ReadDocxFromMemory(readerAt, int64(len(data)))
		if err != nil {
			return
		}
		defer d.Close()

		// Extract whole text
		content := d.Editable().GetContent()
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}

		// Docx metadata merging
		docMeta := make(map[string]any)
		for k, v := range metadata {
			docMeta[k] = v
		}
		docMeta["parser"] = "docx-native"
		docMeta["num_files"] = len(zipReader.File)

		doc := core.NewDocument(
			uuid.New().String(),
			content,
			source,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
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
	return nil, fmt.Errorf("no content extracted from DOCX")
}

func (p *Parser) Supports(contentType string) bool {
	return contentType == ".docx" || contentType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
}
