package email

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/mail"
	"strings"

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/google/uuid"
)

// Parser implements an email parser with MIME support
type Parser struct {
	chunkSize      int
	chunkOverlap   int
	extractHeaders bool
	extractBody    bool
}

// NewParser creates a new email parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:      500,
		chunkOverlap:   50,
		extractHeaders: true,
		extractBody:    true,
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

// SetExtractHeaders sets whether to extract headers
func (p *Parser) SetExtractHeaders(extract bool) {
	p.extractHeaders = extract
}

// SetExtractBody sets whether to extract body
func (p *Parser) SetExtractBody(extract bool) {
	p.extractBody = extract
}

// Parse parses email into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]model.Chunk, error) {
	var chunks []model.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk model.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses email and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(model.Chunk) error) error {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return fmt.Errorf("email parsing error: %w", err)
	}

	var buffer strings.Builder
	var position int

	if p.extractHeaders {
		for key, values := range msg.Header {
			headerLine := fmt.Sprintf("%s: %s\n", key, strings.Join(values, ", "))
			buffer.WriteString(headerLine)

			if buffer.Len() >= p.chunkSize {
				chunkText := strings.TrimSpace(buffer.String())
				chunk := p.createChunk(chunkText, position, "header", key)

				if err := callback(chunk); err != nil {
					return err
				}

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

	if p.extractBody && msg.Body != nil {
		scanner := bufio.NewScanner(msg.Body)
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			line := scanner.Text()
			buffer.WriteString(line)
			buffer.WriteString("\n")

			if buffer.Len() >= p.chunkSize {
				chunkText := strings.TrimSpace(buffer.String())
				chunk := p.createChunk(chunkText, position, "body", "")

				if err := callback(chunk); err != nil {
					return err
				}

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

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("email body scanning error: %w", err)
		}
	}

	if buffer.Len() > 0 {
		chunkText := strings.TrimSpace(buffer.String())
		chunkType := "body"
		if !p.extractBody {
			chunkType = "header"
		}
		chunk := p.createChunk(chunkText, position, chunkType, "")

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) createChunk(content string, position int, chunkType string, headerName string) model.Chunk {
	metadata := map[string]string{
		"type":       "email",
		"position":   fmt.Sprintf("%d", position),
		"chunk_type": chunkType,
	}

	if headerName != "" {
		metadata["header_name"] = headerName
	}

	return model.Chunk{
		ID:       uuid.New().String(),
		Content:  content,
		Metadata: metadata,
	}
}

func (p *Parser) SupportedFormats() []string {
	return []string{".eml"}
}
