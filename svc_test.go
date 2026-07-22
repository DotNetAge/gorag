package gorag

import (
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
)

// TestFoldReadmeIntoDirectories 验证 README.md 文件节点的摘要被折叠到父目录节点，且文件节点被移除。
func TestFoldReadmeIntoDirectories(t *testing.T) {
	root := &SourceTreeNode{Name: ".", IsDir: true}
	subDir := &SourceTreeNode{Name: "docs", IsDir: true}
	readmeFile := &SourceTreeNode{
		Name: "README.md",
		Chunks: []SourceChunkNode{
			{Title: "docs", Summary: "文档目录摘要。"},
			{Title: "intro", Summary: "简介摘要。"},
		},
	}
	otherFile := &SourceTreeNode{Name: "other.md"}

	subDir.Children = []*SourceTreeNode{readmeFile, otherFile}
	root.Children = []*SourceTreeNode{subDir}

	foldReadmeIntoDirectories(root)

	if subDir.Summary == "" {
		t.Fatalf("折叠后目录 Summary 不应为空")
	}
	if subDir.Summary != "文档目录摘要。；简介摘要。" {
		t.Errorf("目录 Summary 期望 %q，实际 %q", "文档目录摘要。；简介摘要。", subDir.Summary)
	}
	if len(subDir.Children) != 1 || subDir.Children[0].Name != "other.md" {
		t.Errorf("README.md 节点应被移除，剩余子节点: %v", subDir.Children)
	}
}

// TestCollectReadmeSummary 验证 README 摘要收集会去重并截断。
func TestCollectReadmeSummary(t *testing.T) {
	node := &SourceTreeNode{
		Name: "README.md",
		Chunks: []SourceChunkNode{
			{Summary: "摘要一。"},
			{Summary: "摘要一。"},
			{Summary: "摘要二。"},
		},
	}

	summary := collectReadmeSummary(node)
	if summary != "摘要一。；摘要二。" {
		t.Errorf("摘要去重后期望 %q，实际 %q", "摘要一。；摘要二。", summary)
	}
}

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
