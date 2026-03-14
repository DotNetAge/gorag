package text

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
var _ dataprep.Parser = (*TextStreamParser)(nil)

// TextStreamParser is a true streaming parser that reads text/markdown files.
// It complies with the Single Responsibility Principle by only extracting content 
// into Document entities and leaving chunking to the Chunker.
type TextStreamParser struct {
	// The maximum bytes to read before yielding a Document part.
	// This ensures memory footprint remains O(1) even for GB-sized log files.
	maxReadBytes int 
}

// NewTextStreamParser creates a new parser optimized for raw text.
func NewTextStreamParser(maxReadBytes int) *TextStreamParser {
	if maxReadBytes <= 0 {
		maxReadBytes = 10 * 1024 * 1024 // Default to 10MB parts
	}
	return &TextStreamParser{
		maxReadBytes: maxReadBytes,
	}
}

// GetSupportedTypes returns the MIME types or extensions this parser can handle.
func (p *TextStreamParser) GetSupportedTypes() []string {
	return []string{".txt", ".md", ".csv", ".log", "text/plain", "text/markdown"}
}

// ParseStream reads the incoming io.Reader and yields chunks of the document via a channel.
func (p *TextStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 1) // buffered channel to decouple reader/writer slightly
	
	// Create a safe copy of metadata for the documents
	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "TextStreamParser"
	
	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)
		
		scanner := bufio.NewScanner(r)
		// We use a custom split function if we want to chunk by specific bytes,
		// but for general text, reading line by line and accumulating is safer for UTF-8.
		
		var sb strings.Builder
		partIndex := 0

		for scanner.Scan() {
			// Check for context cancellation before processing
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			sb.WriteString(line)
			sb.WriteString("\n")

			// Yield the document part when it reaches the memory threshold
			if sb.Len() >= p.maxReadBytes {
				doc := entity.NewDocument(
					uuid.New().String(),
					sb.String(),
					source,
					"text/plain",
					docMeta,
				)
				doc.Metadata["part_index"] = partIndex
				
				select {
				case <-ctx.Done():
					return
				case outChan <- doc:
					// successfully sent, reset buffer
					sb.Reset()
					partIndex++
				}
			}
		}

		// Send any remaining text as the final document part
		if sb.Len() > 0 {
			doc := entity.NewDocument(
				uuid.New().String(),
				sb.String(),
				source,
				"text/plain",
				docMeta,
			)
			doc.Metadata["part_index"] = partIndex
			
			select {
			case <-ctx.Done():
				return
			case outChan <- doc:
			}
		}
	}()

	return outChan, nil
}
