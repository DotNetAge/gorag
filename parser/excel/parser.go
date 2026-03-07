package excel

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
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
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses Excel and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	buffer := make([]byte, 0, p.chunkSize*2) // Preallocate buffer
	var position int

	// Iterate through all sheets
	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		sheetHeader := fmt.Sprintf("Sheet: %s\n", sheet)
		buffer = append(buffer, []byte(sheetHeader)...)

		// Get all rows in the sheet
		rows, err := f.GetRows(sheet)
		if err != nil {
			return fmt.Errorf("failed to get rows from sheet %s: %w", sheet, err)
		}

		for i, row := range rows {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				rowText := fmt.Sprintf("Row %d: %s\n", i+1, strings.Join(row, "\t"))
				buffer = append(buffer, []byte(rowText)...)

				// Check if we have enough content for a chunk
				if len(buffer) >= p.chunkSize {
					// Create chunk with overlap
					chunkText := string(buffer[:p.chunkSize])

					// Create chunk
					chunk := core.Chunk{
						ID:      uuid.New().String(),
						Content: strings.TrimSpace(chunkText),
						Metadata: map[string]string{
							"type":     "excel",
							"position": fmt.Sprintf("%d", position),
							"sheet":    sheet,
						},
					}

					// Call callback
					if err := callback(chunk); err != nil {
						return err
					}

					// Keep overlap for next chunk
					if p.chunkOverlap > 0 && len(buffer) > p.chunkOverlap {
						remaining := buffer[p.chunkSize-p.chunkOverlap:]
						buffer = make([]byte, len(remaining))
						copy(buffer, remaining)
					} else {
						buffer = buffer[:0]
					}

					position++
				}
			}
		}

		buffer = append(buffer, []byte("\n")...)
	}

	// Process remaining content
	if len(buffer) > 0 {
		chunk := core.Chunk{
			ID:      uuid.New().String(),
			Content: strings.TrimSpace(string(buffer)),
			Metadata: map[string]string{
				"type":     "excel",
				"position": fmt.Sprintf("%d", position),
			},
		}

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".xlsx", ".xls"}
}

