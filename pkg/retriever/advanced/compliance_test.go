package advanced

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

// MockDecomposer for testing fallback and limits
type mockDecomposer struct {
	shouldFail bool
	subQueries []string
}

func (m *mockDecomposer) Decompose(ctx context.Context, query *core.Query) (*core.DecompositionResult, error) {
	if m.shouldFail {
		return nil, errors.New("llm error")
	}
	return &core.DecompositionResult{SubQueries: m.subQueries}, nil
}

// AuditStandard_Advanced_Fallback 审计标准：LLM失效时必须优雅降级
func TestAuditStandard_Advanced_Fallback(t *testing.T) {
	mockDec := &mockDecomposer{shouldFail: true}
	
	// Create retriever with failing decomposer
	ret := NewFusionRetrieverWithComponents(nil, nil, mockDec, 5, nil)
	
	ctx := context.Background()
	rctx := core.NewRetrievalContext(ctx, "original query")
	
	// Execute pipeline directly to check internal state
	err := ret.(*fusionRetriever).pipeline.Execute(ctx, rctx)
	
	// Although vector search will fail later (due to nil store), 
	// we want to verify the Step 1 state.
	assert.NoError(t, err, "Pipeline should not return error on Step 1 failure")
	assert.Equal(t, []string{"original query"}, rctx.Agentic.SubQueries, "Should fallback to original query")
}

// AuditStandard_Advanced_QueryLimit 审计标准：子查询数量必须受限
func TestAuditStandard_Advanced_QueryLimit(t *testing.T) {
	// Simulate LLM returning too many queries
	manyQueries := []string{"q1", "q2", "q3", "q4", "q5", "q6", "q7"}
	mockDec := &mockDecomposer{subQueries: manyQueries}
	
	ret := NewFusionRetrieverWithComponents(nil, nil, mockDec, 5, nil)
	
	ctx := context.Background()
	rctx := core.NewRetrievalContext(ctx, "test")
	
	_ = ret.(*fusionRetriever).pipeline.Execute(ctx, rctx)
	
	assert.LessOrEqual(t, len(rctx.Agentic.SubQueries), 5, "Sub-queries must be limited to 5 to protect resources")
}

// AuditStandard_Advanced_Resource 审计标准：必须尊重 Context 取消
func TestAuditStandard_Advanced_Resource(t *testing.T) {
	mockDec := &mockDecomposer{subQueries: []string{"q1"}}
	ret := NewFusionRetrieverWithComponents(nil, nil, mockDec, 5, nil)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	rctx := core.NewRetrievalContext(ctx, "test")
	err := ret.(*fusionRetriever).pipeline.Execute(ctx, rctx)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), context.Canceled.Error())
}
