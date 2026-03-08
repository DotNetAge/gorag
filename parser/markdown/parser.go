package markdown

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
)

// Parser implements markdown parser with streaming support (default behavior)
type Parser struct {
	chunkSize        int
	chunkOverlap     int
	parseFrontmatter bool
	extractTOC       bool
	enableCodeBlocks bool
	enableLinks      bool
}

// NewParser creates a new markdown parser (streaming by default)
func NewParser() *Parser {
	return &Parser{
		chunkSize:        500,
		chunkOverlap:     50,
		parseFrontmatter: true,
		extractTOC:       true,
		enableCodeBlocks: true,
		enableLinks:      true,
	}
}

// Parse parses markdown using streaming processing (default behavior)
// For large files, this automatically uses streaming to avoid loading entire file into memory
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses markdown and calls the callback for each chunk
// This is the primary method - all parsing is streaming by default
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	reader := bufio.NewReader(r)

	// State tracking
	var currentChunk strings.Builder
	var overlapBuffer strings.Builder
	var position int
	var frontmatterProcessed bool

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read line by line for better memory efficiency
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// Process remaining content
				if currentChunk.Len() > 0 {
					chunkText := currentChunk.String()

					// Add overlap from previous chunk
					if overlapBuffer.Len() > 0 {
						chunkText = overlapBuffer.String() + chunkText
					}

					chunk := p.createChunk(chunkText, position, frontmatterProcessed)
					if err := callback(chunk); err != nil {
						return err
					}
				}
				break
			}
			return fmt.Errorf("failed to read input: %w", err)
		}

		// Check context after successful read
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Handle frontmatter specially
		if !frontmatterProcessed && p.parseFrontmatter {
			if bytes.HasPrefix(line, []byte("---")) {
				// Found frontmatter start, read until end
				frontmatterLines := []string{string(line)}
				for {
					nextLine, readErr := reader.ReadBytes('\n')
					if readErr != nil {
						break
					}
					frontmatterLines = append(frontmatterLines, string(nextLine))
					if bytes.HasPrefix(bytes.TrimSpace(nextLine), []byte("---")) {
						break
					}
				}
				frontmatterProcessed = true

				// Create a chunk for frontmatter metadata
				fmContent := strings.Join(frontmatterLines, "")
				chunk := p.createChunk(fmContent, position, true)
				chunk.Metadata["type"] = "markdown_frontmatter"
				if err := callback(chunk); err != nil {
					return err
				}
				position++
				continue
			}
			frontmatterProcessed = true // No frontmatter found
		}

		// Add line to current chunk
		currentChunk.Write(line)

		// Check if we should emit a chunk
		if currentChunk.Len() >= p.chunkSize {
			// Emit chunk with overlap handling
			chunkText := currentChunk.String()

			// Add overlap from previous chunk
			if overlapBuffer.Len() > 0 {
				chunkText = overlapBuffer.String() + chunkText
			}

			chunk := p.createChunk(chunkText, position, frontmatterProcessed)
			if err := callback(chunk); err != nil {
				return err
			}

			// Prepare overlap for next chunk
			lastPart := chunkText
			if len(lastPart) > p.chunkOverlap {
				lastPart = lastPart[len(lastPart)-p.chunkOverlap:]
			}
			overlapBuffer.Reset()
			overlapBuffer.WriteString(lastPart)

			currentChunk.Reset()
			position++
		}
	}

	return nil
}

// createChunk creates a chunk with metadata
func (p *Parser) createChunk(content string, position int, hasFrontmatter bool) core.Chunk {
	metadata := map[string]string{
		"type":            "markdown",
		"position":        fmt.Sprintf("%d", position),
		"has_frontmatter": fmt.Sprintf("%v", hasFrontmatter),
		"streaming":       "true",
	}

	return core.Chunk{
		ID:       uuid.New().String(),
		Content:  strings.TrimSpace(content),
		Metadata: metadata,
	}
}

// SupportedFormats returns supported file formats
func (p *Parser) SupportedFormats() []string {
	return []string{".md", ".markdown", ".mdown", ".mkd", ".mkdn", ".mdwn"}
}

// SetChunkSize sets the chunk size
func (p *Parser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *Parser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// SetParseFrontmatter enables or disables frontmatter parsing
func (p *Parser) SetParseFrontmatter(enabled bool) {
	p.parseFrontmatter = enabled
}

// SetExtractTOC enables or disables TOC extraction
func (p *Parser) SetExtractTOC(enabled bool) {
	p.extractTOC = enabled
}

// SetEnableCodeBlocks enables or disables code block extraction
func (p *Parser) SetEnableCodeBlocks(enabled bool) {
	p.enableCodeBlocks = enabled
}

// SetEnableLinks enables or disables link extraction
func (p *Parser) SetEnableLinks(enabled bool) {
	p.enableLinks = enabled
}
