package evaluation

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// LLMJudge provides production-grade Evaluation metrics (e.g., RAGAS) using an LLM as the evaluator.
type LLMJudge interface {
	// EvaluateFaithfulness checks if the generated answer is strictly grounded in the retrieved chunks.
	EvaluateFaithfulness(ctx context.Context, query string, chunks []*entity.Chunk, answer string) (score float32, reason string, err error)
	
	// EvaluateAnswerRelevance checks if the answer effectively addresses the user's intent.
	EvaluateAnswerRelevance(ctx context.Context, query string, answer string) (score float32, reason string, err error)
	
	// EvaluateContextPrecision checks if the retrieved context actually contains the useful information.
	EvaluateContextPrecision(ctx context.Context, query string, chunks []*entity.Chunk) (score float32, reason string, err error)
}
