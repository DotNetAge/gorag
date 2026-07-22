package indexer

import (
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/query"
)

// TestBoostByKeywords 验证关键词增强只提升匹配分片的排序，不剔除任何结果。
func TestBoostByKeywords(t *testing.T) {
	q := query.New("机器学习 并发")

	hit := &core.Hit{
		Score: 0.5,
		Chunks: []core.ChunkHit{
			{
				Chunk: &core.Chunk{
					ID:      "c1",
					Title:   "深入理解 goroutine",
					Summary: "介绍 Go 的并发模型",
					Tags:    []string{"并发", "Go"},
				},
				Score: 0.5,
			},
			{
				Chunk: &core.Chunk{
					ID:      "c2",
					Title:   "机器学习与并发",
					Summary: "监督学习与非监督学习概述，以及并发实现",
					Tags:    []string{"机器学习", "并发"},
				},
				Score: 0.48,
			},
			{
				Chunk: &core.Chunk{
					ID:      "c3",
					Title:   "字符串处理",
					Summary: "常见字符串操作函数",
					Tags:    []string{"基础"},
				},
				Score: 0.3,
			},
		},
	}

	boostByKeywords(hit, q)

	if len(hit.Chunks) != 3 {
		t.Fatalf("增强后不应丢失任何分片，期望 3 个，实际 %d", len(hit.Chunks))
	}

	// c2 同时命中 "机器学习" 和 "并发"，增强后应超过 c1
	if hit.Chunks[0].ID != "c2" {
		t.Errorf("排序后第一个分片期望 c2，实际 %s", hit.Chunks[0].ID)
	}

	// c1 命中 "并发" tag，应排在 c3 之前
	if hit.Chunks[2].ID != "c3" {
		t.Errorf("排序后最后一个分片期望 c3，实际 %s", hit.Chunks[2].ID)
	}

	// c3 未命中任何关键词，分数应保持不变
	if hit.Chunks[2].Score != 0.3 {
		t.Errorf("c3 分数应保持 0.3，实际 %f", hit.Chunks[2].Score)
	}

	// c2 增强后分数 = 0.48 * (1 + 0.15*2) = 0.624
	// c1 增强后分数 = 0.50 * (1 + 0.15)   = 0.575
	if hit.Chunks[0].Score <= hit.Chunks[1].Score {
		t.Errorf("c2 增强后分数应高于 c1，c2=%f, c1=%f", hit.Chunks[0].Score, hit.Chunks[1].Score)
	}

	// Hit.Score 应保持融合后的综合分数不变，不因 chunk 重排而被覆盖
	if hit.Score != 0.5 {
		t.Errorf("Hit.Score 应保持原始融合分数 0.5，实际 %f", hit.Score)
	}
}

// TestBoostByKeywords_NoKeywords 验证无关键词时不改变结果。
func TestBoostByKeywords_NoKeywords(t *testing.T) {
	q := query.New("的 是") // 只有停用词

	hit := &core.Hit{
		Chunks: []core.ChunkHit{
			{
				Chunk: &core.Chunk{ID: "c1", Title: "标题", Tags: []string{"标签"}},
				Score: 0.5,
			},
		},
	}

	boostByKeywords(hit, q)

	if hit.Chunks[0].Score != 0.5 {
		t.Errorf("无关键词时不应改变分数，实际 %f", hit.Chunks[0].Score)
	}
}

// TestCountKeywordMatches 验证关键词命中统计。
func TestCountKeywordMatches(t *testing.T) {
	ch := &core.ChunkHit{
		Chunk: &core.Chunk{
			Title:   "Go 并发编程",
			Summary: "介绍 goroutine 和 channel",
			Tags:    []string{"Go", "并发"},
		},
	}

	keywords := []string{"go", "并发", "channel", "不相关"}
	count := countKeywordMatches(ch, keywords)

	if count != 3 {
		t.Errorf("期望命中 3 个关键词，实际 %d", count)
	}
}

// TestKeywordInTags 验证标签完全匹配（忽略大小写）。
func TestKeywordInTags(t *testing.T) {
	tags := []string{"Go", "RAG", "并发"}
	if !keywordInTags(tags, "go") {
		t.Error("应忽略大小写匹配 go")
	}
	if keywordInTags(tags, "Golang") {
		t.Error("不应部分匹配 Golang")
	}
}
