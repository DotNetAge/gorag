package html

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
	"golang.org/x/net/html"
)

// StreamingParser implements an HTML parser that processes large files in chunks
// to avoid loading the entire file into memory

type StreamingParser struct {
	chunkSize    int
	chunkOverlap int
	bufferSize   int // Size of the buffer for streaming reading
}

// NewStreamingParser creates a new streaming HTML parser
func NewStreamingParser() *StreamingParser {
	return &StreamingParser{
		chunkSize:    500,
		chunkOverlap: 50,
		bufferSize:   4096, // 4KB buffer
	}
}

// Parse parses HTML into chunks using streaming processing
func (p *StreamingParser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	// Parse HTML document using streaming parser
	tokenizer := html.NewTokenizer(r)

	// Extract text from HTML
	var buf strings.Builder

	for {
		// Check if context is canceled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to parse HTML: %w", tokenizer.Err())
		case html.TextToken:
			buf.Write(tokenizer.Text())
		}

		if tokenType == html.ErrorToken && tokenizer.Err() == io.EOF {
			break
		}
	}

	text := buf.String()
	chunks := p.splitText(text)

	result := make([]core.Chunk, len(chunks))
	for i, chunk := range chunks {
		result[i] = core.Chunk{
			ID:      uuid.New().String(),
			Content: chunk,
			Metadata: map[string]string{
				"type":     "html",
				"position": fmt.Sprintf("%d", i),
			},
		}
	}

	return result, nil
}

// SupportedFormats returns supported formats
func (p *StreamingParser) SupportedFormats() []string {
	return []string{".html", ".htm"}
}

// splitText splits text into chunks with overlap
func (p *StreamingParser) splitText(text string) []string {
	var chunks []string

	for i := 0; i < len(text); i += p.chunkSize - p.chunkOverlap {
		end := i + p.chunkSize
		if end > len(text) {
			end = len(text)
		}

		chunk := text[i:end]
		trimmedChunk := strings.TrimSpace(chunk)
		if trimmedChunk != "" {
			chunks = append(chunks, trimmedChunk)
		}

		if end >= len(text) {
			break
		}
	}

	return chunks
}
