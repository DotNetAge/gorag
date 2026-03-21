package excel

import (
	"bytes"
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"io"
	"strings"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// ensure interface implementation
var _ core.Parser = (*ExcelStreamParser)(nil)

// ExcelStreamParser implements an Excel parser
type ExcelStreamParser struct {
	chunkSize    int
	chunkOverlap int
}

// DefaultExcelStreamParser creates a new Excel parser
func DefaultExcelStreamParser() *ExcelStreamParser {
	return &ExcelStreamParser{
		chunkSize:    500,
		chunkOverlap: 50,
	}
}

// GetSupportedTypes returns supported formats
func (p *ExcelStreamParser) GetSupportedTypes() []string {
	return []string{".xlsx", ".xls"}
}

// ParseStream reads the incoming io.Reader and yields chunks of the document via a channel
func (p *ExcelStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	outChan := make(chan *core.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "ExcelStreamParser"
	docMeta["type"] = "excel"

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		f, err := excelize.OpenReader(r)
		if err != nil {
			return
		}
		defer f.Close()

		buffer := make([]byte, 0, p.chunkSize*2) // Preallocate buffer
		var position int

		// Iterate through all sheets
		sheets := f.GetSheetList()
		for _, sheet := range sheets {
			sheetHeader := "Sheet: " + sheet + "\n"
			buffer = append(buffer, []byte(sheetHeader)...)

			// Get all rows in the sheet
			rows, err := f.GetRows(sheet)
			if err != nil {
				return
			}

			for i, row := range rows {
				select {
				case <-ctx.Done():
					return
				default:
					rowText := "Row " + fmt.Sprintf("%d", i+1) + ": " + strings.Join(row, "\t") + "\n"
					buffer = append(buffer, []byte(rowText)...)

					// Check if we have enough content for a chunk
					if len(buffer) >= p.chunkSize {
						// Create chunk with overlap
						chunkText := string(buffer[:p.chunkSize])

						docMetaCopy := copyMeta(docMeta)
						docMetaCopy["part_index"] = position
						docMetaCopy["position"] = position
						docMetaCopy["sheet"] = sheet

						doc := core.NewDocument(
							uuid.New().String(),
							strings.TrimSpace(chunkText),
							source,
							"text/plain",
							docMetaCopy,
						)

						select {
						case <-ctx.Done():
							return
						case outChan <- doc:
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
			}

			buffer = append(buffer, []byte("\n")...)
		}

		// Process remaining content
		if len(buffer) > 0 {
			docMetaCopy := copyMeta(docMeta)
			docMetaCopy["part_index"] = position
			docMetaCopy["position"] = position

			doc := core.NewDocument(
				uuid.New().String(),
				strings.TrimSpace(string(buffer)),
				source,
				"text/plain",
				docMetaCopy,
			)

			select {
			case <-ctx.Done():
				return
			case outChan <- doc:
			}
		}
	}()

	return outChan, nil
}

func copyMeta(m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		out[k] = v
	}
	return out
}


// Parse implements core.Parser interface.
func (p *ExcelStreamParser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docChan, err := p.ParseStream(ctx, bytes.NewReader(content), metadata)
	if err != nil {
		return nil, err
	}
	for doc := range docChan {
		return doc, nil
	}
	return nil, fmt.Errorf("no document produced")
}

func (p *ExcelStreamParser) Supports(contentType string) bool { return true }
