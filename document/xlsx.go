package document

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/tealeg/xlsx"
)

// ParseXlsx 读取 .xlsx 文件并归一化为 dataDoc（内容为 JSON 字符串）。
//
// 归一化策略：
//   - 每个 sheet 转为一个对象，包含 name 和 rows
//   - 每个 sheet 的第一行作为表头，后续每行转为对象
//   - 输出 JSON 数组字符串：[{"sheet":"Sheet1","rows":[...]},...]
//   - 元数据包含 sheet_count
func ParseXlsx(r io.Reader) (RawDoc, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	xlFile, err := xlsx.OpenBinary(data)
	if err != nil {
		return nil, err
	}

	sheets := make([]map[string]any, 0, len(xlFile.Sheets))
	sheetNames := []string{}

	for _, sheet := range xlFile.Sheets {
		sheetNames = append(sheetNames, sheet.Name)

		// 收集所有行
		allRows := sheet.Rows
		if len(allRows) == 0 {
			sheets = append(sheets, map[string]any{
				"sheet": sheet.Name,
				"rows":  []map[string]string{},
			})
			continue
		}

		// 计算 maxCol
		maxCol := 0
		for _, row := range allRows {
			if len(row.Cells) > maxCol {
				maxCol = len(row.Cells)
			}
		}
		if maxCol == 0 {
			sheets = append(sheets, map[string]any{
				"sheet": sheet.Name,
				"rows":  []map[string]string{},
			})
			continue
		}

		// 第一行作为表头
		headers := make([]string, maxCol)
		for i := 0; i < maxCol; i++ {
			if i < len(allRows[0].Cells) {
				headers[i] = allRows[0].Cells[i].String()
			} else {
				headers[i] = fmt.Sprintf("col_%d", i+1)
			}
		}

		// 数据行转为对象数组
		rows := make([]map[string]string, 0, len(allRows)-1)
		for i := 1; i < len(allRows); i++ {
			row := allRows[i]
			obj := make(map[string]string, maxCol)
			hasContent := false
			for ci := 0; ci < maxCol; ci++ {
				if ci < len(row.Cells) {
					val := row.Cells[ci].String()
					obj[headers[ci]] = val
					if val != "" {
						hasContent = true
					}
				} else {
					obj[headers[ci]] = ""
				}
			}
			if hasContent {
				rows = append(rows, obj)
			}
		}

		sheets = append(sheets, map[string]any{
			"sheet": sheet.Name,
			"rows":  rows,
		})
	}

	jsonBytes, err := json.Marshal(sheets)
	if err != nil {
		return nil, fmt.Errorf("XLSX 转 JSON 失败: %w", err)
	}

	meta := map[string]any{
		"sheet_count": len(sheetNames),
	}
	return newParsedDoc(string(jsonBytes), meta, RawDocData), nil
}
