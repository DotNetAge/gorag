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
	llm            core.Client
	promptTemplate string
	maxSubQueries  int
	logger         logging.Logger
	collector      observability.Collector
}

// QueryDecomposerOption configures a queryDecomposer instance.
type QueryDecomposerOption func(*queryDecomposer)

// WithDecompositionPromptTemplate overrides the default decomposition prompt.
func WithDecompositionPromptTemplate(tmpl string) QueryDecomposerOption {
	return func(d *queryDecomposer) {
		if tmpl != "" {
			d.promptTemplate = tmpl
		}
	}
}

// WithMaxSubQueries sets the maximum number of sub-queries to generate.
func WithMaxSubQueries(max int) QueryDecomposerOption {
	return func(d *queryDecomposer) {
		if max > 0 {
			d.maxSubQueries = max
		}
	}
}

// WithQueryDecomposerLogger sets a structured logger.
func WithQueryDecomposerLogger(logger logging.Logger) QueryDecomposerOption {
	return func(d *queryDecomposer) {
		if logger != nil {
			d.logger = logger
		}
	}
}

// WithQueryDecomposerCollector sets an observability collector.
func WithQueryDecomposerCollector(collector observability.Collector) QueryDecomposerOption {
	return func(d *queryDecomposer) {
		if collector != nil {
			d.collector = collector
		}
	}
}

// NewQueryDecomposer creates a new query decomposer.
//
// Required: llm.
// Optional (via options): WithDecompositionPromptTemplate, WithMaxSubQueries,
// WithQueryDecomposerLogger, WithQueryDecomposerCollector.
func NewQueryDecomposer(llm core.Client, opts ...QueryDecomposerOption) *queryDecomposer {
	d := &queryDecomposer{
		llm:            llm,
		promptTemplate: defaultDecompositionPrompt,
		maxSubQueries:  5,
		logger:         logging.NewNoopLogger(),
		collector:      observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
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
	prompt := fmt.Sprintf(d.promptTemplate, query.Text)

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
	if len(result.SubQueries) > d.maxSubQueries {
		d.logger.Debug("limiting sub-queries", map[string]interface{}{
			"original_count": len(result.SubQueries),
			"max_allowed":    d.maxSubQueries,
		})
		result.SubQueries = result.SubQueries[:d.maxSubQueries]
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
