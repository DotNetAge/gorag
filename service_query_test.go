package gorag

import (
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
)

// TestSourceHasPrefix 验证 source 路径前缀过滤。
func TestSourceHasPrefix(t *testing.T) {
	if sourceHasPrefix("/home/user/docs/file.md", "./docs") {
		t.Errorf("相对路径 filter 不应匹配绝对路径 source")
	}
	if !sourceHasPrefix("/home/user/docs/file.md", "/home/user/docs") {
		t.Errorf("绝对路径前缀应匹配")
	}
	if sourceHasPrefix("/home/user/docs/file.md", "/home/user/other") {
		t.Errorf("非前缀路径不应匹配")
	}
	if sourceHasPrefix("", "/home/user") {
		t.Errorf("空 source 不应匹配")
	}
	// 边界检查：/home/user/doc 不应匹配 /home/user/docs/file.md
	if sourceHasPrefix("/home/user/docs/file.md", "/home/user/doc") {
		t.Errorf("部分目录名前缀不应匹配")
	}
	// 精确目录边界：/home/user/docs/ 应匹配 /home/user/docs/file.md
	if !sourceHasPrefix("/home/user/docs/file.md", "/home/user/docs/") {
		t.Errorf("已带分隔符的目录前缀应匹配")
	}
}

// TestFilterHitBySourcePrefix 验证命中结果按 source 前缀过滤。
func TestFilterHitBySourcePrefix(t *testing.T) {
	hit := &core.Hit{
		Chunks: []core.ChunkHit{
			{Chunk: &core.Chunk{ID: "a", Source: "/home/user/docs/a.md"}, Score: 0.9},
			{Chunk: &core.Chunk{ID: "b", Source: "/home/user/other/b.md"}, Score: 0.8},
		},
	}
	filtered := filterHitBySourcePrefix(hit, "/home/user/docs")
	if len(filtered.Chunks) != 1 || filtered.Chunks[0].ID != "a" {
		t.Errorf("过滤后期望保留 a，实际 %v", filtered.Chunks)
	}
}

// TestTopChunkScore 验证最高分提取。
func TestTopChunkScore(t *testing.T) {
	hits := []core.ChunkHit{
		{Score: 0.5},
		{Score: 0.9},
		{Score: 0.7},
	}
	if got := topChunkScore(hits); got != 0.9 {
		t.Errorf("最高分期望 0.9，实际 %v", got)
	}
	if got := topChunkScore(nil); got != 0 {
		t.Errorf("空切片期望 0，实际 %v", got)
	}
}
