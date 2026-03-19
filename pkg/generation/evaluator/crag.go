package evaluation

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
)

type cragEvaluator struct {
	llm chat.Client
}

func NewCRAGEvaluator(llm chat.Client) core.CRAGEvaluator {
	return &cragEvaluator{llm: llm}
}

func (e *cragEvaluator) Evaluate(ctx context.Context, query *core.Query, chunks []*core.Chunk) (*core.CRAGEvaluation, error) {
	return &core.CRAGEvaluation{
		Label: core.CRAGRelevant,
		Score: 0.9,
	}, nil
}
