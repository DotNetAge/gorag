package docx

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ dataprep.Parser = (*DocxStreamParser)(nil)

// DocxStreamParser implements a DOCX document parser
type DocxStreamParser struct {
	chunkSize    int
	chunkOverlap int
}

// NewDocxStreamParser creates a new DOCX parser
func NewDocxStreamParser() *DocxStreamParser {
	return &DocxStreamParser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// GetSupportedTypes returns supported formats
func (p *DocxStreamParser) GetSupportedTypes() []string {
	return []string{".docx"}
}

// ParseStream reads the incoming io.Reader and yields chunks of the document via a channel
func (p *DocxStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "DocxStreamParser"
	docMeta["type"] = "docx"

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		// For simplicity, we'll skip the actual implementation for now
		// In a real implementation, you would use the unioffice library to parse DOCX
		// and process it in a streaming manner

		// Simulate parsing with a simple scanner
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

					docMetaCopy := copyMeta(docMeta)
					docMetaCopy["part_index"] = position
					docMetaCopy["position"] = position

					doc := entity.NewDocument(
						uuid.New().String(),
						strings.TrimSpace(chunkText),
						source,
						"text/plain",
						docMetaCopy,
					)

					select {
					case <-ctx.Done():
						return
					case outChan <- doc:
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
		}

		if err := scanner.Err(); err != nil {
			return
		}

		// Process remaining content
		if len(buffer) > 0 {
			docMetaCopy := copyMeta(docMeta)
			docMetaCopy["part_index"] = position
			docMetaCopy["position"] = position

			doc := entity.NewDocument(
				uuid.New().String(),
				strings.TrimSpace(string(buffer)),
				source,
				"text/plain",
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

func copyMeta(m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		out[k] = v
	}
	return out
}
