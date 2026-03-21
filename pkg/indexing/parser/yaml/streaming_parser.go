package yaml

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"github.com/google/uuid"
)

// ensure interface implementation

// Parser implements a YAML parser
type Parser struct {
	chunkSize    int
	chunkOverlap int
}

// DefaultParser creates a new YAML parser
func DefaultParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// SetChunkSize sets the chunk size
func (p *Parser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *Parser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// ParseStream implements the core.Parser interface
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	docCh := make(chan *core.Document)

	// Create a safe copy of metadata for the documents
	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "yaml"
	
	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(docCh)

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024) // 10MB buffer for large lines

		buffer := make([]byte, 0, p.chunkSize*2) // Preallocate buffer
		var position int

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Bytes()
				buffer = append(buffer, line...)
				buffer = append(buffer, '\n')

				// Check if we have enough content for a chunk
				if len(buffer) >= p.chunkSize {
					// Create chunk with overlap
					chunkText := string(buffer[:p.chunkSize])

					// 获取文件路径
					filePath := ""
					if path, ok := ctx.Value("file_path").(string); ok {
						filePath = path
					}

					// Create document
					docMetaCopy := make(map[string]any)
					for k, v := range docMeta {
						docMetaCopy[k] = v
					}
					docMetaCopy["type"] = "yaml"
					docMetaCopy["position"] = fmt.Sprintf("%d", position)
					docMetaCopy["file_path"] = filePath

					doc := core.NewDocument(
						uuid.New().String(),
						strings.TrimSpace(chunkText),
						source,
						"application/yaml",
						docMetaCopy,
					)

					select {
					case <-ctx.Done():
						return
					case docCh <- doc:
					}

					// Keep overlap for next chunk
					if p.chunkOverlap > 0 && len(buffer) > p.chunkOverlap {
						remaining := buffer[p.chunkSize-p.chunkOverlap:]
						buffer = make([]byte, len(remaining))
						copy(buffer, remaining)
					} else {
						buffer = buffer[:0]
					}

					position++
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return
		}

		// Process remaining content
		if len(buffer) > 0 {
			// 获取文件路径
			filePath := ""
			if path, ok := ctx.Value("file_path").(string); ok {
				filePath = path
			}

			docMetaCopy := make(map[string]any)
			for k, v := range docMeta {
				docMetaCopy[k] = v
			}
			docMetaCopy["type"] = "yaml"
			docMetaCopy["position"] = fmt.Sprintf("%d", position)
			docMetaCopy["file_path"] = filePath

			doc := core.NewDocument(
				uuid.New().String(),
				strings.TrimSpace(string(buffer)),
				source,
				"application/yaml",
				docMetaCopy,
			)

			select {
			case <-ctx.Done():
				return
			case docCh <- doc:
			}
		}
	}()

	return docCh, nil
}

// GetSupportedTypes returns supported formats
func (p *Parser) GetSupportedTypes() []string {
	return []string{".yaml", ".yml"}
}

// Supports checks if the content type is supported
func (p *Parser) Supports(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return contentType == ".yaml" || contentType == ".yml" || contentType == "text/yaml" || contentType == "application/yaml" || contentType == "application/x-yaml"
}

// Parse implements the core.Parser interface
func (p *Parser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docChan, err := p.ParseStream(ctx, strings.NewReader(string(content)), metadata)
	if err != nil {
		return nil, err
	}

	var firstDoc *core.Document
	for doc := range docChan {
		if firstDoc == nil {
			firstDoc = doc
		}
	}

	if firstDoc == nil {
		return nil, fmt.Errorf("no document parsed")
	}

	return firstDoc, nil
}