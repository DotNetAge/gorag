package document

import (
	"fmt"
	"io"
	"strings"

	"github.com/tealeg/xlsx"
)

// ParseXlsx reads an .xlsx file and converts it to Markdown tables.
func ParseXlsx(r io.Reader) (*RawDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	xlFile, err := xlsx.OpenBinary(data)
	if err != nil {
		return nil, err
	}

	var mdBuilder strings.Builder
	sheetNames := []string{}

	for i, sheet := range xlFile.Sheets {
		sheetNames = append(sheetNames, sheet.Name)
		if i > 0 {
			mdBuilder.WriteString("\n---\n\n")
		}
		mdBuilder.WriteString(fmt.Sprintf("## Sheet %d: %s\n\n", i+1, sheet.Name))

		if len(sheet.Rows) == 0 {
			continue
		}

		// Normalize column count
		maxCol := 0
		for _, row := range sheet.Rows {
			if len(row.Cells) > maxCol {
				maxCol = len(row.Cells)
			}
		}
		if maxCol == 0 {
			continue
		}

		for ri, row := range sheet.Rows {
			rowData := make([]string, maxCol)
			hasContent := false
			for ci := 0; ci < maxCol; ci++ {
				if ci < len(row.Cells) {
					val := strings.TrimSpace(row.Cells[ci].Value)
					rowData[ci] = val
					if val != "" {
						hasContent = true
					}
				}
			}
			if !hasContent {
				continue
			}

			escaped := make([]string, maxCol)
			for ci, cell := range rowData {
				escaped[ci] = escapeMarkdown(cell)
			}
			mdBuilder.WriteString("| " + strings.Join(escaped, " | ") + " |\n")

			if ri == 0 {
				seps := make([]string, maxCol)
				for k := range seps {
					seps[k] = "---"
				}
				mdBuilder.WriteString("| " + strings.Join(seps, " | ") + " |\n")
			}
		}
		mdBuilder.WriteString("\n")
	}

	return NewRawDoc(mdBuilder.String()).
		SetValue("sheet_count", len(sheetNames)), nil
}

// escapeMarkdown escapes pipe and newline in markdown table cells.
func escapeMarkdown(text string) string {
	text = strings.ReplaceAll(text, "|", "\\|")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", "")
	return text
}
