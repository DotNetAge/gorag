package csv

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

var _ dataprep.Parser = (*CSVStreamParser)(nil)

// CSVStreamParser reads CSV files and streams them out.
// It bundles N rows into a single Document to balance chunking granularity.
type CSVStreamParser struct {
	rowsPerDocument int
	hasHeader       bool
}

func NewCSVStreamParser(rowsPerDocument int, hasHeader bool) *CSVStreamParser {
	if rowsPerDocument <= 0 {
		rowsPerDocument = 100 // Default: 100 rows per Document chunk
	}
	return &CSVStreamParser{
		rowsPerDocument: rowsPerDocument,
		hasHeader:       hasHeader,
	}
}

func (p *CSVStreamParser) GetSupportedTypes() []string {
	return []string{".csv", "text/csv"}
}

func (p *CSVStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "CSVStreamParser"

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		reader := csv.NewReader(r)
		// Relax constraints for dirty CSVs
		reader.FieldsPerRecord = -1
		reader.LazyQuotes = true

		var headers []string
		if p.hasHeader {
			h, err := reader.Read()
			if err != nil {
				return // End stream on error (e.g. empty file)
			}
			headers = h
		}

		var sb strings.Builder
		rowCount := 0
		docIndex := 0

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				continue // Skip bad rows in production
			}

			// Format the row as structured text
			if len(headers) > 0 && len(headers) == len(record) {
				for i, val := range record {
					sb.WriteString(fmt.Sprintf("%s: %s; ", headers[i], val))
				}
				sb.WriteString("\n")
			} else {
				sb.WriteString(strings.Join(record, ", ") + "\n")
			}

			rowCount++

			if rowCount >= p.rowsPerDocument {
				docMetaCopy := copyMeta(docMeta)
				docMetaCopy["part_index"] = docIndex
				
				doc := entity.NewDocument(
					uuid.New().String(),
					sb.String(),
					source,
					"text/csv",
					docMetaCopy,
				)

				select {
				case <-ctx.Done():
					return
				case outChan <- doc:
					sb.Reset()
					rowCount = 0
					docIndex++
				}
			}
		}

		// Flush remaining rows
		if rowCount > 0 {
			docMetaCopy := copyMeta(docMeta)
			docMetaCopy["part_index"] = docIndex
			doc := entity.NewDocument(
				uuid.New().String(),
				sb.String(),
				source,
				"text/csv",
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
