package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Hit 测试：适配容器结构（持有 Chunks/Nodes/Edges 三类命中数据）
// 设计对称性：StructuredDoc 是索引过程容器，Hit 是检索过程容器

func TestHitWithChunks(t *testing.T) {
	// 测试 Hit 容器结构：持有 ChunkHit 切片
	chunk1 := &Chunk{
		ID:      "test-chunk-1",
		DocID:   "doc-123",
		Title:   "第一章 概述",
		Summary: "本节介绍 RAG 系统的核心设计",
		Content: "这是测试内容",
		Metadata: map[string]any{
			"author":    "张三",
			"tags":      []string{"RAG", "向量检索"},
			"parent_id": "parent-chunk-0",
			"region_id": "region-001",
		},
	}
	chunk2 := &Chunk{
		ID:      "test-chunk-2",
		DocID:   "doc-123",
		Title:   "第二章 设计",
		Summary: "本节介绍系统设计",
		Content: "设计内容",
	}

	hit := Hit{
		Score: 0.95,
		Chunks: []ChunkHit{
			{Chunk: chunk1, Score: 0.95},
			{Chunk: chunk2, Score: 0.85},
		},
	}

	// 验证基本字段
	assert.Equal(t, float32(0.95), hit.Score)
	assert.Len(t, hit.Chunks, 2)

	// 验证嵌入式访问(无间接:hit.Chunks[i].Content 而非 hit.Chunks[i].Chunk.Content)
	assert.Equal(t, "test-chunk-1", hit.Chunks[0].ID)
	assert.Equal(t, "这是测试内容", hit.Chunks[0].Content)
	assert.Equal(t, "doc-123", hit.Chunks[0].DocID)
	assert.Equal(t, "第一章 概述", hit.Chunks[0].Title)
	assert.Equal(t, "本节介绍 RAG 系统的核心设计", hit.Chunks[0].Summary)
	assert.Equal(t, float32(0.95), hit.Chunks[0].Score)

	// 验证 Metadata 仍可通过嵌入的 *Chunk 访问
	assert.NotNil(t, hit.Chunks[0].Metadata)
	assert.Equal(t, "张三", hit.Chunks[0].Metadata["author"])
	tags, ok := hit.Chunks[0].Metadata["tags"].([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"RAG", "向量检索"}, tags)
	assert.Equal(t, "parent-chunk-0", hit.Chunks[0].Metadata["parent_id"])
	assert.Equal(t, "region-001", hit.Chunks[0].Metadata["region_id"])

	// 验证 JSON 序列化(嵌入式结构 JSON 序列化自然扁平化)
	data, err := json.Marshal(hit)
	assert.NoError(t, err)
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"chunks"`)
	assert.Contains(t, jsonStr, `"score"`)
	assert.Contains(t, jsonStr, `"title"`)
	assert.Contains(t, jsonStr, `"summary"`)
	assert.Contains(t, jsonStr, `"content"`)
	// Query 字段应被忽略(json:"-" 标签)
	assert.NotContains(t, jsonStr, `"query"`)

	// 反序列化验证
	var hit2 Hit
	err = json.Unmarshal(data, &hit2)
	assert.NoError(t, err)
	assert.Equal(t, hit.Score, hit2.Score)
	assert.Len(t, hit2.Chunks, 2)
	assert.Equal(t, hit.Chunks[0].ID, hit2.Chunks[0].ID)
	assert.Equal(t, hit.Chunks[0].Content, hit2.Chunks[0].Content)
}

func TestHitWithNodesAndEdges(t *testing.T) {
	// 测试 Hit 容器结构：同时持有 Nodes/Edges（图命中的场景）
	node1 := &Node{
		ID:     "node-1",
		Labels: []string{"Person"},
		Name:   "张三",
		Properties: map[string]any{
			"confidence": 0.9,
		},
		SourceChunkIDs: []string{"chunk-1"},
		SourceDocIDs:   []string{"doc-1"},
	}
	edge1 := &Edge{
		ID:        "edge-1",
		Type:      "WORKS_FOR",
		Source:    "node-1",
		Target:    "node-2",
		Predicate: "就职于",
		Properties: map[string]any{
			"confidence": 0.85,
		},
		SourceChunkIDs: []string{"chunk-1"},
		SourceDocIDs:   []string{"doc-1"},
	}

	hit := Hit{
		Score: 0.8,
		Nodes: []NodeHit{
			{Node: node1, Score: 0.9},
		},
		Edges: []EdgeHit{
			{Edge: edge1, Score: 0.85},
		},
	}

	// 验证嵌入式访问
	assert.Equal(t, "node-1", hit.Nodes[0].ID)
	assert.Equal(t, "张三", hit.Nodes[0].Name)
	assert.Equal(t, []string{"Person"}, hit.Nodes[0].Labels)
	assert.Equal(t, float32(0.9), hit.Nodes[0].Score)

	assert.Equal(t, "edge-1", hit.Edges[0].ID)
	assert.Equal(t, "WORKS_FOR", hit.Edges[0].Type)
	assert.Equal(t, "就职于", hit.Edges[0].Predicate)
	assert.Equal(t, float32(0.85), hit.Edges[0].Score)

	// 验证 JSON 序列化
	data, err := json.Marshal(hit)
	assert.NoError(t, err)
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"nodes"`)
	assert.Contains(t, jsonStr, `"edges"`)
	assert.Contains(t, jsonStr, `"name":"张三"`)
	assert.Contains(t, jsonStr, `"type":"WORKS_FOR"`)
}

func TestHitEmpty(t *testing.T) {
	// 测试空 Hit 的序列化(omitempty 应让 chunks/nodes/edges 字段被省略)
	hit := Hit{}

	data, err := json.Marshal(hit)
	assert.NoError(t, err)
	jsonStr := string(data)
	// 空切片应被 omitempty 省略
	assert.NotContains(t, jsonStr, `"chunks"`)
	assert.NotContains(t, jsonStr, `"nodes"`)
	assert.NotContains(t, jsonStr, `"edges"`)
	// score 应存在(0 值)
	assert.Contains(t, jsonStr, `"score"`)
}

func TestHitMixedContent(t *testing.T) {
	// 测试 HyperIndexer.Search 的双线融合场景:
	// Chunks 来自语义线,Nodes/Edges 来自关系线
	chunk := &Chunk{
		ID:      "chunk-mixed",
		DocID:   "doc-1",
		Content: "混合检索的内容",
	}
	node := &Node{
		ID:     "node-mixed",
		Labels: []string{"Concept"},
		Name:   "混合检索",
	}

	hit := Hit{
		Score: 0.75,
		Chunks: []ChunkHit{
			{Chunk: chunk, Score: 0.9},
		},
		Nodes: []NodeHit{
			{Node: node, Score: 0.6},
		},
	}

	// 验证三类数据共存
	assert.Len(t, hit.Chunks, 1)
	assert.Len(t, hit.Nodes, 1)
	assert.Nil(t, hit.Edges)

	// 验证分数独立:ChunkHit.Score 是分片级分数,Hit.Score 是综合分
	assert.Equal(t, float32(0.75), hit.Score)
	assert.Equal(t, float32(0.9), hit.Chunks[0].Score)
	assert.NotEqual(t, hit.Score, hit.Chunks[0].Score)
}
