package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.RAGEvaluator = (*ragEvaluator)(nil)

const defaultFaithfulnessPrompt = `Evaluate if the answer is faithful to the provided context.
An answer is faithful if:
- All claims can be directly inferred from the context
- No external knowledge or assumptions are used
- No hallucinations or fabricated information

[Query]
%s

[Context]
%s

[Answer]
%s

Rate faithfulness from 0.0 to 1.0 and output as JSON:
{"score": 0.0-1.0}`

const defaultAnswerRelevancePrompt = `Evaluate if the answer directly addresses the query.
Consider:
- Does it answer all parts of the question?
- Is the information relevant to the query?
- Is there unnecessary or off-topic content?

[Query]
%s

[Answer]
%s

Rate answer relevance from 0.0 to 1.0 and output as JSON:
{"score": 0.0-1.0}`

const defaultContextPrecisionPrompt = `Evaluate if the most important information appears early in the context.
Good precision means critical information is in the first few chunks.

[Query]
%s

[Context - multiple chunks]
%s

Rate context precision from 0.0 to 1.0 and output as JSON:
{"score": 0.0-1.0}`

// ragEvaluator is the infrastructure implementation of retrieval.RAGEvaluator.
type ragEvaluator struct {
	llm                core.Client
	faithfulnessPrompt string
	relevancePrompt    string
	precisionPrompt    string
	logger             logging.Logger
	collector          observability.Collector
}

// RAGEvaluatorOption configures a ragEvaluator instance.
type RAGEvaluatorOption func(*ragEvaluator)

// WithRAGEvaluationPromptTemplates overrides the default evaluation prompts.
func WithRAGEvaluationPromptTemplates(faithfulness, relevance, precision string) RAGEvaluatorOption {
	return func(e *ragEvaluator) {
		if faithfulness != "" {
			e.faithfulnessPrompt = faithfulness
		}
		if relevance != "" {
			e.relevancePrompt = relevance
		}
		if precision != "" {
			e.precisionPrompt = precision
		}
	}
}

// WithRAGEvaluatorLogger sets a structured logger.
func WithRAGEvaluatorLogger(logger logging.Logger) RAGEvaluatorOption {
	return func(e *ragEvaluator) {
		if logger != nil {
			e.logger = logger
		}
	}
}

// WithRAGEvaluatorCollector sets an observability collector.
func WithRAGEvaluatorCollector(collector observability.Collector) RAGEvaluatorOption {
	return func(e *ragEvaluator) {
		if collector != nil {
			e.collector = collector
		}
	}
}

// NewRAGEvaluator creates a new RAG evaluator.
//
// Required: llm.
// Optional (via options): WithRAGEvaluationPromptTemplates, WithRAGEvaluatorLogger,
// WithRAGEvaluatorCollector.
func NewRAGEvaluator(llm core.Client, opts ...RAGEvaluatorOption) *ragEvaluator {
	e := &ragEvaluator{
		llm:                llm,
		faithfulnessPrompt: defaultFaithfulnessPrompt,
		relevancePrompt:    defaultAnswerRelevancePrompt,
		precisionPrompt:    defaultContextPrecisionPrompt,
		logger:             logging.NewNoopLogger(),
		collector:          observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Evaluate assesses the quality of generated answers using RAGAS-inspired metrics.
func (e *ragEvaluator) Evaluate(ctx context.Context, query, answer, context string) (*retrieval.RAGEScores, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("rag_evaluation", time.Since(start), nil)
	}()

	if query == "" || answer == "" {
		e.logger.Error("evaluate failed", fmt.Errorf("query and answer required"), map[string]interface{}{
			"operation": "rag_evaluation",
		})
		e.collector.RecordCount("rag_evaluation", "error", nil)
		return nil, fmt.Errorf("query and answer are required")
	}

	e.logger.Debug("evaluating RAG response", map[string]interface{}{
		"query_length":   len(query),
		"answer_length":  len(answer),
		"context_length": len(context),
	})

	scores := &retrieval.RAGEScores{}

	// Evaluate Faithfulness
	faithfulness, err := e.evaluateFaithfulness(ctx, query, answer, context)
	if err != nil {
		e.logger.Warn("faithfulness evaluation failed", map[string]interface{}{
			"error":         err,
			"default_score": 0.5,
		})
		faithfulness = 0.5 // Default score
	}
	scores.Faithfulness = faithfulness

	// Evaluate Answer Relevance
	relevance, err := e.evaluateAnswerRelevance(ctx, query, answer)
	if err != nil {
		e.logger.Warn("relevance evaluation failed", map[string]interface{}{
			"error":         err,
			"default_score": 0.5,
		})
		relevance = 0.5
	}
	scores.AnswerRelevance = relevance

	// Evaluate Context Precision
	precision, err := e.evaluateContextPrecision(ctx, query, context)
	if err != nil {
		e.logger.Warn("precision evaluation failed", map[string]interface{}{
			"error":         err,
			"default_score": 0.5,
		})
		precision = 0.5
	}
	scores.ContextPrecision = precision

	// Calculate overall score (weighted average)
	scores.OverallScore = (faithfulness*0.4 + relevance*0.4 + precision*0.2)

	// Pass threshold: overall >= 0.7
	scores.Passed = scores.OverallScore >= 0.7

	e.logger.Info("RAG evaluation completed", map[string]interface{}{
		"faithfulness": faithfulness,
		"relevance":    relevance,
		"precision":    precision,
		"overall":      scores.OverallScore,
		"passed":       scores.Passed,
	})
	e.collector.RecordCount("rag_evaluation", "success", nil)

	return scores, nil
}

// evaluateFaithfulness evaluates if the answer is faithful to the context.
func (e *ragEvaluator) evaluateFaithfulness(ctx context.Context, query, answer, context string) (float32, error) {
	prompt := fmt.Sprintf(e.faithfulnessPrompt, query, context, answer)

	response, err := e.llm.Chat(ctx, []core.Message{core.NewUserMessage(prompt)})
	if err != nil {
		return 0, err
	}

	return parseScore(response.Content)
}

// evaluateAnswerRelevance evaluates if the answer addresses the query.
func (e *ragEvaluator) evaluateAnswerRelevance(ctx context.Context, query, answer string) (float32, error) {
	prompt := fmt.Sprintf(e.relevancePrompt, query, answer)

	response, err := e.llm.Chat(ctx, []core.Message{core.NewUserMessage(prompt)})
	if err != nil {
		return 0, err
	}

	return parseScore(response.Content)
}

// evaluateContextPrecision evaluates if key information appears early.
func (e *ragEvaluator) evaluateContextPrecision(ctx context.Context, query, context string) (float32, error) {
	prompt := fmt.Sprintf(e.precisionPrompt, query, context)

	response, err := e.llm.Chat(ctx, []core.Message{core.NewUserMessage(prompt)})
	if err != nil {
		return 0, err
	}

	return parseScore(response.Content)
}

// parseScore extracts a score from LLM response.
func parseScore(content string) (float32, error) {
	content = strings.TrimSpace(content)

	// Extract JSON
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	var result struct {
		Score float32 `json:"score"`
	}

	err := json.Unmarshal([]byte(content), &result)
	if err != nil {
		return 0, fmt.Errorf("failed to parse score: %w", err)
	}

	// Validate score range
	if result.Score < 0 || result.Score > 1 {
		return 0, fmt.Errorf("score out of range: %.2f", result.Score)
	}

	return result.Score, nil
}
