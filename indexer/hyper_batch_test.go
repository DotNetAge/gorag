package indexer

import (
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
)

// TestSplitChunksBySize 验证按 content 字符数分批工具函数的正确性。
func TestSplitChunksBySize(t *testing.T) {
	mk := func(content string) core.Chunk {
		return core.Chunk{ID: content, Content: content}
	}

	t.Run("空列表返回单批", func(t *testing.T) {
		got := splitChunksBySize(nil, 100)
		if len(got) != 1 || len(got[0]) != 0 {
			t.Fatalf("期望 [[]]，实际 %v", got)
		}
	})

	t.Run("单个 chunk 不足阈值", func(t *testing.T) {
		chunks := []core.Chunk{mk("hello")}
		got := splitChunksBySize(chunks, 100)
		if len(got) != 1 || len(got[0]) != 1 {
			t.Fatalf("期望单批 1 个 chunk，实际 %v", got)
		}
	})

	t.Run("累加超出阈值自动 flush", func(t *testing.T) {
		// 3 个 chunk，每个 4 字符，阈值 10 字符 → 期望 2 批
		chunks := []core.Chunk{mk("abcd"), mk("efgh"), mk("ijkl")}
		got := splitChunksBySize(chunks, 10)
		if len(got) != 2 {
			t.Fatalf("期望 2 批，实际 %d 批", len(got))
		}
		if len(got[0]) != 2 {
			t.Fatalf("期望第 1 批 2 个 chunk，实际 %d 个", len(got[0]))
		}
		if len(got[1]) != 1 {
			t.Fatalf("期望第 2 批 1 个 chunk，实际 %d 个", len(got[1]))
		}
	})

	t.Run("单 chunk 已超阈值独立成批", func(t *testing.T) {
		// chunk 1 字符，chunk 2 超大，chunk 3 字符
		big := strings.Repeat("x", 100)
		chunks := []core.Chunk{mk("a"), mk(big), mk("b")}
		got := splitChunksBySize(chunks, 20)
		if len(got) != 3 {
			t.Fatalf("期望 3 批，实际 %d 批", len(got))
		}
		if len(got[0]) != 1 || got[0][0].Content != "a" {
			t.Fatalf("第 1 批应为单 chunk 'a'，实际 %v", got[0])
		}
		if len(got[1]) != 1 || len(got[1][0].Content) != 100 {
			t.Fatalf("第 2 批应为单大 chunk，实际 %v", got[1])
		}
		if len(got[2]) != 1 || got[2][0].Content != "b" {
			t.Fatalf("第 3 批应为单 chunk 'b'，实际 %v", got[2])
		}
	})

	t.Run("中文按 rune 计数", func(t *testing.T) {
		// 3 个 4 字中文 = 12 字符，阈值 10 → 2 批
		chunks := []core.Chunk{
			mk("一二三四"),
			mk("五六七八"),
			mk("九十十一"),
		}
		got := splitChunksBySize(chunks, 10)
		if len(got) != 2 {
			t.Fatalf("期望 2 批，实际 %d 批", len(got))
		}
		if len(got[0]) != 2 {
			t.Fatalf("期望第 1 批 2 个 chunk，实际 %d 个", len(got[0]))
		}
		if len(got[1]) != 1 {
			t.Fatalf("期望第 2 批 1 个 chunk，实际 %d 个", len(got[1]))
		}
	})
}
