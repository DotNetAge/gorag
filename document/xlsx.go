package document

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/unidoc/unioffice/spreadsheet"
)

func ParseXlsx(r io.Reader) (*RawDocument, error) {
	var mdBuilder strings.Builder
	mediaList := [][]byte{}

	sheetCount := 0

	// 读取所有内容
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	f, err := spreadsheet.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.Sheets()
	sheetCount = len(sheets)

	for i, sheet := range sheets {
		if i > 0 {
			mdBuilder.WriteString("\n---\n\n")
		}
		mdBuilder.WriteString(fmt.Sprintf("## Sheet %d: %s\n\n", i+1, sheet.Name()))

		sheetText := sheet.ExtractText()
		if sheetText != nil && len(sheetText.Cells) > 0 {
			mdBuilder.WriteString(sheetToMarkdown(sheetText))
		}
	}

	// imageIndex := 0
	for _, img := range f.Images {
		data := img.Data()
		if data == nil || len(*data) == 0 {
			continue
		}
		mediaList = append(mediaList, *data)
	}

	return NewRawDoc(mdBuilder.String()).
		AddImages(mediaList).
		SetValue("sheet_count", sheetCount), nil
}

func sheetToMarkdown(sheetText *spreadsheet.SheetText) string {
	rowCells := make(map[int]map[int]string)
	maxRow := 0
	maxCol := 0

	for _, cellText := range sheetText.Cells {
		rowIdx := 1
		colIdx := 1
		if cellText.Cell.X() != nil && cellText.Cell.X().RAttr != nil {
			ref := *cellText.Cell.X().RAttr
			parts := splitRef(ref)
			if n, err := strconv.Atoi(parts[1]); err == nil {
				rowIdx = n
			}
			colIdx = colNameToIndex(parts[0])
		}

		if rowCells[rowIdx] == nil {
			rowCells[rowIdx] = make(map[int]string)
		}
		rowCells[rowIdx][colIdx] = strings.TrimSpace(cellText.Text)

		if rowIdx > maxRow {
			maxRow = rowIdx
		}
		if colIdx > maxCol {
			maxCol = colIdx
		}
	}

	if maxRow == 0 || maxCol == 0 {
		return ""
	}

	var builder strings.Builder
	sortedRows := make([]int, 0, len(rowCells))
	for rowIdx := range rowCells {
		sortedRows = append(sortedRows, rowIdx)
	}
	sort.Ints(sortedRows)

	for i, rowIdx := range sortedRows {
		cells := rowCells[rowIdx]
		rowData := make([]string, maxCol)
		for colIdx := 1; colIdx <= maxCol; colIdx++ {
			if cell, ok := cells[colIdx]; ok {
				rowData[colIdx-1] = escapeMarkdown(cell)
			} else {
				rowData[colIdx-1] = ""
			}
		}
		builder.WriteString("| " + strings.Join(rowData, " | ") + " |\n")

		if i == 0 {
			separators := make([]string, maxCol)
			for k := range separators {
				separators[k] = "---"
			}
			builder.WriteString("| " + strings.Join(separators, " | ") + " |\n")
		}
	}

	return builder.String() + "\n"
}

func colNameToIndex(colName string) int {
	result := 0
	for _, c := range colName {
		result = result*26 + int(c-'A') + 1
	}
	return result
}

func escapeMarkdown(text string) string {
	text = strings.ReplaceAll(text, "|", "\\|")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", "")
	return text
}

func splitRef(ref string) [2]string {
	col := ""
	row := ""
	for _, c := range ref {
		if c >= 'A' && c <= 'Z' {
			col += string(c)
		} else if c >= '0' && c <= '9' {
			row += string(c)
		}
	}
	return [2]string{col, row}
}
