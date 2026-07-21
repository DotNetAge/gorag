package result

import (
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
)

// makeChunkHit 构造测试用的 ChunkHit（嵌入 *Chunk）。
func makeChunkHit(id, docID, content string, score float32) core.ChunkHit {
	return core.ChunkHit{
		Chunk: &core.Chunk{
			ID:      id,
			DocID:   docID,
			Content: content,
		},
		Score: score,
	}
}

// makeNodeHit 构造测试用的 NodeHit（嵌入 *Node）。
func makeNodeHit(id, name string, score float32) core.NodeHit {
	return core.NodeHit{
		Node: &core.Node{
			ID:   id,
			Name: name,
		},
		Score: score,
	}
}

func TestFusionRRFBasic(t *testing.T) {
	// 语义源：chunks a/b/c 按分数降序
	semHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("a", "doc1", "content a", 0.9),
			makeChunkHit("b", "doc1", "content b", 0.7),
			makeChunkHit("c", "doc1", "content c", 0.5),
		},
	}
	// 关键词源：chunks b/d/a 按分数降序（b 排第 1，a 排第 3）
	kwHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("b", "doc1", "content b", 0.95),
			makeChunkHit("d", "doc1", "content d", 0.6),
			makeChunkHit("a", "doc1", "content a", 0.4),
		},
	}

	sources := []*FusionSource{
		NewSource("vector", 1.0, semHit),
		NewSource("keyword", 1.0, kwHit),
	}

	fused, err := RRF(sources...)
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}
	if fused == nil {
		t.Fatalf("融合结果为 nil")
	}
	if len(fused.Chunks) != 4 {
		t.Fatalf("期望 4 个去重后的 Chunk，实际 %d", len(fused.Chunks))
	}
	// b 在两个源中排名都靠前（keyword 第 1，vector 第 2），应排第一
	if fused.Chunks[0].ID != "b" {
		t.Errorf("期望首个结果为 'b'，实际 '%s'", fused.Chunks[0].ID)
	}
}

func TestFusionWithWeights(t *testing.T) {
	primaryHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("x", "doc1", "content x", 0.9),
			makeChunkHit("y", "doc1", "content y", 0.5),
		},
	}
	secondaryHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("y", "doc1", "content y", 0.9),
			makeChunkHit("x", "doc1", "content x", 0.1),
		},
	}

	sources := []*FusionSource{
		NewSource("primary", 3.0, primaryHit),   // 高权重
		NewSource("secondary", 0.5, secondaryHit), // 低权重
	}

	fused, _ := RRF(sources...)

	// x 在 primary 中排名第 1（高权重），应在 y 前面
	if fused.Chunks[0].ID != "x" {
		t.Errorf("加权后期望 'x' 排第一，实际 '%s'", fused.Chunks[0].ID)
	}
}

func TestFusionEmpty(t *testing.T) {
	result, err := RRF()
	if err != nil || result != nil {
		t.Errorf("空输入应返回 (nil, nil)，实际 (%v, %v)", result, err)
	}
}

func TestFusionAllNilHits(t *testing.T) {
	// 所有源的 Hit 都为 nil，应返回 (nil, nil)
	sources := []*FusionSource{
		NewSource("empty1", 1.0, nil),
		NewSource("empty2", 1.0, nil),
	}
	result, err := RRF(sources...)
	if err != nil || result != nil {
		t.Errorf("全 nil 源应返回 (nil, nil)，实际 (%v, %v)", result, err)
	}
}

func TestFusionZeroWeightDefaultsToOne(t *testing.T) {
	semHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("a", "doc1", "content a", 0.9),
		},
	}

	sources := []*FusionSource{
		NewSource("src", 0, semHit), // 权重 0 应默认为 1.0
	}

	fused, _ := RRF(sources...)
	if len(fused.Chunks) != 1 {
		t.Fatalf("期望 1 个结果，实际 %d", len(fused.Chunks))
	}
	expectedScore := float32(1.0) / float32(62) // k=60, rank=0
	if absDiff(fused.Chunks[0].Score, expectedScore) > 0.001 {
		t.Errorf("期望分数 ~%.4f，实际 %.4f", expectedScore, fused.Chunks[0].Score)
	}
}

func TestFusionSingleSource(t *testing.T) {
	semHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("a", "doc1", "content a", 0.9),
			makeChunkHit("b", "doc1", "content b", 0.7),
			makeChunkHit("c", "doc1", "content c", 0.5),
		},
	}

	sources := []*FusionSource{
		NewSource("single", 1.0, semHit),
	}

	fused, err := RRF(sources...)
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}
	if len(fused.Chunks) != 3 {
		t.Fatalf("期望 3 个结果，实际 %d", len(fused.Chunks))
	}
}

