package document

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

func ParseCSV(r io.Reader) (*RawDocument, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	maxCols := 0
	for _, record := range records {
		if len(record) > maxCols {
			maxCols = len(record)
		}
	}

	doc := NewRawDoc(csvToMarkdown(records, maxCols))

	return doc.SetValue("rows", len(records)).
		SetValue("columns", maxCols), nil
}

func csvToMarkdown(records [][]string, maxCols int) string {
	if len(records) == 0 {
		return ""
	}

	var builder strings.Builder

	for i, record := range records {
		row := make([]string, maxCols)
		for j := 0; j < maxCols; j++ {
			if j < len(record) {
				row[j] = escapeCSVField(record[j])
			} else {
				row[j] = ""
			}
		}
		builder.WriteString("| " + strings.Join(row, " | ") + " |\n")

		if i == 0 {
			separators := make([]string, maxCols)
			for k := range separators {
				separators[k] = "---"
			}
			builder.WriteString("| " + strings.Join(separators, " | ") + " |\n")
		}
	}

	return builder.String()
}

func escapeCSVField(text string) string {
	text = strings.ReplaceAll(text, "|", "\\|")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.TrimSpace(text)
	return text
}
