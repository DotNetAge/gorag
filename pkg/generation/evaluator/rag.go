package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
)

type ragEvaluator struct {
	llm chat.Client
}

func NewRAGEvaluator(llm chat.Client) core.RAGEvaluator {
	return &ragEvaluator{llm: llm}
}

// Evaluate performs a RAGAS-style evaluation using LLM as a judge.
// It measures Faithfulness, Relevance, and Context Recall.
func (e *ragEvaluator) Evaluate(ctx context.Context, query string, answer string, contextText string) (*core.RAGEvaluation, error) {
	prompt := fmt.Sprintf(`### Role: RAG Quality Auditor
As an expert RAG auditor, evaluate the following RAG response based on the provided query and retrieved context.

### Input Data
- **User Query:** %s
- **Generated Answer:** %s
- **Retrieved Context:** %s

### Evaluation Criteria (0.0 to 1.0)
1. **Faithfulness**: Is the answer derived solely from the provided context without hallucinations? (1.0 = perfect, 0.0 = completely hallucinated)
2. **Relevance**: Does the answer directly address the user's query effectively? (1.0 = highly relevant, 0.0 = off-topic)
3. **Context Recall**: Is the information necessary to answer the query present in the retrieved context? (1.0 = fully present, 0.0 = missing necessary info)

### Output Format
Return ONLY a JSON object in this format:
{
  "faithfulness": 0.85,
  "relevance": 0.9,
  "context_recall": 0.75,
  "reasoning": "brief explanation"
}`, query, answer, contextText)

	messages := []chat.Message{chat.NewUserMessage(prompt)}
	resp, err := e.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("evaluator LLM call failed: %w", err)
	}

	content := resp.Content
	return e.parseEvalResponse(content)
}

func (e *ragEvaluator) parseEvalResponse(content string) (*core.RAGEvaluation, error) {
	// Extract JSON from potential Markdown code blocks
	re := regexp.MustCompile("(?s)\\{.*\\}")
	match := re.FindString(content)
	if match == "" {
		return nil, fmt.Errorf("failed to find JSON in LLM response")
	}

	var raw struct {
		Faithfulness float32 `json:"faithfulness"`
		Relevance    float32 `json:"relevance"`
		Recall       float32 `json:"context_recall"`
		Reasoning    string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(match), &raw); err != nil {
		// Fallback: If JSON parsing fails, try manual extraction
		return e.manualParse(content)
	}

	return &core.RAGEvaluation{
		Faithfulness:  raw.Faithfulness,
		Relevance:     raw.Relevance,
		ContextRecall: raw.Recall,
		OverallScore:  (raw.Faithfulness + raw.Relevance + raw.Recall) / 3.0,
		Passed:        raw.Faithfulness > 0.6 && raw.Relevance > 0.6,
		Feedback:      raw.Reasoning,
	}, nil
}

func (e *ragEvaluator) manualParse(content string) (*core.RAGEvaluation, error) {
	res := &core.RAGEvaluation{OverallScore: 0.5, Passed: true}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.ToLower(line)
		if strings.Contains(line, "faithfulness") {
			res.Faithfulness = e.extractFloat(line)
		}
		if strings.Contains(line, "relevance") {
			res.Relevance = e.extractFloat(line)
		}
		if strings.Contains(line, "recall") {
			res.ContextRecall = e.extractFloat(line)
		}
	}
	res.OverallScore = (res.Faithfulness + res.Relevance + res.ContextRecall) / 3.0
	return res, nil
}

func (e *ragEvaluator) extractFloat(s string) float32 {
	re := regexp.MustCompile(`0?\.\d+`)
	match := re.FindString(s)
	if val, err := strconv.ParseFloat(match, 32); err == nil {
		return float32(val)
	}
	return 0.0
}
