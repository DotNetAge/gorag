package native

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
)

// mockEmbedder 确定性字符级词袋向量：每个字符哈希到固定维度的一个位置并累加，
// 最后归一化。相同或包含关系的文本相似度高，语义无关文本接近正交，
// 足以验证索引与检索逻辑（不验证 embedding 质量）。
type mockEmbedder struct {
	dim int
}

func newMockEmbedder() *mockEmbedder { return &mockEmbedder{dim: 64} }

func (m *mockEmbedder) CalcText(text string) (*core.Vector, error) {
	v := make([]float32, m.dim)
	for _, r := range text {
		if r == ' ' {
			continue
		}
		sum := sha256.Sum256([]byte(string(r)))
		idx := int(binary.BigEndian.Uint16(sum[:2])) % m.dim
		v[idx]++
	}
	norm := float32(0)
	for _, x := range v {
		norm += x * x
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range v {
			v[i] /= norm
		}
	}
	return &core.Vector{Values: v}, nil
}

func (m *mockEmbedder) CalcImage([]byte) (*core.Vector, error) {
	return nil, fmt.Errorf("mock 不支持图片")
}

func (m *mockEmbedder) Dim() int          { return m.dim }
func (m *mockEmbedder) Multimoding() bool { return false }

// newTestIndexer 在临时目录创建索引器。
func newTestIndexer(t *testing.T) *SegIndexer {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "native.db")
	seg, err := NewSegIndexer(dbPath, newMockEmbedder())
	if err != nil {
		t.Fatalf("创建索引器失败: %v", err)
	}
	return seg
}

// TestAddAndSearchByMainValue 主值索引与检索闭环。
func TestAddAndSearchByMainValue(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	err := seg.Add(t.Context(), "skill", "websearch", map[string]string{
		"name":        "websearch",
		"title":       "网页搜索",
		"description": "搜索网络上内容",
	})
	if err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	// 查询与主值完全一致时必然命中该条目；主值与 meta["name"] 值相同时
	// 主值记录与 name 子键记录分数并列，Field 为 "" 或 "name" 均属正确
	hits, err := seg.Search(t.Context(), "skill", "websearch", 1)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("期望命中 1 条，实际 %d 条", len(hits))
	}
	hit := hits[0]
	if hit.Value != "websearch" {
		t.Errorf("Value 期望 websearch，实际 %q", hit.Value)
	}
	if hit.Meta["title"] != "网页搜索" {
		t.Errorf("Meta[title] 期望 网页搜索，实际 %q", hit.Meta["title"])
	}
	if hit.Field != "" && hit.Field != "name" {
		t.Errorf("Field 应为空或 name（两记录分数并列），实际 %q", hit.Field)
	}
}

// TestSubkeyHitReturnsFullEntry 子键命中时返回完整条目（主值 + 元数据）。
func TestSubkeyHitReturnsFullEntry(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	err := seg.Add(t.Context(), "skill", "websearch", map[string]string{
		"title":       "网页搜索",
		"description": "搜索网络上内容",
	})
	if err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	// "网页" 只出现在 title 子键中，应通过 title 命中并取回完整条目
	hits, err := seg.Search(t.Context(), "skill", "网页", 1)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("期望命中 1 条，实际 %d 条", len(hits))
	}
	hit := hits[0]
	if hit.Value != "websearch" {
		t.Errorf("Value 期望 websearch，实际 %q", hit.Value)
	}
	if hit.Field != "title" {
		t.Errorf("Field 期望 title，实际 %q", hit.Field)
	}
	if hit.Meta["description"] != "搜索网络上内容" {
		t.Errorf("Meta[description] 期望完整元数据，实际 %v", hit.Meta)
	}
}

// TestAddSameValueOverwrites 同值重复 Add 视为更新索引。
func TestAddSameValueOverwrites(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	if err := seg.Add(t.Context(), "skill", "websearch", map[string]string{"title": "旧标题"}); err != nil {
		t.Fatalf("首次 Add 失败: %v", err)
	}
	if err := seg.Add(t.Context(), "skill", "websearch", map[string]string{"title": "新标题"}); err != nil {
		t.Fatalf("重复 Add 失败: %v", err)
	}

	hits, err := seg.Search(t.Context(), "skill", "websearch", 10)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("同值条目应唯一，实际命中 %d 条", len(hits))
	}
	if hits[0].Meta["title"] != "新标题" {
		t.Errorf("重复 Add 应覆盖元数据，期望 新标题，实际 %q", hits[0].Meta["title"])
	}
}

