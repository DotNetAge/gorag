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
var _ retrieval.QueryDecomposer = (*queryDecomposer)(nil)

// queryDecomposerConfig holds configuration for query decomposition.
type queryDecomposerConfig struct {
	PromptTemplate string
	MaxSubQueries  int
}

// DefaultQueryDecomposerConfig returns a default configuration.
func DefaultQueryDecomposerConfig() queryDecomposerConfig {
	return queryDecomposerConfig{
		PromptTemplate: defaultDecompositionPrompt,
		MaxSubQueries:  5,
	}
}

const defaultDecompositionPrompt = `You are an expert at breaking down complex questions into simpler sub-questions.
Your task is to analyze the user's query and decompose it into 2-5 simpler, independent sub-questions.

Guidelines:
1. Each sub-question should be answerable independently
2. Sub-questions should cover different aspects of the original query
3. Keep sub-questions clear and specific
4. If the query is already simple, return it as-is

[Query]
%s

Output your response as a valid JSON object with this exact structure:
{
  "sub_queries": ["sub-question 1", "sub-question 2", ...],
  "reasoning": "brief explanation of your decomposition strategy",
  "is_complex": true/false
}`

// queryDecomposer is the infrastructure implementation of retrieval.QueryDecomposer.
type queryDecomposer struct {
	llm       core.Client
	config    queryDecomposerConfig
	logger    logging.Logger
	collector observability.Collector
}

// NewQueryDecomposer creates a new query decomposer with logger and metrics.
func NewQueryDecomposer(llm core.Client, config queryDecomposerConfig, logger logging.Logger, collector observability.Collector) *queryDecomposer {
	if config.PromptTemplate == "" {
		config = DefaultQueryDecomposerConfig()
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	if collector == nil {
		collector = observability.NewNoopCollector()
	}
	return &queryDecomposer{
		llm:       llm,
		config:    config,
		logger:    logger,
		collector: collector,
	}
}

// Decompose breaks down a complex query into simpler sub-queries.
func (d *queryDecomposer) Decompose(ctx context.Context, query *entity.Query) (*retrieval.DecompositionResult, error) {
	start := time.Now()
	defer func() {
		d.collector.RecordDuration("query_decomposition", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		d.logger.Error("decompose failed", fmt.Errorf("query is nil or empty"), map[string]interface{}{
			"operation": "query_decomposition",
		})
		d.collector.RecordCount("query_decomposition", "error", nil)
		return nil, fmt.Errorf("query is nil or empty")
	}

	d.logger.Debug("decomposing query", map[string]interface{}{
		"query": query.Text,
	})

	// Build prompt
	prompt := fmt.Sprintf(d.config.PromptTemplate, query.Text)

	// Call LLM
	messages := []core.Message{
		core.NewUserMessage(prompt),
	}

	response, err := d.llm.Chat(ctx, messages)
	if err != nil {
		d.logger.Error("LLM chat failed", err, map[string]interface{}{
			"operation": "query_decomposition",
			"query":     query.Text,
		})
		d.collector.RecordCount("query_decomposition", "error", nil)
		return nil, fmt.Errorf("QueryDecomposerImpl.Decompose failed to call LLM: %w", err)
	}

	// Parse JSON response
	var result retrieval.DecompositionResult
	content := strings.TrimSpace(response.Content)

	// Extract JSON
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		// Fallback: treat as non-complex query
		d.logger.Warn("failed to parse decomposition", map[string]interface{}{
			"error":          err,
			"fallback_query": query.Text,
		})
		result = retrieval.DecompositionResult{
			SubQueries: []string{query.Text},
			Reasoning:  "Failed to parse LLM response, using original query",
			IsComplex:  false,
		}
	}

	// Limit number of sub-queries
	if len(result.SubQueries) > d.config.MaxSubQueries {
		d.logger.Debug("limiting sub-queries", map[string]interface{}{
			"original_count": len(result.SubQueries),
			"max_allowed":    d.config.MaxSubQueries,
		})
		result.SubQueries = result.SubQueries[:d.config.MaxSubQueries]
	}

	// If no sub-queries generated, use original query
	if len(result.SubQueries) == 0 {
		d.logger.Debug("no sub-queries generated", map[string]interface{}{
			"using_original": query.Text,
		})
		result.SubQueries = []string{query.Text}
	}

	d.logger.Info("query decomposed successfully", map[string]interface{}{
		"sub_queries_count": len(result.SubQueries),
		"is_complex":        result.IsComplex,
		"query":             query.Text,
	})
	d.collector.RecordCount("query_decomposition", "success", nil)

	return &result, nil
}
