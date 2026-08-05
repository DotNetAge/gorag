package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	"github.com/DotNetAge/gorag/v2/utils"
)

// mockGraphStore 是用于 Region 相关单元测试的内存图存储。
type mockGraphStore struct {
	nodes map[string]*core.Node
	edges map[string]*core.Edge
}

func newMockGraphStore() *mockGraphStore {
	return &mockGraphStore{
		nodes: make(map[string]*core.Node),
		edges: make(map[string]*core.Edge),
	}
}

func (m *mockGraphStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		cp := *n
		m.nodes[n.ID] = &cp
	}
	return nil
}

func (m *mockGraphStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	for _, e := range edges {
		if e == nil {
			continue
		}
		cp := *e
		m.edges[e.ID] = &cp
	}
	return nil
}

func (m *mockGraphStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	if n, ok := m.nodes[id]; ok {
		cp := *n
		return &cp, nil
	}
	return nil, nil
}

func (m *mockGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth, limit int) ([]*core.Node, []*core.Edge, error) {
	if depth <= 0 {
		return nil, nil, nil
	}

	visitedNodes := make(map[string]*core.Node)
	visitedEdges := make(map[string]*core.Edge)
	current := []string{nodeID}

	for d := 0; d < depth && len(current) > 0; d++ {
		next := []string{}
		for _, id := range current {
			for _, e := range m.edges {
				if e.Source == id && e.Target != "" && e.Target != nodeID {
					if n, ok := m.nodes[e.Target]; ok {
						visitedNodes[n.ID] = n
						visitedEdges[e.ID] = e
						next = append(next, e.Target)
					}
				}
				if e.Target == id && e.Source != "" && e.Source != nodeID {
					if n, ok := m.nodes[e.Source]; ok {
						visitedNodes[n.ID] = n
						visitedEdges[e.ID] = e
						next = append(next, e.Source)
					}
				}
			}
		}
		current = next
		if len(visitedNodes) >= limit {
			break
		}
	}

	nodes := make([]*core.Node, 0, len(visitedNodes))
	for _, n := range visitedNodes {
		cp := *n
		nodes = append(nodes, &cp)
	}
	edges := make([]*core.Edge, 0, len(visitedEdges))
	for _, e := range visitedEdges {
		cp := *e
		edges = append(edges, &cp)
	}
	return nodes, edges, nil
}

func (m *mockGraphStore) GetByChunkIDs(ctx context.Context, chunkIDs []string) ([]*core.Node, []*core.Edge, error) {
	return nil, nil, nil
}

func (m *mockGraphStore) GetByLabels(ctx context.Context, labels []string, limit int) ([]*core.Node, error) {
	return nil, nil
}

func (m *mockGraphStore) DeleteNode(ctx context.Context, id string) error { return nil }
func (m *mockGraphStore) DeleteEdge(ctx context.Context, id string) error { return nil }
func (m *mockGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	if strings.Contains(query, "count(n)") {
		return []map[string]any{{"cnt": int64(len(m.nodes))}}, nil
	}
	return nil, fmt.Errorf("mockGraphStore: 不支持的 Cypher 查询")
}
func (m *mockGraphStore) Clear(ctx context.Context) error { return nil }
func (m *mockGraphStore) Close(ctx context.Context) error { return nil }

// TestGraphIndexer_RegionIDMatchesChunkRegionID 验证 GraphIndexer 创建的 Region 节点 ID 与 Chunk.RegionID 一致。
func TestGraphIndexer_RegionIDMatchesChunkRegionID(t *testing.T) {
	ctx := context.Background()
	store := newMockGraphStore()
	idx, err := New(store)
	if err != nil {
		t.Fatalf("创建 GraphIndexer 失败: %v", err)
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "doc.md")
	if writeErr := os.WriteFile(filePath, []byte("# 标题\n\n正文。"), 0o644); writeErr != nil {
		t.Fatalf("写入测试文件失败: %v", writeErr)
	}

	raw, err := document.Open(filePath)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	doc, err := core.Structurize(raw)
	if err != nil {
		t.Fatalf("Structurize 失败: %v", err)
	}

	// 构造一个文档根节点，模拟 Chunker 产出
	docNode := core.Node{
		ID:     utils.GenerateID([]byte(raw.ID() + ":doc:Document")),
		Labels: []string{core.LabelDocument},
		Name:   "doc",
	}
	doc.SetNodes([]core.Node{docNode})

	storeIndexer, ok := idx.(IndexerStore)
	if !ok {
		t.Fatalf("GraphIndexer 未实现 IndexerStore")
	}
	if err := storeIndexer.Save(ctx, doc); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	expectedRegionID := utils.GenerateID([]byte(strings.ToLower(dir)))
	regionNode, ok := store.nodes[expectedRegionID]
	if !ok {
		t.Fatalf("未找到期望的 Region 节点 %s", expectedRegionID)
	}
	foundRegionLabel := false
	for _, l := range regionNode.Labels {
		if l == core.LabelRegion {
			foundRegionLabel = true
			break
		}
	}
	if !foundRegionLabel {
		t.Errorf("Region 节点 Labels 期望包含 %q，实际 %v", core.LabelRegion, regionNode.Labels)
	}
	if regionNode.Name != filepath.Base(strings.ToLower(dir)) {
		t.Errorf("Region 节点 Name 期望 %q，实际 %q", filepath.Base(strings.ToLower(dir)), regionNode.Name)
	}
}

