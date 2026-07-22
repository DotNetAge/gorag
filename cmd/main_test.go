package main

import (
	"reflect"
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
)

// TestSplitQueryGroups 验证查询字符串按 | 拆分为多个关键字组。
func TestSplitQueryGroups(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"a | b | c", []string{"a", "b", "c"}},
		{"machine learning | neural networks", []string{"machine learning", "neural networks"}},
		{"  single  ", []string{"single"}},
		{"a |  | c", []string{"a", "c"}},
	}

	for _, c := range cases {
		got := splitQueryGroups(c.input)
		if !reflect.DeepEqual(got, c.expected) {
			t.Errorf("splitQueryGroups(%q) 期望 %v，实际 %v", c.input, c.expected, got)
		}
	}
}

// TestTruncateString 验证字符串截断。
func TestTruncateString(t *testing.T) {
	if got := truncateString("hello", 10); got != "hello" {
		t.Errorf("未超长时不应截断，实际 %q", got)
	}
	if got := truncateString("hello world", 5); got != "hello..." {
		t.Errorf("超长时应截断，实际 %q", got)
	}
}

// TestFormatChunkItems 验证 Chunk 到 chunkItem 的格式化。
func TestFormatChunkItems(t *testing.T) {
	chunks := []core.Chunk{
		{ID: "c1", Title: "标题一", Summary: "摘要一", Tags: []string{"Go", "并发"}},
		{ID: "c2", Title: "标题二", Summary: "摘要二", Tags: nil},
	}

	items := formatChunkItems(chunks)
	if len(items) != 2 {
		t.Fatalf("期望 2 条，实际 %d", len(items))
	}
	if items[0].ID != "c1" || items[0].Title != "标题一" || items[0].Summary != "摘要一" {
		t.Errorf("第一条格式化错误: %+v", items[0])
	}
	if !reflect.DeepEqual(items[0].Tags, []string{"Go", "并发"}) {
		t.Errorf("第一条 tags 错误: %v", items[0].Tags)
	}
	if items[1].Tags != nil {
		t.Errorf("nil tags 应保持 nil，实际 %v", items[1].Tags)
	}
}
