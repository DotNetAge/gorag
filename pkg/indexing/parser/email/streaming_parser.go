package email

import (
	"fmt"
	"bytes"
	"github.com/DotNetAge/gorag/pkg/core"
	"bufio"
	"context"
	"io"
	"net/mail"
	"strings"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ core.Parser = (*EmailStreamParser)(nil)

// EmailStreamParser implements an email parser with MIME support
type EmailStreamParser struct {
	chunkSize      int
	chunkOverlap   int
	extractHeaders bool
	extractBody    bool
}

// DefaultEmailStreamParser creates a new email parser
func DefaultEmailStreamParser() *EmailStreamParser {
	return &EmailStreamParser{
		chunkSize:      500,
		chunkOverlap:   50,
		extractHeaders: true,
		extractBody:    true,
	}
}

// GetSupportedTypes returns supported formats
func (p *EmailStreamParser) GetSupportedTypes() []string {
	return []string{".eml"}
}

// ParseStream reads the incoming io.Reader and yields chunks of the document via a channel
func (p *EmailStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	outChan := make(chan *core.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "EmailStreamParser"
	docMeta["type"] = "email"

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		msg, err := mail.ReadMessage(r)
		if err != nil {
			return
		}

		var buffer strings.Builder
		var position int

		if p.extractHeaders {
			for key, values := range msg.Header {
				headerLine := key + ": " + strings.Join(values, ", ") + "\n"
				buffer.WriteString(headerLine)

				if buffer.Len() >= p.chunkSize {
					chunkText := strings.TrimSpace(buffer.String())
					docMetaCopy := copyMeta(docMeta)
					docMetaCopy["part_index"] = position
					docMetaCopy["position"] = position
					docMetaCopy["chunk_type"] = "header"
					docMetaCopy["header_name"] = key

					doc := core.NewDocument(
						uuid.New().String(),
						chunkText,
						source,
						"text/plain",
						docMetaCopy,
					)

					select {
					case <-ctx.Done():
						return
					case outChan <- doc:
						position++
						if p.chunkOverlap > 0 && buffer.Len() > p.chunkOverlap {
							remaining := buffer.String()[len(buffer.String())-p.chunkOverlap:]
							buffer.Reset()
							buffer.WriteString(remaining)
						} else {
							buffer.Reset()
						}
					}
				}
			}
		}

		if p.extractBody && msg.Body != nil {
			scanner := bufio.NewScanner(msg.Body)
			scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
					line := scanner.Text()
					buffer.WriteString(line)
					buffer.WriteString("\n")

					if buffer.Len() >= p.chunkSize {
						chunkText := strings.TrimSpace(buffer.String())
						docMetaCopy := copyMeta(docMeta)
						docMetaCopy["part_index"] = position
						docMetaCopy["position"] = position
						docMetaCopy["chunk_type"] = "body"

						doc := core.NewDocument(
							uuid.New().String(),
							chunkText,
							source,
							"text/plain",
							docMetaCopy,
						)

						select {
						case <-ctx.Done():
							return
						case outChan <- doc:
							position++
							if p.chunkOverlap > 0 && buffer.Len() > p.chunkOverlap {
								remaining := buffer.String()[len(buffer.String())-p.chunkOverlap:]
								buffer.Reset()
								buffer.WriteString(remaining)
							} else {
								buffer.Reset()
							}
						}
					}
				}
			}
		}

		if buffer.Len() > 0 {
			chunkText := strings.TrimSpace(buffer.String())
			chunkType := "body"
			if !p.extractBody {
				chunkType = "header"
			}
			docMetaCopy := copyMeta(docMeta)
			docMetaCopy["part_index"] = position
			docMetaCopy["position"] = position
			docMetaCopy["chunk_type"] = chunkType

			doc := core.NewDocument(
				uuid.New().String(),
				chunkText,
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


// Parse implements core.Parser interface.
func (p *EmailStreamParser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docChan, err := p.ParseStream(ctx, bytes.NewReader(content), metadata)
	if err != nil {
		return nil, err
	}
	for doc := range docChan {
		return doc, nil
	}
	return nil, fmt.Errorf("no document produced")
}

func (p *EmailStreamParser) Supports(contentType string) bool { return true }
