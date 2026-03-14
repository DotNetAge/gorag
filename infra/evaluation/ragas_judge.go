package evaluation

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/evaluation"
)

// ensure interface implementation
var _ evaluation.LLMJudge = (*RagasLLMJudge)(nil)

// RagasLLMJudge implements the LLMJudge interface using standard RAGAS-style prompts.
// It leverages a strong LLM (like GPT-4) to grade the pipeline's output.
type RagasLLMJudge struct {
	judgeLLM chat.Client
}

func NewRagasLLMJudge(judgeLLM chat.Client) *RagasLLMJudge {
	return &RagasLLMJudge{judgeLLM: judgeLLM}
}

// EvaluateFaithfulness checks for hallucinations against the retrieved context.
func (j *RagasLLMJudge) EvaluateFaithfulness(ctx context.Context, query string, chunks []*entity.Chunk, answer string) (float32, string, error) {
	contextText := buildContextText(chunks)

	prompt := fmt.Sprintf(`You are an expert evaluator. Your task is to check if the generated answer is faithful to the provided context.
An answer is faithful (Score 1.0) if ALL claims made in the answer can be directly inferred from the context.
It is unfaithful (Score 0.0) if it contains hallucinations or outside knowledge not present in the context.
If it is partially faithful, give a score between 0.1 and 0.9.

[Context]
%s

[Generated Answer]
%s

Please output your response strictly in the following format:
Score: <float between 0.0 and 1.0>
Reason: <brief explanation>`, contextText, answer)

	return j.parseEvalResponse(ctx, prompt)
}

// EvaluateAnswerRelevance checks if the answer actually answers the user's question.
func (j *RagasLLMJudge) EvaluateAnswerRelevance(ctx context.Context, query string, answer string) (float32, string, error) {
	prompt := fmt.Sprintf(`You are an expert evaluator. Your task is to check if the generated answer directly addresses the user's query.
Score 1.0 if it is a complete and direct answer.
Score 0.0 if it dodges the question or answers something completely unrelated.

[Query]
%s

[Generated Answer]
%s

Please output your response strictly in the following format:
Score: <float between 0.0 and 1.0>
Reason: <brief explanation>`, query, answer)

	return j.parseEvalResponse(ctx, prompt)
}

// EvaluateContextPrecision checks the quality of the retrieved chunks.
func (j *RagasLLMJudge) EvaluateContextPrecision(ctx context.Context, query string, chunks []*entity.Chunk) (float32, string, error) {
	contextText := buildContextText(chunks)

	prompt := fmt.Sprintf(`You are an expert evaluator. Your task is to evaluate the Context Precision of a retrieval system.
Does the retrieved context contain the information needed to answer the query?
Score 1.0 if the most relevant information is at the very top.
Score 0.0 if the context is completely useless for the query.

[Query]
%s

[Retrieved Contexts]
%s

Please output your response strictly in the following format:
Score: <float between 0.0 and 1.0>
Reason: <brief explanation>`, query, contextText)

	return j.parseEvalResponse(ctx, prompt)
}

// --- Internal Parsers ---

func buildContextText(chunks []*entity.Chunk) string {
	var sb strings.Builder
	for i, chunk := range chunks {
		sb.WriteString(fmt.Sprintf("\n--- Chunk %d ---\n%s\n", i+1, chunk.Content))
	}
	return sb.String()
}

func (j *RagasLLMJudge) parseEvalResponse(ctx context.Context, prompt string) (float32, string, error) {
	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}
	response, err := j.judgeLLM.Chat(ctx, messages)
	if err != nil {
		return 0, "", err
	}

	// Parse "Score: 0.8\nReason: The answer..."
	lines := strings.Split(response.Content, "\n")
	var score float32 = 0.0
	var reason string = "Could not parse reason"

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Score:") {
			scoreStr := strings.TrimSpace(strings.TrimPrefix(line, "Score:"))
			if s, err := strconv.ParseFloat(scoreStr, 32); err == nil {
				score = float32(s)
			}
		} else if strings.HasPrefix(line, "Reason:") {
			reason = strings.TrimSpace(strings.TrimPrefix(line, "Reason:"))
		}
	}

	return score, reason, nil
}
