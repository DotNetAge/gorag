package dbschema

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/google/uuid"
)

// Parser implements a database schema parser for SQL DDL
type Parser struct {
	chunkSize      int
	chunkOverlap   int
	extractTables  bool
	extractColumns bool
	extractIndexes bool
}

// NewParser creates a new database schema parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:      500,
		chunkOverlap:   50,
		extractTables:  true,
		extractColumns: true,
		extractIndexes: true,
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

// SetExtractTables sets whether to extract tables
func (p *Parser) SetExtractTables(extract bool) {
	p.extractTables = extract
}

// SetExtractColumns sets whether to extract columns
func (p *Parser) SetExtractColumns(extract bool) {
	p.extractColumns = extract
}

// SetExtractIndexes sets whether to extract indexes
func (p *Parser) SetExtractIndexes(extract bool) {
	p.extractIndexes = extract
}

// Parse parses SQL DDL into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]model.Chunk, error) {
	var chunks []model.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk model.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses SQL DDL and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(model.Chunk) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	var buffer strings.Builder
	var position int
	var inCreateTable bool
	var currentTable strings.Builder
	var tableName string
	var parenCount int

	createTablePattern := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
	createIndexPattern := regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s+ON\s+(\w+)`)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if !inCreateTable {
			if p.extractTables && createTablePattern.MatchString(trimmedLine) {
				matches := createTablePattern.FindStringSubmatch(trimmedLine)
				if len(matches) >= 2 {
					tableName = matches[1]
					inCreateTable = true
					parenCount = strings.Count(line, "(") - strings.Count(line, ")")
					currentTable.Reset()
					currentTable.WriteString(fmt.Sprintf("-- TABLE: %s\n", tableName))
					currentTable.WriteString(line)
					currentTable.WriteString("\n")
					continue
				}
			}

			if p.extractIndexes && createIndexPattern.MatchString(trimmedLine) {
				matches := createIndexPattern.FindStringSubmatch(trimmedLine)
				if len(matches) >= 3 {
					indexName := matches[1]
					tableName := matches[2]
					chunk := p.createChunk(line, position, "index", tableName, indexName)
					if err := callback(chunk); err != nil {
						return err
					}
					position++
					continue
				}
			}

			buffer.WriteString(line)
			buffer.WriteString("\n")

			if buffer.Len() >= p.chunkSize {
				chunkText := strings.TrimSpace(buffer.String())
				chunk := p.createChunk(chunkText, position, "mixed", "", "")

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
		} else {
			currentTable.WriteString(line)
			currentTable.WriteString("\n")
			parenCount += strings.Count(line, "(") - strings.Count(line, ")")

			if parenCount <= 0 {
				chunk := p.createChunk(currentTable.String(), position, "table", tableName, "")
				if err := callback(chunk); err != nil {
					return err
				}
				position++
				inCreateTable = false
				currentTable.Reset()
				tableName = ""
			}
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("SQL DDL scanning error: %w", err)
	}

	if inCreateTable && currentTable.Len() > 0 {
		chunk := p.createChunk(currentTable.String(), position, "table", tableName, "")
		if err := callback(chunk); err != nil {
			return err
		}
		position++
	}

	if buffer.Len() > 0 {
		chunkText := strings.TrimSpace(buffer.String())
		chunk := p.createChunk(chunkText, position, "mixed", "", "")

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) createChunk(content string, position int, chunkType string, tableName string, objectName string) model.Chunk {
	metadata := map[string]string{
		"type":       "dbschema",
		"position":   fmt.Sprintf("%d", position),
		"chunk_type": chunkType,
	}

	if tableName != "" {
		metadata["table_name"] = tableName
	}

	if objectName != "" {
		metadata["object_name"] = objectName
	}

	return model.Chunk{
		ID:       uuid.New().String(),
		Content:  content,
		Metadata: metadata,
	}
}

func (p *Parser) SupportedFormats() []string {
	return []string{".sql"}
}
