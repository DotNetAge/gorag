package json

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	filePathKey contextKey = "file_path"
)

// ensure interface implementation
var _ dataprep.Parser = (*JsonStreamParser)(nil)

// JsonStreamParser implements the dataprep.Parser interface for JSON files
type JsonStreamParser struct {
	chunkSize    int
	chunkOverlap int
}

// NewJsonStreamParser creates a new JSON stream parser
func NewJsonStreamParser() *JsonStreamParser {
	return &JsonStreamParser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// SetChunkSize sets the chunk size
func (p *JsonStreamParser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *JsonStreamParser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// GetSupportedTypes returns the supported formats
func (p *JsonStreamParser) GetSupportedTypes() []string {
	return []string{".json"}
}

// ParseStream parses JSON content from a reader and returns a channel of documents
func (p *JsonStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 10)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "JsonStreamParser"

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		decoder := json.NewDecoder(r)

		// Read opening token
		token, err := decoder.Token()
		if err != nil {
			return
		}

		var buffer strings.Builder
		var position int

		// Process JSON tokens
		for decoder.More() {
			select {
			case <-ctx.Done():
				return
			default:
				// Encode token back to JSON string
				tokenBytes, err := json.Marshal(token)
				if err != nil {
					return
				}
				buffer.WriteString(string(tokenBytes))

				// Check if we have enough content for a chunk
				if buffer.Len() >= p.chunkSize {
					// Create chunk with overlap
					chunkText := buffer.String()
					if len(chunkText) > p.chunkSize {
						chunkText = chunkText[:p.chunkSize]
					}

					// 获取文件路径
					filePath := ""
					if path, ok := ctx.Value(filePathKey).(string); ok {
						filePath = path
					}

					// Create document
					docMetaCopy := make(map[string]any)
					for k, v := range docMeta {
						docMetaCopy[k] = v
					}
					docMetaCopy["type"] = "json"
					docMetaCopy["position"] = fmt.Sprintf("%d", position)
					docMetaCopy["file_path"] = filePath

					doc := entity.NewDocument(
						uuid.New().String(),
						strings.TrimSpace(chunkText),
						source,
						"application/json",
						docMetaCopy,
					)

					select {
					case <-ctx.Done():
						return
					case outChan <- doc:
						// Keep overlap for next chunk
						if p.chunkOverlap > 0 && buffer.Len() > p.chunkOverlap {
							remaining := buffer.String()[p.chunkSize-p.chunkOverlap:]
							buffer.Reset()
							buffer.WriteString(remaining)
						} else {
							buffer.Reset()
						}

						position++
					}
				}

				// Read next token
				token, err = decoder.Token()
				if err != nil {
					return
				}
			}
		}

		// Process last token
		tokenBytes, err := json.Marshal(token)
		if err != nil {
			return
		}
		buffer.WriteString(string(tokenBytes))

		// Process remaining content
		if buffer.Len() > 0 {
			// 获取文件路径
			filePath := ""
			if path, ok := ctx.Value(filePathKey).(string); ok {
				filePath = path
			}

			docMetaCopy := make(map[string]any)
			for k, v := range docMeta {
				docMetaCopy[k] = v
			}
			docMetaCopy["type"] = "json"
			docMetaCopy["position"] = fmt.Sprintf("%d", position)
			docMetaCopy["file_path"] = filePath

			doc := entity.NewDocument(
				uuid.New().String(),
				strings.TrimSpace(buffer.String()),
				source,
				"application/json",
				docMetaCopy,
			)

			select {
			case <-ctx.Done():
				return
			case outChan <- doc:
			}
		}
	}()

	return outChan, nil
}
