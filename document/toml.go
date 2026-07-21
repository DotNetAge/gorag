package document

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/pelletier/go-toml/v2"
)

// ParseTOML 读取 TOML 文件并归一化为 dataDoc（内容为 JSON 字符串）。
//
// 归一化策略：
//   - 将 TOML 解析为通用数据结构，再序列化为紧凑 JSON 字符串
//   - 输出 JSON 字符串（紧凑格式，无缩进）
//   - 元数据包含 source_format 标记
func ParseTOML(r io.Reader) (RawDoc, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("TOML 输入为空")
	}

	// 解析 TOML 为通用数据结构
	var parsed any
	if err := toml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("无效的 TOML: %w", err)
	}

	// 序列化为紧凑 JSON
	jsonBytes, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("TOML 转 JSON 失败: %w", err)
	}

	meta := map[string]any{
		"source_format": "toml",
	}
	return newParsedDoc(string(jsonBytes), meta, RawDocData), nil
}
