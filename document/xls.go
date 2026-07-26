package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/extrame/xls"
)

// ParseXls 读取 .xls（旧版 BIFF 格式）文件并归一化为 dataDoc（内容为 JSON 字符串）。
//
// .xls 是 Excel 97-2003 的二进制格式，与 .xlsx（OOXML / ZIP）不同；
// 解析需要 ole2 + xls 专用库（github.com/extrame/xls）。
//
// 归一化策略：
//   - 每个 sheet 转为一个对象，包含 name 和 rows
//   - 每个 sheet 的第一行作为表头，后续每行转为对象
//   - 输出 JSON 数组字符串：[{"sheet":"Sheet1","rows":[...]},...]
//   - 元数据包含 sheet_count
//
// 容错处理：使用 defer recover 兜底单个 sheet 的解析异常，
// 避免某个 sheet 的格式异常导致整个文件解析失败。
func ParseXls(r io.Reader) (RawDoc, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// extrame/xls 库需要 io.ReadSeeker；用 bytes.NewReader 包装
	var xlFile *xls.WorkBook
	var openErr error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				openErr = fmt.Errorf("解析 .xls 文件时发生 panic: %v", rec)
			}
		}()
		xlFile, openErr = xls.OpenReader(bytes.NewReader(data), "utf-8")
	}()
	if openErr != nil {
		return nil, openErr
	}
	if xlFile == nil {
		return nil, fmt.Errorf("解析 .xls 文件失败: 库返回空")
	}

	sheets := make([]map[string]any, 0, xlFile.NumSheets())
	sheetNames := make([]string, 0, xlFile.NumSheets())

	for i := 0; i < xlFile.NumSheets(); i++ {
		// 单 sheet 异常不应影响其他 sheet
		sheetData, sheetName, ok := parseOneSheet(xlFile, i)
		if !ok {
			continue
		}
		sheetNames = append(sheetNames, sheetName)
		sheets = append(sheets, sheetData)
	}

	// 每个 sheet 单独 marshal 后用换行符拼接，让下游 DatumChunker 能按 sheet 边界切分；
	// 顶层使用 NDJSON 风格（每行一个独立 JSON 对象），而不是单行数组，
	// 避免 DatumChunker 的 tree-sitter 解析失败时回退到按行切分也只产出 1 个 chunk。
	var buf bytes.Buffer
	for i, sheet := range sheets {
		b, err := json.Marshal(sheet)
		if err != nil {
			return nil, fmt.Errorf("XLS 第 %d 个 sheet 序列化失败: %w", i, err)
		}
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.Write(b)
	}

	meta := map[string]any{
		"sheet_count": len(sheetNames),
	}
	return newParsedDoc(buf.String(), meta, RawDocData), nil
}

// parseOneSheet 解析单个 sheet；异常时返回 ok=false。
func parseOneSheet(xlFile *xls.WorkBook, idx int) (map[string]any, string, bool) {
	var sheetData map[string]any
	var sheetName string

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				// 记录后返回 ok=false（外层跳过）
				sheetData = nil
				sheetName = ""
			}
		}()

		sheet := xlFile.GetSheet(idx)
		if sheet == nil {
			return
		}
		sheetName = sheet.Name

		lastCol := 0
		maxRow := max(int(sheet.MaxRow), 0)

		// 第一遍：扫描每行找 lastCol 最大值
		for r := 0; r <= maxRow; r++ {
			row := sheet.Row(r)
			if row == nil {
				continue
			}
			if row.LastCol() > lastCol {
				lastCol = row.LastCol()
			}
		}
		if lastCol == 0 {
			sheetData = map[string]any{
				"sheet": sheetName,
				"rows":  []map[string]string{},
			}
			return
		}

		// 第二遍：收集所有行
		allRows := make([][]string, 0, maxRow+1)
		for r := 0; r <= maxRow; r++ {
			row := sheet.Row(r)
			rowData := make([]string, lastCol+1)
			if row != nil {
				for c := 0; c <= lastCol; c++ {
					rowData[c] = row.Col(c)
				}
			}
			allRows = append(allRows, rowData)
		}

		if len(allRows) == 0 {
			sheetData = map[string]any{
				"sheet": sheetName,
				"rows":  []map[string]string{},
			}
			return
		}

		// 第一行作为表头
		headers := make([]string, lastCol+1)
		for i := 0; i <= lastCol; i++ {
			if i < len(allRows[0]) {
				headers[i] = allRows[0][i]
				if headers[i] == "" {
					headers[i] = fmt.Sprintf("col_%d", i+1)
				}
			} else {
				headers[i] = fmt.Sprintf("col_%d", i+1)
			}
		}

		// 数据行转为对象数组
		rows := make([]map[string]string, 0, len(allRows)-1)
		for i := 1; i < len(allRows); i++ {
			row := allRows[i]
			obj := make(map[string]string, lastCol+1)
			hasContent := false
			for ci := 0; ci <= lastCol; ci++ {
				val := ""
				if ci < len(row) {
					val = row[ci]
				}
				obj[headers[ci]] = val
				if val != "" {
					hasContent = true
				}
			}
			if hasContent {
				rows = append(rows, obj)
			}
		}

		sheetData = map[string]any{
			"sheet": sheetName,
			"rows":  rows,
		}
	}()

	return sheetData, sheetName, sheetData != nil
}
