package result

import (
	"context"
	"fmt"
	"sort"
	"strings"

	goChatCore "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/core"
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

// Compress 先按分数排序取 top N，再对每条 ChunkHit 的 Content 调用 LLM 压缩。
//
// 入参 hit 不会被修改，返回新的 *Hit（Chunks 已压缩，Nodes/Edges 保持不变）。
// 压缩后的 ChunkHit 会创建新的 Chunk 副本（不共享原 Chunk 指针），避免副作用。
func Compress(limit int, llm goChatCore.Client, hit *core.Hit) (*core.Hit, error) {
	if hit == nil {
		return nil, nil
	}
	if len(hit.Chunks) == 0 {
		return cloneHit(hit), nil
	}

	// 按分数排序并截断到 limit
	sorted := make([]core.ChunkHit, len(hit.Chunks))
	copy(sorted, hit.Chunks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	// 逐条调用 LLM 压缩
	result := make([]core.ChunkHit, len(sorted))
	for i, ch := range sorted {
		compressed, err := compressChunkHit(llm, ch)
		if err != nil {
			return nil, fmt.Errorf("压缩 Chunk %s 失败: %w", ch.ID, err)
		}
		result[i] = compressed
	}

	// 构建压缩后的 Hit（保留原 Query/Score/Nodes/Edges）
	return &core.Hit{
		Query:  hit.Query,
		Score:  hit.Score,
		Chunks: result,
		Nodes:  hit.Nodes,
		Edges:  hit.Edges,
	}, nil
}

// compressChunkHit 对单条 ChunkHit 的 Content 调用 LLM 进行压缩。
//
// 创建新的 Chunk 副本（避免修改原 Chunk），仅替换 Content 字段。
// LLM 返回空时保留原文。
func compressChunkHit(llm goChatCore.Client, ch core.ChunkHit) (core.ChunkHit, error) {
	if ch.Chunk == nil {
		return ch, nil
	}

	userMsg := `请压缩以下文本，仅保留关键信息：

要求：
1. 保留所有事实性数据：数字、日期、人名、专有名词
2. 删除重复表述、冗余修饰词和无关联接词
3. 保持原文的逻辑结构和因果关系
4. 不得编造原文中不存在的信息
5. 输出为简洁的完整段落，不要使用列表形式

待压缩文本：
` + ch.Content

	messages := []goChatCore.Message{
		goChatCore.NewTextMessage("user", userMsg),
	}

	resp, err := llm.Chat(context.Background(), messages,
		goChatCore.WithTemperature(0.1),
	)
	if err != nil {
		return ch, err
	}

	compressedContent := strings.TrimSpace(resp.Content)
	if compressedContent == "" {
		return ch, nil // LLM 返回空则保留原文
	}

	// 创建 Chunk 副本，仅替换 Content（避免修改原 Chunk 指针）
	newChunk := *ch.Chunk
	newChunk.Content = compressedContent

	return core.ChunkHit{
		Chunk: &newChunk,
		Score: ch.Score,
	}, nil
}
