package document

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ParseLog 读取日志文件并归一化为 dataDoc（内容为 JSON 字符串）。
//
// 归一化策略：
//   - 按行切分日志，每行转为一个 JSON 对象 {"line":N,"text":"..."}
//   - 输出 JSON 数组字符串：[{"line":1,"text":"..."},{"line":2,"text":"..."}]
//   - 空行会被跳过
//   - 元数据包含 lines（总行数，含空行）
func ParseLog(r io.Reader) (RawDoc, error) {
	scanner := bufio.NewScanner(r)
	// 提高单行上限到 1MB 以容纳长日志行
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	entries := make([]map[string]any, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		text := strings.TrimRight(scanner.Text(), "\r")
		// 跳过空行
		if strings.TrimSpace(text) == "" {
			continue
		}
		entries = append(entries, map[string]any{
			"line": lineNo,
			"text": text,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取日志失败: %w", err)
	}

	jsonBytes, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("日志转 JSON 失败: %w", err)
	}

	meta := map[string]any{
		"lines":          lineNo,
		"nonblank_lines": len(entries),
	}
	return newParsedDoc(string(jsonBytes), meta, RawDocData), nil
}