func TestFusionDuplicateIDsAcrossSources(t *testing.T) {
	src1Hit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("a", "doc1", "content a", 0.9),
			makeChunkHit("b", "doc1", "content b", 0.5),
		},
	}
	src2Hit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("a", "doc1", "content a", 0.8),
			makeChunkHit("b", "doc1", "content b", 0.6),
		},
	}

	sources := []*FusionSource{
		NewSource("src1", 1.0, src1Hit),
		NewSource("src2", 1.0, src2Hit),
	}

	fused, err := RRF(sources...)
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}
	if len(fused.Chunks) != 2 {
		t.Fatalf("期望 2 个去重后的结果，实际 %d", len(fused.Chunks))
	}
}

func TestFusionResultsSortedByScore(t *testing.T) {
	semHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("z", "doc1", "content z", 0.1),
			makeChunkHit("y", "doc1", "content y", 0.2),
			makeChunkHit("x", "doc1", "content x", 0.3),
		},
	}

	sources := []*FusionSource{
		NewSource("src1", 1.0, semHit),
	}

	fused, _ := RRF(sources...)
	for i := 1; i < len(fused.Chunks); i++ {
		if fused.Chunks[i].Score > fused.Chunks[i-1].Score {
			t.Errorf("结果未按分数降序排序: [%d].Score=%.4f > [%d-1].Score=%.4f",
				i, fused.Chunks[i].Score, i-1, fused.Chunks[i-1].Score)
		}
	}
}

func TestRRFWithK_CustomK(t *testing.T) {
	semHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("a", "doc1", "content a", 0.9),
		},
	}

	sources := []*FusionSource{
		NewSource("src", 2.0, semHit),
	}

	fused, err := RRFWithK(10, sources...)
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}
	if len(fused.Chunks) != 1 {
		t.Fatalf("期望 1 个结果，实际 %d", len(fused.Chunks))
	}
	expectedScore := float32(2.0) / float32(11) // k=10, rank=0
	if absDiff(fused.Chunks[0].Score, expectedScore) > 0.001 {
		t.Errorf("k=10 时期望分数 ~%.4f，实际 %.4f", expectedScore, fused.Chunks[0].Score)
	}
}

func TestNewSource(t *testing.T) {
	hit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("a", "doc1", "content a", 0.9),
		},
	}
	src := NewSource("test", 1.5, hit)

	if src.Name != "test" {
		t.Errorf("期望 name 'test'，实际 '%s'", src.Name)
	}
	if src.Weight != 1.5 {
		t.Errorf("期望 weight 1.5，实际 %.2f", src.Weight)
	}
	if src.Hit == nil || len(src.Hit.Chunks) != 1 {
		t.Errorf("期望 Hit 含 1 个 Chunk，实际 %v", src.Hit)
	}
}

// TestFusionMixedContent 测试 HyperIndexer 双线融合场景：语义 Hit + 图 Hit。
func TestFusionMixedContent(t *testing.T) {
	// 语义源：仅 Chunks
	semHit := &core.Hit{
		Chunks: []core.ChunkHit{
			makeChunkHit("chunk1", "doc1", "content 1", 0.9),
			makeChunkHit("chunk2", "doc1", "content 2", 0.7),
		},
	}
	// 图源：仅 Nodes/Edges
	graphHit := &core.Hit{
		Nodes: []core.NodeHit{
			makeNodeHit("node1", "实体1", 0.8),
			makeNodeHit("node2", "实体2", 0.6),
		},
		Edges: []core.EdgeHit{
			{Edge: &core.Edge{ID: "edge1", Type: "CALLS", Source: "node1", Target: "node2"}, Score: 0.5},
		},
	}

	sources := []*FusionSource{
		NewSource("semantic", 1.0, semHit),
		NewSource("graph", 0.7, graphHit),
	}

	fused, err := RRF(sources...)
	if err != nil {
		t.Fatalf("未预期的错误: %v", err)
	}
	if fused == nil {
		t.Fatalf("融合结果为 nil")
	}

	// 验证 Chunks 来自语义线
	if len(fused.Chunks) != 2 {
		t.Errorf("期望 2 个 Chunks，实际 %d", len(fused.Chunks))
	}
	// 验证 Nodes 来自图线
	if len(fused.Nodes) != 2 {
		t.Errorf("期望 2 个 Nodes，实际 %d", len(fused.Nodes))
	}
	// 验证 Edges 来自图线
	if len(fused.Edges) != 1 {
		t.Errorf("期望 1 个 Edge，实际 %d", len(fused.Edges))
	}

	// 验证 Hit.Score = topChunkScore + topNodeScore + topEdgeScore
	// topChunk = 1.0/(60+0+1) = 1/61
	// topNode = 0.7/(60+0+1) = 0.7/61
	// topEdge = 0.7/(60+0+1) = 0.7/61
	expectedScore := float32(1.0+0.7+0.7) / float32(61)
	if absDiff(fused.Score, expectedScore) > 0.001 {
		t.Errorf("期望综合分数 ~%.4f，实际 %.4f", expectedScore, fused.Score)
	}
}

func absDiff(a, b float32) float32 {
	if a > b {
		return a - b
	}
	return b - a
}
