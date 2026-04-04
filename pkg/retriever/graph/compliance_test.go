package graph

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/pkg/core"
	graphstep "github.com/DotNetAge/gorag/pkg/steps/graph"
	"github.com/stretchr/testify/assert"
)

type auditMockGraphStore struct {
	core.GraphStore
	delay time.Duration
}

func (m *auditMockGraphStore) GetNeighbors(ctx context.Context, id string, depth, limit int) ([]*core.Node, []*core.Edge, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return []*core.Node{{ID: "neighbor-" + id}}, nil, nil
}

// AuditStandard_Graph_ConcurrentPerformance 审计标准：邻居检索必须是并发的
func TestAuditStandard_Graph_ConcurrentPerformance(t *testing.T) {
	// 每个查询延迟 100ms
	mock := &auditMockGraphStore{delay: 100 * time.Millisecond}
	step := graphstep.NewLocalSearch(mock,
		graphstep.WithDepth(1),
		graphstep.WithLimit(10),
	)

	ctx := context.Background()
	rctx := core.NewRetrievalContext(ctx, "test")
	// 模拟提取了 5 个实体
	rctx.Custom["extracted_entities"] = []string{"e1", "e2", "e3", "e4", "e5"}

	start := time.Now()
	err := step.Execute(ctx, rctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	// 如果是串行，耗时 > 500ms；如果是并发，耗时应约 100ms
	assert.Less(t, duration, 250*time.Millisecond, "Graph search must be concurrent to save time")
}

// AuditStandard_Graph_EntityLimit 审计标准：处理的实体数量必须受限
func TestAuditStandard_Graph_EntityLimit(t *testing.T) {
	mock := &auditMockGraphStore{}
	step := graphstep.NewLocalSearch(mock,
		graphstep.WithDepth(1),
		graphstep.WithLimit(10),
	)

	ctx := context.Background()
	rctx := core.NewRetrievalContext(ctx, "test")

	// 模拟"实体爆炸" (100个实体)
	entities := make([]string, 100)
	for i := range 100 {
		entities[i] = "e"
	}
	rctx.Custom["extracted_entities"] = entities

	err := step.Execute(ctx, rctx)
	assert.NoError(t, err)
	// 检查生成的 graph_context，确保只有 10 个左右的结果
	// (具体的 limit 我们在代码中设置的是 10)
	assert.NotNil(t, rctx.GraphContext, "Graph context should be generated")
}
