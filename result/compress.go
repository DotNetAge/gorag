package result

import (
	"context"
	"fmt"
	"sort"
	"strings"

	goChatCore "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
)

// Compresser 实现基于 LLM 的结果压缩（LLM-based Context Compression）。
//
// 与简单的 TopK 截断不同，结果压缩利用 LLM 对检索到的文档内容做信息密度提升，
// 在保留关键信息的前提下大幅减少 token 数量。
//
// 典型应用场景（RAG 理论中的 Result Compression）：
//   - 检索到的 chunk 包含大量填充文本，需要提取关键信息
//   - 多个 chunk 存在内容重叠，需要去重合并
//   - 总 token 超出 LLM 上下文窗口限制时，需要智能压缩而非粗暴截断

// Compress 先按分数排序取 top N，再对每条 Content 调用 LLM 压缩。
func Compress(limit int, llm goChatCore.Client, hits []core.Hit) ([]core.Hit, error) {
	if len(hits) == 0 {
		return hits, nil
	}

	// 按分数排序并截断到 limit
	sorted := make([]core.Hit, len(hits))
	copy(sorted, hits)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	// 逐条调用 LLM 压缩
	result := make([]core.Hit, len(sorted))
	for i, hit := range sorted {
		compressed, err := compressOne(llm, hit)
		if err != nil {
			return nil, fmt.Errorf("compress hit %s: %w", hit.ID, err)
		}
		result[i] = compressed
	}

	return result, nil
}

// compressOne 对单条 Hit 的 Content 调用 LLM 进行压缩。
func compressOne(llm goChatCore.Client, hit core.Hit) (core.Hit, error) {
	userMsg := `Compress the following text by extracting and preserving only essential information:

Requirements:
1. Preserve all factual data: numbers, dates, names, proper nouns
2. Remove repetitive statements, redundant modifiers, and irrelevant transitions
3. Maintain original logical structure and causal relationships
4. Do not fabricate information not present in the original text
5. Output as a concise complete paragraph, not a bulleted list

Text to compress:
` + hit.Content

	messages := []goChatCore.Message{
		goChatCore.NewTextMessage("user", userMsg),
	}

	resp, err := llm.Chat(context.Background(), messages,
		goChatCore.WithTemperature(0.1),
		goChatCore.WithMaxTokens(1024),
	)
	if err != nil {
		return hit, err
	}

	compressedContent := strings.TrimSpace(resp.Content)
	if compressedContent == "" {
		return hit, nil // LLM 返回空则保留原文
	}

	hit.Content = compressedContent
	return hit, nil
}
