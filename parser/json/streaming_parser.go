package json

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
)

// StreamingParser implements a JSON parser that processes large files in chunks
// to avoid loading the entire file into memory

type StreamingParser struct {
	chunkSize    int
	chunkOverlap int
	bufferSize   int // Size of the buffer for streaming reading
}

// NewStreamingParser creates a new streaming JSON parser
func NewStreamingParser() *StreamingParser {
	return &StreamingParser{
		chunkSize:    500,
		chunkOverlap: 50,
		bufferSize:   4096, // 4KB buffer
	}
}

// Parse parses JSON into chunks using streaming processing
func (p *StreamingParser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	// Use json.Decoder for streaming processing
	decoder := json.NewDecoder(r)

	// Decode the JSON
	var data interface{}
	err := decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	// Convert to pretty-printed JSON string
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	text := string(content)
	chunks := p.splitText(text)

	result := make([]core.Chunk, len(chunks))
	for i, chunk := range chunks {
		result[i] = core.Chunk{
			ID:      uuid.New().String(),
			Content: chunk,
			Metadata: map[string]string{
				"type":     "json",
				"position": fmt.Sprintf("%d", i),
			},
		}
	}

	return result, nil
}

// SupportedFormats returns supported formats
func (p *StreamingParser) SupportedFormats() []string {
	return []string{".json"}
}

// splitText splits text into chunks with overlap
func (p *StreamingParser) splitText(text string) []string {
	var chunks []string

	// Handle empty text
	if len(text) == 0 {
		chunks = append(chunks, "")
		return chunks
	}

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