// TestSubkeyValueCollisionOverwrites 不同条目子键值相同时视为同一个东西，后写覆盖。
func TestSubkeyValueCollisionOverwrites(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	if err := seg.Add(t.Context(), "skill", "entry-x", map[string]string{"title": "相同标题"}); err != nil {
		t.Fatalf("Add entry-x 失败: %v", err)
	}
	if err := seg.Add(t.Context(), "skill", "entry-y", map[string]string{"title": "相同标题"}); err != nil {
		t.Fatalf("Add entry-y 失败: %v", err)
	}

	hits, err := seg.Search(t.Context(), "skill:title", "相同标题", 10)
	if err != nil {
		t.Fatalf("定向子键 Search 失败: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("子键值冲突应覆盖为单条，实际 %d 条", len(hits))
	}
	if hits[0].Value != "entry-y" {
		t.Errorf("后写应覆盖先写，期望 entry-y，实际 %q", hits[0].Value)
	}
}

// TestSearchSubkeyDirected 定向子键检索只命中该字段的记录。
func TestSearchSubkeyDirected(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	if err := seg.Add(t.Context(), "skill", "websearch", map[string]string{
		"title":       "网页搜索",
		"description": "搜索网络上内容",
	}); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	hits, err := seg.Search(t.Context(), "skill:title", "网页", 5)
	if err != nil {
		t.Fatalf("定向 Search 失败: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("期望命中 1 条，实际 %d 条", len(hits))
	}
	if hits[0].Field != "title" {
		t.Errorf("定向检索 Field 期望 title，实际 %q", hits[0].Field)
	}

	// 定向 description 检索语义无关词：族内仅一条 description 记录，
	// 近邻搜索无相似度阈值，零分记录仍会返回，但只能命中该记录所属条目
	hits, err = seg.Search(t.Context(), "skill:description", "绘画", 5)
	if err != nil {
		t.Fatalf("定向 Search 失败: %v", err)
	}
	if len(hits) > 1 {
		t.Errorf("定向 description 检索至多命中 1 条，实际 %d 条", len(hits))
	}
	for _, h := range hits {
		if h.Value != "websearch" || h.Field != "description" {
			t.Errorf("定向 description 不应命中其他条目或字段: %+v", h)
		}
	}
}

// TestSearchTopKAndOrder 多条目检索时 topK 截断且按分数降序。
func TestSearchTopKAndOrder(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	entries := []struct{ value, title string }{
		{"tool-alpha", "阿尔法工具"},
		{"tool-beta", "贝塔搜索"},
		{"tool-gamma", "伽马搜索"},
	}
	for _, e := range entries {
		if err := seg.Add(t.Context(), "tool", e.value, map[string]string{"title": e.title}); err != nil {
			t.Fatalf("Add %s 失败: %v", e.value, err)
		}
	}

	hits, err := seg.Search(t.Context(), "tool", "搜索", 2)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("topK=2 期望 2 条，实际 %d 条", len(hits))
	}
	for i := 1; i < len(hits); i++ {
		if hits[i].Score > hits[i-1].Score {
			t.Errorf("结果未按分数降序: [%d]=%f > [%d]=%f", i, hits[i].Score, i-1, hits[i-1].Score)
		}
	}
	for _, h := range hits {
		if h.Value != "tool-beta" && h.Value != "tool-gamma" {
			t.Errorf("命中了不相关的条目: %s", h.Value)
		}
	}
}

// TestSearchEmptyFamily 检索不存在的族返回空结果而非报错。
func TestSearchEmptyFamily(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	hits, err := seg.Search(t.Context(), "nothing", "任意", 1)
	if err != nil {
		t.Fatalf("空族 Search 不应报错: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("空族期望 0 条命中，实际 %d 条", len(hits))
	}
}

// TestPersistenceAcrossClose 持久化闭环：Close 后重开同库数据仍在。
func TestPersistenceAcrossClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "native.db")
	emb := newMockEmbedder()

	seg, err := NewSegIndexer(dbPath, emb)
	if err != nil {
		t.Fatalf("创建索引器失败: %v", err)
	}
	if err = seg.Add(t.Context(), "skill", "websearch", map[string]string{"title": "网页搜索"}); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}
	if err = seg.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}

	seg2, err := NewSegIndexer(dbPath, emb)
	if err != nil {
		t.Fatalf("重开索引器失败: %v", err)
	}
	defer seg2.Close()

	hits, err := seg2.Search(t.Context(), "skill", "网页", 1)
	if err != nil {
		t.Fatalf("重开后 Search 失败: %v", err)
	}
	if len(hits) != 1 || hits[0].Value != "websearch" {
		t.Errorf("重开后数据丢失: hits=%v", hits)
	}
}

