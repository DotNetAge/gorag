package indexer

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/gorag/core"
)

// 默认模型最大上下文窗口（token 数）
const defaultMaxTokens = 128000

// 输入占比：输入 token 不超过上下文窗口的 75%，为输出预留空间
const inputTokenRatio = 0.75

// 安全边际系数：content 的最大 token 数不超过切片上限的 80%
const contentSafetyMargin = 0.8

// parseIndexData 从 LLM 响应内容中解析 JSON。
// 自动兜底处理 markdown 代码块包裹。
func parseIndexData(content string) (*IndexData, error) {
	jsonStr := extractJSONBlock(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON content found in LLM response")
	}

	var data IndexData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	return &data, nil
}

// extractJSONBlock 从 LLM 响应中提取 JSON 字符串。
// 支持直接 JSON、```json ... ``` 和 ``` ... ``` 三种格式。
func extractJSONBlock(content string) string {
	var v any
	if err := json.Unmarshal([]byte(content), &v); err == nil {
		return content
	}

	start := 0
	if idx := indexOf(content, "```json"); idx >= 0 {
		start = idx + 7
	} else if idx := indexOf(content, "```"); idx >= 0 {
		start = idx + 3
	}

	if start == 0 {
		return content
	}

	end := len(content)
	if idx := indexOf(content[start:], "```"); idx >= 0 {
		end = start + idx
	}

	return content[start:end]
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Ensure implementation of core.ChunkIndexer interface
var _ core.ChunkIndexer = (*LLMIndexer)(nil)
