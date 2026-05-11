package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHitWithMetadata(t *testing.T) {
	// 测试 Hit 结构体包含完整的元信息
	hit := Hit{
		ID:      "test-chunk-1",
		Score:   0.95,
		Content: "这是测试内容",
		DocID:   "doc-123",
		Metadata: map[string]any{
			"author": "张三",
			"tags":   []string{"RAG", "向量检索"},
		},
		ChunkMeta: ChunkMeta{
			Index:        0,
			StartPos:     100,
			EndPos:       250,
			HeadingLevel: 2,
			HeadingPath:  []string{"第一章", "1.1节"},
		},
	}

	// 验证基本字段
	assert.Equal(t, "test-chunk-1", hit.ID)
	assert.Equal(t, float32(0.95), hit.Score)
	assert.Equal(t, "这是测试内容", hit.Content)
	assert.Equal(t, "doc-123", hit.DocID)

	// 验证 Metadata
	assert.NotNil(t, hit.Metadata)
	assert.Equal(t, "张三", hit.Metadata["author"])
	tags, ok := hit.Metadata["tags"].([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"RAG", "向量检索"}, tags)

	// 验证 ChunkMeta
	assert.Equal(t, 0, hit.ChunkMeta.Index)
	assert.Equal(t, 100, hit.ChunkMeta.StartPos)
	assert.Equal(t, 250, hit.ChunkMeta.EndPos)
	assert.Equal(t, 2, hit.ChunkMeta.HeadingLevel)
	assert.Equal(t, []string{"第一章", "1.1节"}, hit.ChunkMeta.HeadingPath)

	// 验证 JSON 序列化
	data, err := json.Marshal(hit)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"metadata"`)
	assert.Contains(t, string(data), `"chunk_meta"`)

	// 反序列化验证
	var hit2 Hit
	err = json.Unmarshal(data, &hit2)
	assert.NoError(t, err)
	assert.Equal(t, hit.ID, hit2.ID)
	assert.Equal(t, hit.Score, hit2.Score)
	assert.NotNil(t, hit2.Metadata)
	assert.Equal(t, "张三", hit2.Metadata["author"])
	assert.Equal(t, 0, hit2.ChunkMeta.Index)
}

func TestHitWithNilMetadata(t *testing.T) {
	// 测试 Metadata 和 ChunkMeta 为 nil 的情况（如图搜索）
	hit := Hit{
		ID:      "graph-hit-1",
		Score:   0.85,
		Content: `{"entities":[],"relations":[]}`,
		DocID: "",
	}

	// 应该正常工作（nil 值会被序列化为 null）
	data, err := json.Marshal(hit)
	assert.NoError(t, err)
	// 验证 JSON 包含 metadata 和 chunk_meta 字段（即使是零值）
	assert.Contains(t, string(data), `"metadata"`)
	assert.Contains(t, string(data), `"chunk_meta"`)

	// 反序列化验证
	var hit2 Hit
	err = json.Unmarshal(data, &hit2)
	assert.NoError(t, err)
	assert.Equal(t, hit.ID, hit2.ID)
	assert.Nil(t, hit2.Metadata) // nil 应该保持为 nil
}

func TestFullTextSearchResultWithMetadata(t *testing.T) {
	// 测试 FullTextSearchResult 包含元信息
	result := FullTextSearchResult{
		ID:      "chunk-abc",
		Score:   0.92,
		DocID:   "doc-456",
		Content: "匹配的内容片段",
		Metadata: map[string]any{
			"source": "test.pdf",
		},
		ChunkMeta: ChunkMeta{
			Index:    5,
			StartPos: 500,
			EndPos:   750,
		},
	}

	assert.Equal(t, "chunk-abc", result.ID)
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, "test.pdf", result.Metadata["source"])
	assert.Equal(t, 5, result.ChunkMeta.Index)
}
