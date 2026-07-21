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
func normalizeLLMJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// 仅替换 JSON 结构所需标点；内容中的中文句号/分号保留原样。
	s = strings.ReplaceAll(s, "，", ",")
	s = strings.ReplaceAll(s, "：", ":")
	s = strings.ReplaceAll(s, "“", "\"")
	s = strings.ReplaceAll(s, "”", "\"")
	s = strings.ReplaceAll(s, "‘", "'")
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.ReplaceAll(s, "【", "[")
	s = strings.ReplaceAll(s, "】", "]")
	s = strings.ReplaceAll(s, "（", "(")
	s = strings.ReplaceAll(s, "）", ")")
	return s
}
