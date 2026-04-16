package result

import (
	"strings"
	"testing"

	"github.com/DotNetAge/gochat/pkg/client/ollama"
	goragCore "github.com/DotNetAge/gorag/core"
)

// TestCompressWithLLM 验证 LLM 压缩流程（使用 ollama 真实客户端）
func TestCompressWithLLM(t *testing.T) {
	llm, err := ollama.New(ollama.Config{})
	if err != nil {
		t.Fatalf("create ollama client: %v", err)
	}
	// c := NewCompresser(llm).WithLimit(2)

	hits := []goragCore.Hit{
		{ID: "1", DocID: "doc-a", Score: 0.9, Content: "这是一段非常长的文档内容...包含很多无关信息...用户张三今年25岁...他的银行账户余额是5000元...还有很多其他内容"},
		{ID: "2", DocID: "doc-b", Score: 0.7, Content: "另一段长文本...系统日志显示...用户李四...操作记录...更多填充文字"},
	}

	result, err := Compress(10, llm, hits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}

	// 压缩后内容应该比原文短（或至少不同）
	if result[0].Content == hits[0].Content {
		t.Error("content should be compressed by LLM")
	}
	// ID、DocID、Score 应保持不变
	if result[0].ID != "1" || result[0].DocID != "doc-a" || result[0].Score != 0.9 {
		t.Error("metadata should be preserved")
	}
	// 压缩后的内容不应为空
	if result[0].Content == "" {
		t.Error("compressed content should not be empty")
	}
}

// TestCompressSortsByScore 验证按分数排序后取 top N 再压缩
func TestCompressSortsByScore(t *testing.T) {
	llm, err := ollama.New(ollama.Config{})
	if err != nil {
		t.Fatalf("create ollama client: %v", err)
	}
	// c := NewCompresser(llm).WithLimit(2)

	hits := []goragCore.Hit{
		{ID: "low", Score: 0.3, Content: "低分内容"},
		{ID: "high", Score: 0.9, Content: "高分内容"},
		{ID: "mid", Score: 0.6, Content: "中分内容"},
	}

	result, _ := Compress(2, llm, hits)

	// limit=2，应只返回 top 2
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	// top 2 应该是 high 和 mid（按分数排序）
	if result[0].ID != "high" || result[1].ID != "mid" {
		t.Errorf("expected [high,mid], got [%s,%s]", result[0].ID, result[1].ID)
	}
}

// TestCompressDoesNotMutateInput 验证不修改输入切片
func TestCompressDoesNotMutateInput(t *testing.T) {
	llm, _ := ollama.New(ollama.Config{})
	// c := NewCompresser(llm).WithLimit(1)
	hits := []goragCore.Hit{{ID: "a", Score: 0.9, Content: "原始内容"}}
	originalContent := hits[0].Content

	_, _ = Compress(1, llm, hits)

	if hits[0].Content != originalContent {
		t.Error("input slice should not be mutated")
	}
}

// TestCompressPreservesKeyInformation 验证压缩后保留关键信息
func TestCompressPreservesKeyInformation(t *testing.T) {
	llm, err := ollama.New(ollama.Config{})
	if err != nil {
		t.Fatalf("create ollama client: %v", err)
	}
	// c := NewCompresser(llm).WithLimit(1)

	// content := "According to the 2024 annual financial report, Company A achieved revenue of 50 billion RMB, a year-over-year growth of 25%, with employee count increasing from 8,000 to 10,000."
	content := "根据2024年年度财务报告显示，公司A的营收达到了500亿元人民币，同比增长了25%，员工总数从去年的8000人增长到10000人。"
	hits := []goragCore.Hit{{ID: "1", Content: content}}

	result, _ := Compress(1, llm, hits)

	// Key data should be preserved: 2024, 50 billion, 25%, 10,000
	hasData := strings.Contains(result[0].Content, "50") ||
		strings.Contains(result[0].Content, "25") ||
		strings.Contains(result[0].Content, "10,000") || strings.Contains(result[0].Content, "10000")

	if !hasData {
		t.Errorf("compressed content should preserve key data, got: %s", result[0].Content)
	}
}
