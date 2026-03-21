package agentic

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockRetriever struct {
	shouldFail bool
	called     bool
}

func (m *mockRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	m.called = true
	if m.shouldFail {
		return nil, errors.New("retrieval failed")
	}
	return []*core.RetrievalResult{{Query: queries[0]}}, nil
}

type mockClassifier struct {
	intent core.IntentType
}

func (m *mockClassifier) Classify(ctx context.Context, query *core.Query) (*core.IntentResult, error) {
	return &core.IntentResult{Intent: m.intent, Confidence: 1.0}, nil
}

// AuditStandard_Agentic_ExecutionFallback 审计标准：路由执行失败必须自动回退到默认
func TestAuditStandard_Agentic_ExecutionFallback(t *testing.T) {
	failedTarget := &mockRetriever{shouldFail: true}
	defaultRet := &mockRetriever{shouldFail: false}
	
	router := NewSmartRouter(
		&mockClassifier{intent: core.IntentFactCheck},
		map[core.IntentType]core.Retriever{
			core.IntentFactCheck: failedTarget,
		},
		defaultRet,
		nil,
	)

	res, err := router.Retrieve(context.Background(), []string{"test query"}, 5)
	
	assert.NoError(t, err)
	assert.True(t, failedTarget.called, "Failed target should have been called")
	assert.True(t, defaultRet.called, "Default retriever should have been called as fallback")
	assert.Len(t, res, 1)
	assert.Equal(t, core.IntentFactCheck, res[0].Metadata["intent"], "Original intent should still be preserved in metadata")
}

// AuditStandard_Agentic_NoRetriever 审计标准：无匹配意图时使用默认
func TestAuditStandard_Agentic_NoRetriever(t *testing.T) {
	defaultRet := &mockRetriever{shouldFail: false}
	
	router := NewSmartRouter(
		&mockClassifier{intent: core.IntentRelational}, // Mapping doesn't have this
		map[core.IntentType]core.Retriever{},
		defaultRet,
		nil,
	)

	res, err := router.Retrieve(context.Background(), []string{"test query"}, 5)
	
	assert.NoError(t, err)
	assert.True(t, defaultRet.called)
	assert.Len(t, res, 1)
}