// TestGraphIndexer_RegionNodeNotOverwritten 验证当 Region 节点已存在时，Save 不会覆盖其 SourceChunkIDs。
func TestGraphIndexer_RegionNodeNotOverwritten(t *testing.T) {
	ctx := context.Background()
	store := newMockGraphStore()
	idx, err := New(store)
	if err != nil {
		t.Fatalf("创建 GraphIndexer 失败: %v", err)
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "doc.md")
	if writeErr := os.WriteFile(filePath, []byte("# 标题\n\n正文。"), 0o644); writeErr != nil {
		t.Fatalf("写入测试文件失败: %v", writeErr)
	}

	raw, err := document.Open(filePath)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	doc, err := core.Structurize(raw)
	if err != nil {
		t.Fatalf("Structurize 失败: %v", err)
	}

	regionID := utils.GenerateID([]byte(dir))
	// 预置一个由 README.md 产出的 Region 节点，携带 SourceChunkIDs
	existingRegion := &core.Node{
		ID:             regionID,
		Labels:         []string{core.LabelRegion},
		Name:           filepath.Base(dir),
		Properties:     map[string]any{core.PropDir: dir},
		SourceChunkIDs: []string{"readme-chunk-id"},
	}
	store.nodes[regionID] = existingRegion

	docNode := core.Node{
		ID:     utils.GenerateID([]byte(raw.ID() + ":doc:Document")),
		Labels: []string{core.LabelDocument},
		Name:   "doc",
	}
	doc.SetNodes([]core.Node{docNode})

	storeIndexer, ok := idx.(IndexerStore)
	if !ok {
		t.Fatalf("GraphIndexer 未实现 IndexerStore")
	}
	if err := storeIndexer.Save(ctx, doc); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	regionNode := store.nodes[regionID]
	if len(regionNode.SourceChunkIDs) != 1 || regionNode.SourceChunkIDs[0] != "readme-chunk-id" {
		t.Errorf("Region 节点的 SourceChunkIDs 不应被覆盖，实际 %v", regionNode.SourceChunkIDs)
	}
}

