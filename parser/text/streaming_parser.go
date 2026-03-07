package text

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
)

// StreamingParser implements a text parser that processes large files in chunks
// to avoid loading the entire file into memory

type StreamingParser struct {
	chunkSize    int
	chunkOverlap int
	bufferSize   int // Size of the buffer for streaming reading
}

// NewStreamingParser creates a new streaming text parser
func NewStreamingParser() *StreamingParser {
	return &StreamingParser{
		chunkSize:    500,
		chunkOverlap: 50,
		bufferSize:   4096, // 4KB buffer
	}
}

// Parse parses text into chunks using streaming processing
func (p *StreamingParser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []string
	var currentBuffer strings.Builder
	var overlapBuffer strings.Builder

	// Create a buffer for streaming reading
	buffer := make([]byte, p.bufferSize)

	for {
		// Check if context is canceled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Read a chunk of data
		n, err := r.Read(buffer)
		if err != nil {
			if err == io.EOF {
				// Process any remaining data in the buffer
				if currentBuffer.Len() > 0 {
					chunks = p.processBuffer(currentBuffer.String(), chunks)
				}
				break
			}
			return nil, err
		}

		// Add the read data to the current buffer
		currentBuffer.Write(buffer[:n])

		// Process the buffer when it reaches a reasonable size
		if currentBuffer.Len() > p.chunkSize*2 {
			chunks = p.processBuffer(currentBuffer.String(), chunks)
			// Keep the overlap in the buffer
			overlapBuffer.Reset()
			if len(chunks) > 0 {
				lastChunk := chunks[len(chunks)-1]
				if len(lastChunk) > p.chunkOverlap {
					overlapBuffer.WriteString(lastChunk[len(lastChunk)-p.chunkOverlap:])
				}
			}
			currentBuffer.Reset()
			currentBuffer.WriteString(overlapBuffer.String())
		}
	}

	// Convert chunks to core.Chunk format
	result := make([]core.Chunk, len(chunks))
	for i, chunk := range chunks {
		result[i] = core.Chunk{
			ID:      uuid.New().String(),
			Content: chunk,
			Metadata: map[string]string{
				"type":     "text",
				"position": fmt.Sprintf("%d", i),
			},
		}
	}

	return result, nil
}

// SupportedFormats returns supported formats
func (p *StreamingParser) SupportedFormats() []string {
	return []string{".txt", ".md"}
}

// processBuffer processes the current buffer into chunks
func (p *StreamingParser) processBuffer(buffer string, chunks []string) []string {
	for i := 0; i < len(buffer); i += p.chunkSize - p.chunkOverlap {
		end := i + p.chunkSize
		if end > len(buffer) {
			end = len(buffer)
		}

		chunk := buffer[i:end]
		trimmedChunk := strings.TrimSpace(chunk)
		if trimmedChunk != "" {
			chunks = append(chunks, trimmedChunk)
		}

		if end >= len(buffer) {
			break
		}
	}
	return chunks
}
