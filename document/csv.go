package document

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
)

// ParseCSV 读取 CSV 文件并归一化为 dataDoc（内容为 JSON 字符串）。
//
// 归一化策略：
//   - 第一行作为表头（key），后续每行转为一个 JSON 对象
//   - 输出 JSON 数组字符串：[{"col1":"v1","col2":"v2"},...]
//   - 元数据包含 rows（含表头）和 columns
func ParseCSV(r io.Reader) (RawDoc, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV 文件为空")
	}

	maxCols := 0
	for _, record := range records {
		if len(record) > maxCols {
			maxCols = len(record)
		}
	}

	// 第一行作为表头
	headers := make([]string, maxCols)
	for i := 0; i < maxCols; i++ {
		if i < len(records[0]) {
			headers[i] = records[0][i]
		} else {
			headers[i] = fmt.Sprintf("col_%d", i+1)
		}
	}

	// 数据行转为对象数组
	rows := make([]map[string]string, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		obj := make(map[string]string, maxCols)
		for j := 0; j < maxCols; j++ {
			if j < len(records[i]) {
				obj[headers[j]] = records[i][j]
			} else {
				obj[headers[j]] = ""
			}
		}
		rows = append(rows, obj)
	}

	jsonBytes, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("CSV 转 JSON 失败: %w", err)
	}

	meta := map[string]any{
		"rows":    len(records),
		"columns": maxCols,
	}
	return newParsedDoc(string(jsonBytes), meta, RawDocData), nil
}
