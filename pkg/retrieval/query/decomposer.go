// Package query provides query processing components for the RAG system.
// It includes query decomposition, rewriting, expansion, and classification capabilities
// to improve retrieval quality and handle complex user queries.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

var _ core.QueryDecomposer = (*Decomposer)(nil)

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

// Decomposer implements core.QueryDecomposer to break down complex queries.
// It uses an LLM to analyze queries and generate simpler sub-queries that can be
// processed independently, enabling multi-hop retrieval for complex questions.
//
// Query decomposition is useful for:
//   - Multi-hop questions: "What is the revenue of the company that acquired GitHub?"
//   - Comparative questions: "Compare the features of Product A and Product B"
//   - Temporal questions: "How has the market changed since 2020?"
//
// Example:
//
//	llm := openai.NewClient(apiKey)
//	decomposer := query.NewDecomposer(llm,
//	    query.WithMaxSubQueries(3),
//	    query.WithDecomposerLogger(logger),
//	)
//	result, err := decomposer.Decompose(ctx, query)
//	// result.SubQueries contains ["What company acquired GitHub?", "What is the revenue of that company?"]
type Decomposer struct {
	llm            chat.Client
	promptTemplate string
	maxSubQueries  int
	logger         logging.Logger
	collector      observability.Collector
}

// DecomposerOption configures a Decomposer instance.
type DecomposerOption func(*Decomposer)

// WithDecompositionPromptTemplate sets a custom prompt template for decomposition.
// The template should contain one %s placeholder for the query text.
//
// Parameters:
//   - tmpl: Custom prompt template (must contain %s placeholder)
//
// Returns:
//   - DecomposerOption: Configuration function
//
// Example:
//
//	decomposer := query.NewDecomposer(llm,
//	    query.WithDecompositionPromptTemplate("Custom prompt: %s"),
//	)
func WithDecompositionPromptTemplate(tmpl string) DecomposerOption {
	return func(d *Decomposer) {
		if tmpl != "" {
			d.promptTemplate = tmpl
		}
	}
}

// WithMaxSubQueries sets the maximum number of sub-queries to generate.
// Default is 5. If decomposition produces more, they are truncated.
//
// Parameters:
//   - max: Maximum number of sub-queries (must be > 0)
//
// Returns:
//   - DecomposerOption: Configuration function
func WithMaxSubQueries(max int) DecomposerOption {
	return func(d *Decomposer) {
		if max > 0 {
			d.maxSubQueries = max
		}
	}
}

// WithDecomposerLogger sets a structured logger for the decomposer.
//
// Parameters:
//   - logger: Logger implementation (if nil, no-op logger is used)
//
// Returns:
//   - DecomposerOption: Configuration function
func WithDecomposerLogger(logger logging.Logger) DecomposerOption {
	return func(d *Decomposer) {
		if logger != nil {
			d.logger = logger
		}
	}
}

// WithDecomposerCollector sets an observability collector for metrics.
//
// Parameters:
//   - collector: Metrics collector (if nil, no-op collector is used)
//
// Returns:
//   - DecomposerOption: Configuration function
func WithDecomposerCollector(collector observability.Collector) DecomposerOption {
	return func(d *Decomposer) {
		if collector != nil {
			d.collector = collector
		}
	}
}

// NewDecomposer creates a new query decomposer with the given LLM client.
// The decomposer uses default settings unless modified by options.
//
// Parameters:
//   - llm: LLM client for generating decompositions
//   - opts: Optional configuration functions
//
// Returns:
//   - *Decomposer: Configured decomposer instance
//
// Example:
//
//	decomposer := query.NewDecomposer(llm,
//	    query.WithMaxSubQueries(3),
//	    query.WithDecomposerLogger(logger),
//	)
func NewDecomposer(llm chat.Client, opts ...DecomposerOption) *Decomposer {
	d := &Decomposer{
		llm:            llm,
		promptTemplate: defaultDecompositionPrompt,
		maxSubQueries:  5,
		logger:         logging.DefaultNoopLogger(),
		collector:      observability.DefaultNoopCollector(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Decompose breaks down a complex query into simpler sub-queries.
// It uses the configured LLM to analyze the query and generate sub-queries
// that can be answered independently.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - query: The query to decompose
//
// Returns:
//   - *core.DecompositionResult: Contains sub-queries, reasoning, and complexity flag
//   - error: Any error that occurred during decomposition
//
// The result includes:
//   - SubQueries: List of simpler questions
//   - Reasoning: Explanation of the decomposition strategy
//   - IsComplex: Whether the original query was deemed complex
//
// If decomposition fails or the query is simple, returns the original query as a single sub-query.
func (d *Decomposer) Decompose(ctx context.Context, query *core.Query) (*core.DecompositionResult, error) {
	start := time.Now()
	defer func() {
		d.collector.RecordDuration("query_decomposition", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		return nil, fmt.Errorf("query is nil or empty")
	}

	prompt := fmt.Sprintf(d.promptTemplate, query.Text)
	messages := []chat.Message{chat.NewUserMessage(prompt)}

	response, err := d.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("query decomposition failed: %w", err)
	}

	var result core.DecompositionResult
	content := strings.TrimSpace(response.Content)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		result = core.DecompositionResult{
			SubQueries: []string{query.Text},
			Reasoning:  "Failed to parse LLM response, using original",
			IsComplex:  false,
		}
	}

	if len(result.SubQueries) > d.maxSubQueries {
		result.SubQueries = result.SubQueries[:d.maxSubQueries]
	}

	if len(result.SubQueries) == 0 {
		result.SubQueries = []string{query.Text}
	}

	return &result, nil
}