// TestList 列出族内条目：仅主记录、去重、limit 截断。
func TestList(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	entries := []struct{ value, title string }{
		{"tool-alpha", "阿尔法工具"},
		{"tool-beta", "贝塔工具"},
	}
	for _, e := range entries {
		if err := seg.Add(t.Context(), "tool", e.value, map[string]string{"title": e.title}); err != nil {
			t.Fatalf("Add %s 失败: %v", e.value, err)
		}
	}

	hits, err := seg.List(t.Context(), "tool", 0)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("期望列出 2 条主记录，实际 %d 条", len(hits))
	}
	values := map[string]bool{}
	for _, h := range hits {
		values[h.Value] = true
		if h.Field != "" {
			t.Errorf("List 不应返回子键记录，Field=%q", h.Field)
		}
		if h.Meta["title"] == "" {
			t.Errorf("List 应携带元数据: %+v", h)
		}
	}
	if !values["tool-alpha"] || !values["tool-beta"] {
		t.Errorf("条目缺失: %v", values)
	}

	limited, err := seg.List(t.Context(), "tool", 1)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limit=1 期望 1 条，实际 %d 条", len(limited))
	}
}

// TestContextCancellation 已取消的 ctx 应让 Add / Search 立即失败。
func TestContextCancellation(t *testing.T) {
	seg := newTestIndexer(t)
	defer seg.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := seg.Add(ctx, "skill", "websearch", map[string]string{"title": "网页搜索"}); !errors.Is(err, context.Canceled) {
		t.Errorf("已取消 ctx 的 Add 应返回 context.Canceled，实际 %v", err)
	}
	if _, err := seg.Search(ctx, "skill", "网页", 1); !errors.Is(err, context.Canceled) {
		t.Errorf("已取消 ctx 的 Search 应返回 context.Canceled，实际 %v", err)
	}
}

// TestValidation 参数校验：相对路径、空族名、冒号族名、空值。
func TestValidation(t *testing.T) {
	if _, err := NewSegIndexer("relative/native.db", newMockEmbedder()); err == nil {
		t.Error("相对路径应报错")
	}
	if _, err := NewSegIndexer("", newMockEmbedder()); err == nil {
		t.Error("空路径应报错")
	}
	if _, err := NewSegIndexer(filepath.Join(t.TempDir(), "native.db"), nil); err == nil {
		t.Error("空 embedder 应报错")
	}

	seg := newTestIndexer(t)
	defer seg.Close()

	if err := seg.Add(t.Context(), "", "value", nil); err == nil {
		t.Error("空族名应报错")
	}
	if err := seg.Add(t.Context(), "fa:mily", "value", nil); err == nil {
		t.Error("带冒号族名应报错")
	}
	if err := seg.Add(t.Context(), "skill", " ", nil); err == nil {
		t.Error("空白值应报错")
	}
	if _, err := seg.Search(t.Context(), "", "query", 1); err == nil {
		t.Error("空检索键应报错")
	}
	if _, err := seg.Search(t.Context(), "skill:", "query", 1); err == nil {
		t.Error("空子键名应报错")
	}
}
