package csv

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/google/uuid"
)

// Parser implements CSV/TSV parser with streaming support (default behavior)
type Parser struct {
	chunkSize    int
	chunkOverlap int
	detectSep    bool
	separator    rune
}

// NewParser creates a new CSV/TSV parser (streaming by default)
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
		detectSep:    true,
		separator:    ',',
	}
}

// Parse parses CSV/TSV using streaming processing (default behavior)
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]model.Chunk, error) {
	var chunks []model.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk model.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses CSV/TSV and calls the callback for each chunk
// This is the primary method - all parsing is streaming by default
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(model.Chunk) error) error {
	reader := bufio.NewReader(r)

	// State tracking
	var currentChunk strings.Builder
	var overlapBuffer strings.Builder
	var position int
	var rowNum int64
	var separator rune

	// Auto-detect separator if enabled
	if p.detectSep {
		sep, err := p.detectSeparator(reader)
		if err == nil {
			separator = sep
		} else {
			separator = p.separator
		}
	} else {
		separator = p.separator
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read line by line for streaming processing
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

					chunk := p.createChunk(chunkText, position, rowNum)
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

		// Process line
		lineStr := string(line)
		if strings.TrimSpace(lineStr) == "" {
			continue // Skip empty lines
		}

		// Parse CSV/TSV row
		fields, err := p.parseLine(lineStr, separator)
		if err != nil {
			// Continue processing even if a line fails
			continue
		}

		// Convert fields to readable text
		rowText := p.formatRow(fields, rowNum)
		currentChunk.WriteString(rowText)
		rowNum++

		// Check if we should emit a chunk
		if currentChunk.Len() >= p.chunkSize {
			chunkText := currentChunk.String()

			// Add overlap from previous chunk
			if overlapBuffer.Len() > 0 {
				chunkText = overlapBuffer.String() + chunkText
			}

			chunk := p.createChunk(chunkText, position, rowNum)
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

// detectSeparator auto-detects the separator character
func (p *Parser) detectSeparator(reader *bufio.Reader) (rune, error) {
	// Peek at first line without consuming
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return ',', err
	}

	// Put the line back by creating a custom reader
	// For simplicity, just detect from this line
	lineStr := string(line)

	// Count potential separators
	commaCount := strings.Count(lineStr, ",")
	tabCount := strings.Count(lineStr, "\t")
	semicolonCount := strings.Count(lineStr, ";")

	// Choose the most frequent separator
	maxCount := commaCount
	separator := ','

	if tabCount > maxCount {
		maxCount = tabCount
		separator = '\t'
	}

	if semicolonCount > maxCount {
		separator = ';'
	}

	return separator, nil
}

// parseLine parses a single CSV/TSV line
func (p *Parser) parseLine(line string, separator rune) ([]string, error) {
	var fields []string
	var currentField strings.Builder
	inQuotes := false

	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch {
		case inQuotes:
			if r == '"' {
				// Check for escaped quote
				if i+1 < len(runes) && runes[i+1] == '"' {
					currentField.WriteRune('"')
					i++ // Skip next quote
				} else {
					inQuotes = false
				}
			} else {
				currentField.WriteRune(r)
			}

		case r == '"':
			inQuotes = true

		case r == separator:
			fields = append(fields, strings.TrimSpace(currentField.String()))
			currentField.Reset()

		default:
			currentField.WriteRune(r)
		}
	}

	// Add last field
	fields = append(fields, strings.TrimSpace(currentField.String()))

	return fields, nil
}

// formatRow formats a row as readable text
func (p *Parser) formatRow(fields []string, rowNum int64) string {
	if len(fields) == 0 {
		return ""
	}

	// Format as: Row N: field1 | field2 | field3
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Row %d: ", rowNum))

	for i, field := range fields {
		if i > 0 {
			builder.WriteString(" | ")
		}
		builder.WriteString(field)
	}

	builder.WriteString("\n")
	return builder.String()
}

// createChunk creates a chunk with metadata
func (p *Parser) createChunk(content string, position int, rowNum int64) model.Chunk {
	metadata := map[string]string{
		"type":       "csv",
		"position":   fmt.Sprintf("%d", position),
		"streaming":  "true",
		"rows_until": fmt.Sprintf("%d", rowNum),
	}

	return model.Chunk{
		ID:       uuid.New().String(),
		Content:  strings.TrimSpace(content),
		Metadata: metadata,
	}
}

// SupportedFormats returns supported file formats
func (p *Parser) SupportedFormats() []string {
	return []string{".csv", ".tsv", ".tab"}
}

// SetChunkSize sets the chunk size
func (p *Parser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *Parser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// SetDetectSep enables or disables auto-detection of separator
func (p *Parser) SetDetectSep(enabled bool) {
	p.detectSep = enabled
}

// SetSeparator sets the separator character
func (p *Parser) SetSeparator(sep rune) {
	p.separator = sep
	p.detectSep = false
}

// IsWhitespace checks if a rune is whitespace
func isWhitespace(r rune) bool {
	return unicode.IsSpace(r) && r != '\n' && r != '\r'
}
