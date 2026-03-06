package excel

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/parser"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// Parser implements an Excel parser
 type Parser struct {
	chunkSize    int
	chunkOverlap int
}

// NewParser creates a new Excel parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// Parse parses Excel file into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]parser.Chunk, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	var content strings.Builder

	// Iterate through all sheets
	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		content.WriteString(fmt.Sprintf("Sheet: %s\n", sheet))
		
		// Get all rows in the sheet
		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("failed to get rows from sheet %s: %w", sheet, err)
		}
		
		for i, row := range rows {
			content.WriteString(fmt.Sprintf("Row %d: ", i+1))
			content.WriteString(strings.Join(row, "\t"))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	text := content.String()
	chunks := p.splitText(text)

	result := make([]parser.Chunk, len(chunks))
	for i, chunk := range chunks {
		result[i] = parser.Chunk{
			ID:      uuid.New().String(),
			Content: chunk,
			Metadata: map[string]string{
				"type":     "excel",
				"position": fmt.Sprintf("%d", i),
			},
		}
	}

	return result, nil
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".xlsx", ".xls"}
}

// splitText splits text into chunks with overlap
func (p *Parser) splitText(text string) []string {
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
		chunks = append(chunks, strings.TrimSpace(chunk))

		if end >= len(text) {
			break
		}
	}

	return chunks
}
