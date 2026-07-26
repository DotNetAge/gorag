// Package llm 提供基于 LLM 的文本增强工具。
//
// 设计要点：
//   - 与 chunker 包同层，独立开发，最后在集成层使用
//   - 所有 LLM 调用统一使用 gochat 客户端
//   - 工具包括：
//   - Summarizer：为 Chunk 生成更语义化的 Title/Summary
//   - Refiller：从预分块文本中提取实体并回填 Nodes/Edges
//   - 构造函数返回 error，所有必传参数进行非空检查
//   - 日志使用 gorag/logging 接口，Logger 通过构造函数注入
package llm

import "strings"

// normalizeLLMJSON 清洗 LLM 返回的 JSON 字符串。
//
// 处理项：
//   - 去除 markdown 代码块标记
//   - 自动替换中文标点为英文标点，避免 JSON 解析失败
//   - 删除 ] 之后的多余内容（LLM 可能在数组外附加文字）
//   - 在 } 与 { 之间插入逗号（数组元素间缺少逗号）
//   - 删除 ] 之前的多余逗号（末尾多逗号）
func normalizeLLMJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// 仅替换 JSON 结构所需标点；内容中的中文句号/分号保留原样。
	s = strings.ReplaceAll(s, "，", ",")
	s = strings.ReplaceAll(s, "：", ":")
	s = strings.ReplaceAll(s, "\u201c", "\"")
	s = strings.ReplaceAll(s, "\u201d", "\"")
	s = strings.ReplaceAll(s, "\u2018", "'")
	s = strings.ReplaceAll(s, "\u2019", "'")
	s = strings.ReplaceAll(s, "\u3010", "[")
	s = strings.ReplaceAll(s, "\u3011", "]")
	s = strings.ReplaceAll(s, "\uff08", "(")
	s = strings.ReplaceAll(s, "\uff09", ")")

	// 修复常见 LLM JSON 格式错误
	// 1. 删除 ] 之后的多余内容（仅当根是数组时）
	// 2. 如果数组中根本没有 ]，说明被截断，补充 }] 尝试闭合
	if strings.HasPrefix(s, "[") {
		if idx := strings.LastIndex(s, "]"); idx >= 0 {
			s = s[:idx+1]
		} else {
			// 数组被截断（连 ] 都没有），补充 }] 尝试闭合最后一个对象和数组
			s += "}]"
		}
	}
	// 3. 在 } 与 { 之间插入逗号（数组元素间缺逗号）
	s = strings.ReplaceAll(s, "}{", "},{")
	// 4. 删除 ] 之前的多余逗号
	s = strings.ReplaceAll(s, ",]", "]")

	return s
}