// TestGraphIndexer_Neighbors 验证多跳邻居遍历。
func TestGraphIndexer_Neighbors(t *testing.T) {
	ctx := context.Background()
	store := newMockGraphStore()
	idx, err := New(store)
	if err != nil {
		t.Fatalf("创建 GraphIndexer 失败: %v", err)
	}

	nav, ok := idx.(GraphNavigator)
	if !ok {
		t.Fatalf("GraphIndexer 未实现 GraphNavigator")
	}

	// 构造图：region --CONTAINS--> doc --HAS--> entity
	store.nodes["region"] = &core.Node{ID: "region", Labels: []string{"Region"}, Name: "docs"}
	store.nodes["doc"] = &core.Node{ID: "doc", Labels: []string{core.LabelDocument}, Name: "intro.md"}
	store.nodes["entity"] = &core.Node{ID: "entity", Labels: []string{"Person"}, Name: "张三"}
	store.edges["e1"] = &core.Edge{ID: "e1", Type: "CONTAINS", Source: "region", Target: "doc"}
	store.edges["e2"] = &core.Edge{ID: "e2", Type: "HAS", Source: "doc", Target: "entity"}

	nodes, edges, err := nav.Neighbors(ctx, "region", 1, 100)
	if err != nil {
		t.Fatalf("Neighbors 失败: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "doc" {
		t.Errorf("1 跳应只返回 doc，实际 %v", nodes)
	}
	if len(edges) != 1 || edges[0].ID != "e1" {
		t.Errorf("1 跳应只返回 e1，实际 %v", edges)
	}

	nodes, edges, err = nav.Neighbors(ctx, "region", 2, 100)
	if err != nil {
		t.Fatalf("Neighbors 2 跳失败: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("2 跳应返回 2 个节点，实际 %d", len(nodes))
	}
	if len(edges) != 2 {
		t.Errorf("2 跳应返回 2 条边，实际 %d", len(edges))
	}
}

// TestGraphIndexer_GetNode 验证按 ID 获取节点。
func TestGraphIndexer_GetNode(t *testing.T) {
	ctx := context.Background()
	store := newMockGraphStore()
	idx, err := New(store)
	if err != nil {
		t.Fatalf("创建 GraphIndexer 失败: %v", err)
	}

	nav, ok := idx.(GraphNavigator)
	if !ok {
		t.Fatalf("GraphIndexer 未实现 GraphNavigator")
	}

	store.nodes["n1"] = &core.Node{ID: "n1", Labels: []string{"Person"}, Name: "李四"}
	node, err := nav.GetNode(ctx, "n1")
	if err != nil {
		t.Fatalf("GetNode 失败: %v", err)
	}
	if node == nil || node.Name != "李四" {
		t.Errorf("GetNode 返回错误: %+v", node)
	}

	node, err = nav.GetNode(ctx, "not-exist")
	if err != nil {
		t.Fatalf("GetNode 不存在节点不应报错: %v", err)
	}
	if node != nil {
		t.Errorf("不存在节点应返回 nil，实际 %+v", node)
	}
}

// TestGraphIndexer_CypherQuery 验证原始 Cypher 查询委托。
func TestGraphIndexer_CypherQuery(t *testing.T) {
	ctx := context.Background()
	store := newMockGraphStore()
	idx, err := New(store)
	if err != nil {
		t.Fatalf("创建 GraphIndexer 失败: %v", err)
	}

	store.nodes["n1"] = &core.Node{ID: "n1", Labels: []string{"Person"}, Name: "王五"}
	store.nodes["n2"] = &core.Node{ID: "n2", Labels: []string{"Person"}, Name: "赵六"}

	gidx, ok := idx.(*GraphIndexer)
	if !ok {
		t.Fatalf("idx 应为 *GraphIndexer")
	}
	rows, err := gidx.CypherQuery(ctx, "MATCH (n) RETURN count(n) AS cnt", nil)
	if err != nil {
		t.Fatalf("CypherQuery 失败: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("期望返回 1 行，实际 %d", len(rows))
	}
	if cnt, ok := rows[0]["cnt"].(int64); !ok || cnt != 2 {
		t.Errorf("节点数应为 2，实际 %v", rows[0]["cnt"])
	}
}

// TestGraphIndexer_ExploreRegion 验证目录级图探索。
func TestGraphIndexer_ExploreRegion(t *testing.T) {
	ctx := context.Background()
	store := newMockGraphStore()
	idx, err := New(store)
	if err != nil {
		t.Fatalf("创建 GraphIndexer 失败: %v", err)
	}

	explorer, ok := idx.(GraphExplorer)
	if !ok {
		t.Fatalf("GraphIndexer 未实现 GraphExplorer")
	}

	dir := "/home/user/docs"
	regionID := utils.GenerateID([]byte(dir))
	store.nodes[regionID] = &core.Node{ID: regionID, Labels: []string{core.LabelRegion}, Name: "docs"}
	store.nodes["doc"] = &core.Node{ID: "doc", Labels: []string{core.LabelDocument}, Name: "intro.md"}
	store.nodes["entity"] = &core.Node{ID: "entity", Labels: []string{"Person"}, Name: "张三"}
	store.edges["e1"] = &core.Edge{ID: "e1", Type: "CONTAINS", Source: regionID, Target: "doc"}
	store.edges["e2"] = &core.Edge{ID: "e2", Type: "HAS", Source: "doc", Target: "entity"}

	view, err := explorer.ExploreRegion(ctx, dir, 2, 100)
	if err != nil {
		t.Fatalf("ExploreRegion 失败: %v", err)
	}

	if view.RegionID != regionID {
		t.Errorf("RegionID 期望 %q，实际 %q", regionID, view.RegionID)
	}
	if view.RegionName != "docs" {
		t.Errorf("RegionName 期望 %q，实际 %q", "docs", view.RegionName)
	}

	// Region + doc + entity = 3 个节点
	if len(view.Nodes) != 3 {
		t.Errorf("2 跳探索应返回 3 个节点，实际 %d", len(view.Nodes))
	}

	// e1 + e2 = 2 条边
	if len(view.Edges) != 2 {
		t.Errorf("2 跳探索应返回 2 条边，实际 %d", len(view.Edges))
	}
}
