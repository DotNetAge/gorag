package view

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// fakeGraphStore 极简图存储，仅用于 ViewService 单元测试。
// 实现 GraphStore 的 Cypher 子集：MATCH (n:LABEL) RETURN n、RETURN count、RETURN DISTINCT labels。
type fakeGraphStore struct {
	nodes []map[string]any
}

func (f *fakeGraphStore) CypherQuery(_ context.Context, q string, _ map[string]any) ([]map[string]any, error) {
	qUpper := strings.ToUpper(q)
	// 注意：必须同时在原始 q 中做 extractLabel 以保留大小写

	// 1) 收集所有 DISTINCT labels
	if strings.Contains(qUpper, "DISTINCT LABELS(N)") {
		set := make(map[string]struct{})
		for _, n := range f.nodes {
			if lbls, ok := n["labels"].([]string); ok {
				for _, l := range lbls {
					set[l] = struct{}{}
				}
			}
		}
		out := make([]map[string]any, 0, len(set))
		for l := range set {
			out = append(out, map[string]any{
				"labels": []any{l},
			})
		}
		return out, nil
	}

	// 2) count(n)
	if strings.Contains(qUpper, "COUNT(N)") {
		label := extractLabel(q)
		count := 0
		for _, n := range f.nodes {
			if label == "" {
				count++
				continue
			}
			if lbls, ok := n["labels"].([]string); ok && slices.Contains(lbls, label) {
				count++
			}
		}
		return []map[string]any{{"cnt": int64(count)}}, nil
	}

	// 3) 简化版：MATCH (n:LABEL) RETURN n
	label := extractLabel(q)
	if label == "" {
		return nil, nil
	}

	out := make([]map[string]any, 0, len(f.nodes))
	for _, n := range f.nodes {
		if lbls, ok := n["labels"].([]string); ok {
			if !slices.Contains(lbls, label) {
				continue
			}
		}
		out = append(out, map[string]any{
			"n": n["properties"],
		})
	}
	return out, nil
}

func extractLabel(q string) string {
	idx := strings.Index(q, "(N:")
	if idx < 0 {
		idx = strings.Index(q, "(n:")
	}
	if idx < 0 {
		return ""
	}
	rest := q[idx+3:]
	end := strings.IndexAny(rest, " )")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// TestViewServiceQuery 验证基础查询流程。
func TestViewServiceQuery(t *testing.T) {
	fake := &fakeGraphStore{
		nodes: []map[string]any{
			{
				"labels":     []string{"Person"},
				"properties": map[string]any{"name": "Alice", "age": int64(30), "role": "Engineer"},
			},
			{
				"labels":     []string{"Person"},
				"properties": map[string]any{"name": "Bob", "age": int64(25), "role": "Designer"},
			},
		},
	}

	svc, err := New(fake)
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}

	result, err := svc.Query(context.Background(), Spec{
		Label:     "Person",
		Page:      1,
		Size:      10,
		WithTotal: true,
	})
	if err != nil {
		t.Fatalf("Query 失败: %v", err)
	}

	if result.Label != "Person" {
		t.Errorf("Label 错误: %s", result.Label)
	}
	if result.Total != 2 {
		t.Errorf("Total 错误: %d", result.Total)
	}
	if len(result.Rows) != 2 {
		t.Errorf("Rows 长度错误: %d", len(result.Rows))
	}
}

// TestViewServiceListLabels 验证标签列表功能（使用 fakeGraphStore）。
func TestViewServiceListLabels(t *testing.T) {
	fake := &fakeGraphStore{
		nodes: []map[string]any{
			{
				"labels":     []string{"Person"},
				"properties": map[string]any{"name": "Alice"},
			},
			{
				"labels":     []string{"Document"},
				"properties": map[string]any{"title": "Doc"},
			},
		},
	}

	svc, err := New(fake)
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	labels, err := svc.ListLabels(context.Background())
	if err != nil {
		t.Fatalf("ListLabels 失败: %v", err)
	}
	// Document 是内部标签，应被过滤；只剩 Person
	if len(labels) != 1 {
		t.Errorf("过滤内部标签后应剩 1 个，实际 %d", len(labels))
	}
	if labels[0].Label != "Person" {
		t.Errorf("Label 错误: %s", labels[0].Label)
	}
}

// TestViewServiceDescribe 验证 Describe 接口（图内省）。
func TestViewServiceDescribe(t *testing.T) {
	fake := &fakeGraphStore{
		nodes: []map[string]any{
			{
				"labels":     []string{"Person"},
				"properties": map[string]any{"name": "Alice", "age": int64(30)},
			},
		},
	}
	svc, err := New(fake)
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}

	schema, err := svc.Describe(context.Background(), "Person", "")
	if err != nil {
		t.Fatalf("Describe 失败: %v", err)
	}
	if schema.Label != "Person" {
		t.Errorf("Label 错误: %s", schema.Label)
	}
	if len(schema.Columns) == 0 {
		t.Errorf("Columns 为空（应有 name + age）")
	}
}

// TestViewServiceInvalidParam 验证错误码。
func TestViewServiceInvalidParam(t *testing.T) {
	svc, _ := New(&fakeGraphStore{})

	_, err := svc.Query(context.Background(), Spec{Label: ""})
	if err == nil {
		t.Fatal("应返回错误")
	}
	ve, ok := err.(*Error)
	if !ok {
		t.Fatalf("应返回 *Error，实际 %T", err)
	}
	if ve.Code != ErrInvalidParam {
		t.Errorf("错误码错误: %d", ve.Code)
	}
}

// TestViewServiceFieldInjection 验证 Cypher 注入防御。
func TestViewServiceFieldInjection(t *testing.T) {
	svc, _ := New(&fakeGraphStore{})
	_, err := svc.Query(context.Background(), Spec{
		Label: "Person",
		Where: &Where{
			Equals: map[string]any{
				"name; DROP TABLE": "x", // 非法标识符
			},
		},
	})
	if err == nil {
		t.Fatal("应拒绝非法字段名")
	}
	ve, ok := err.(*Error)
	if !ok || ve.Code != ErrUnknownField {
		t.Errorf("应返回 ErrUnknownField，实际 %v", err)
	}
}

// TestIsCypherIdentifier 验证标识符校验。
func TestIsCypherIdentifier(t *testing.T) {
	cases := []struct {
		input  string
		expect bool
	}{
		{"name", true},
		{"first_name", true},
		{"_private", true},
		{"user123", true},
		{"", false},
		{"123abc", false},
		{"a-b", false},
		{"a b", false},
		{"a; DROP", false},
		{"a.b", false},
	}
	for _, c := range cases {
		got := isCypherIdentifier(c.input)
		if got != c.expect {
			t.Errorf("isCypherIdentifier(%q) = %v, want %v", c.input, got, c.expect)
		}
	}
}
