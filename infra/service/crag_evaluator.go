package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.CRAGEvaluator = (*cragEvaluator)(nil)

// cragEvaluatorConfig holds configuration for CRAG evaluation.
type cragEvaluatorConfig struct {
	PromptTemplate string
}

// DefaultCRAGEvaluatorConfig returns a default configuration.
func DefaultCRAGEvaluatorConfig() cragEvaluatorConfig {
	return cragEvaluatorConfig{
		PromptTemplate: defaultCRAGEvaluationPrompt,
	}
}

const defaultCRAGEvaluationPrompt = `You are an expert evaluator of retrieved context quality for RAG systems.
Your task is to assess how relevant the retrieved documents are to the user's query.

Rate the relevance on a scale from 0.0 to 1.0, then classify into one of three categories:
- **relevant** (0.7-1.0): Documents contain information directly useful for answering the query
- **ambiguous** (0.4-0.7): Documents contain some relevant information but also significant noise
- **irrelevant** (0.0-0.4): Documents contain little or no useful information for the query

[Query]
%s

[Retrieved Context - %d chunks]
%s

Output your response as a valid JSON object with this exact structure:
{
  "relevance": 0.0-1.0,
  "label": "relevant|ambiguous|irrelevant",
  "reason": "brief explanation of your evaluation"
}`

// cragEvaluator is the infrastructure implementation of retrieval.CRAGEvaluator.
type cragEvaluator struct {
	llm       core.Client
	config    cragEvaluatorConfig
	logger    logging.Logger
	collector observability.Collector
}

// NewCRAGEvaluator creates a new CRAG evaluator with logger and metrics.
func NewCRAGEvaluator(llm core.Client, config cragEvaluatorConfig, logger logging.Logger, collector observability.Collector) *cragEvaluator {
	if config.PromptTemplate == "" {
		config = DefaultCRAGEvaluatorConfig()
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	if collector == nil {
		collector = observability.NewNoopCollector()
	}
	return &cragEvaluator{
		llm:       llm,
		config:    config,
		logger:    logger,
		collector: collector,
	}
}

// Evaluate assesses the quality of retrieved context.
func (e *cragEvaluator) Evaluate(ctx context.Context, query *entity.Query, chunks []*entity.Chunk) (*retrieval.CRAGEvaluation, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("crag_evaluation", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		e.logger.Error("evaluate failed", fmt.Errorf("query required"), map[string]interface{}{
			"operation": "crag_evaluation",
		})
		e.collector.RecordCount("crag_evaluation", "error", nil)
		return nil, fmt.Errorf("query is nil or empty")
	}

	if len(chunks) == 0 {
		// No chunks retrieved - automatically irrelevant
		e.logger.Debug("no chunks retrieved", map[string]interface{}{
			"query": query.Text,
		})
		e.collector.RecordCount("crag_evaluation", "success", nil)
		return &retrieval.CRAGEvaluation{
			Relevance: 0.0,
			Label:     retrieval.CRAGIrrelevant,
			Reason:    "No documents retrieved",
		}, nil
	}

	e.logger.Debug("evaluating context", map[string]interface{}{
		"query":        query.Text,
		"chunks_count": len(chunks),
	})

	// Build context string from chunks
	var contextBuilder strings.Builder
	for i, chunk := range chunks {
		contextBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, chunk.Content))
	}
	contextStr := contextBuilder.String()

	// Build prompt
	prompt := fmt.Sprintf(e.config.PromptTemplate, query.Text, len(chunks), contextStr)

	// Call LLM
	messages := []core.Message{
		core.NewUserMessage(prompt),
	}

	response, err := e.llm.Chat(ctx, messages)
	if err != nil {
		e.logger.Error("LLM chat failed", err, map[string]interface{}{
			"operation": "crag_evaluation",
			"query":     query.Text,
		})
		e.collector.RecordCount("crag_evaluation", "error", nil)
		return nil, fmt.Errorf("CRAGEvaluatorImpl.Evaluate failed to call LLM: %w", err)
	}

	// Parse JSON response
	var result retrieval.CRAGEvaluation
	content := strings.TrimSpace(response.Content)

	// Extract JSON
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		// Fallback to ambiguous
		e.logger.Warn("failed to parse evaluation", map[string]interface{}{
			"error":         err,
			"default_label": retrieval.CRAGAmbiguous,
		})
		result = retrieval.CRAGEvaluation{
			Relevance: 0.5,
			Label:     retrieval.CRAGAmbiguous,
			Reason:    "Failed to parse LLM response, using default evaluation",
		}
	}

	// Validate label matches relevance score
	if !isValidLabel(result.Relevance, result.Label) {
		// Auto-correct label based on score
		e.logger.Debug("auto-correcting label", map[string]interface{}{
			"original_label": result.Label,
			"relevance":      result.Relevance,
		})
		result.Label = scoreToLabel(result.Relevance)
	}

	e.logger.Info("CRAG evaluation completed", map[string]interface{}{
		"label":     result.Label,
		"relevance": result.Relevance,
		"query":     query.Text,
	})
	e.collector.RecordCount("crag_evaluation", "success", nil)

	return &result, nil
}

// isValidLabel checks if the label matches the relevance score.
func isValidLabel(relevance float32, label retrieval.CRAGLabel) bool {
	expected := scoreToLabel(relevance)
	return label == expected
}

// scoreToLabel converts a relevance score to a label.
func scoreToLabel(relevance float32) retrieval.CRAGLabel {
	if relevance >= 0.7 {
		return retrieval.CRAGRelevant
	} else if relevance >= 0.4 {
		return retrieval.CRAGAmbiguous
	}
	return retrieval.CRAGIrrelevant
}
