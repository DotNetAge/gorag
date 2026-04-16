package result

import (
	"testing"

	"github.com/DotNetAge/gorag/core"
)

func TestFusionRRFBasic(t *testing.T) {
	// f := NewFusion(60)

	sources := []FusionSource{
		{
			Name: "vector",
			Hits: []core.Hit{
				{ID: "a", Score: 0.9},
				{ID: "b", Score: 0.7},
				{ID: "c", Score: 0.5},
			},
			Weight: 1.0,
		},
		{
			Name: "keyword",
			Hits: []core.Hit{
				{ID: "b", Score: 0.95}, // 排名第1
				{ID: "d", Score: 0.6},
				{ID: "a", Score: 0.4}, // 排名第3
			},
			Weight: 1.0,
		},
	}

	fused, err := RRF(sources...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fused) != 4 {
		t.Fatalf("expected 4 unique docs, got %d", len(fused))
	}
	// b 在两个源中排名都靠前（keyword 第1，vector 第2），应排第一
	if fused[0].ID != "b" {
		t.Errorf("expected top result 'b', got '%s'", fused[0].ID)
	}
}

func TestFusionWithWeights(t *testing.T) {

	sources := []FusionSource{
		{
			Name:   "primary",
			Hits:   []core.Hit{{ID: "x", Score: 0.9}, {ID: "y", Score: 0.5}},
			Weight: 3.0, // 高权重
		},
		{
			Name:   "secondary",
			Hits:   []core.Hit{{ID: "y", Score: 0.9}, {ID: "x", Score: 0.1}},
			Weight: 0.5, // 低权重
		},
	}

	fused, _ := RRF(sources...)

	// x 在 primary 中排名第1（高权重），应在 y 前面
	if fused[0].ID != "x" {
		t.Errorf("with weighted sources, expected 'x' first, got '%s'", fused[0].ID)
	}
}

func TestFusionEmpty(t *testing.T) {
	result, err := RRF()
	if err != nil || result != nil {
		t.Errorf("empty input should return nil, got %v %v", result, err)
	}
}

func TestFusionZeroWeightDefaultsToOne(t *testing.T) {

	sources := []FusionSource{
		{Name: "src", Hits: []core.Hit{{ID: "a", Score: 0.9}}, Weight: 0},
	}

	fused, _ := RRF(sources...)
	if len(fused) != 1 {
		t.Fatalf("expected 1, got %d", len(fused))
	}
	expectedScore := float32(1.0) / float32(62)
	if absDiff(fused[0].Score, expectedScore) > 0.001 {
		t.Errorf("expected score ~%.4f, got %.4f", expectedScore, fused[0].Score)
	}
}

func absDiff(a, b float32) float32 {
	if a > b {
		return a - b
	}
	return b - a
}
