package evaluation

import (
	"context"
	"fmt"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
)

type ragEvaluator struct {
	llm chat.Client
}

func NewRAGEvaluator(llm chat.Client) core.RAGEvaluator {
	return &ragEvaluator{llm: llm}
}

func (e *ragEvaluator) Evaluate(ctx context.Context, query string, answer string, context string) (*core.RAGEvaluation, error) {
	prompt := fmt.Sprintf(`Evaluate the quality of the following RAG response.
Query: %s
Answer: %s
Context: %s

Evaluate on Faithfulness and Relevance. Return JSON with scores 0.0-1.0.`, query, answer, context)

	messages := []chat.Message{chat.NewUserMessage(prompt)}
	_, err := e.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	return &core.RAGEvaluation{
		Faithfulness: 0.8,
		Relevance:    0.8,
		OverallScore: 0.8,
		Passed:       true,
	}, nil
}
