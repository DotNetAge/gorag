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

// ensure interface implementation
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

// Decomposer is the implementation of core.QueryDecomposer.
type Decomposer struct {
	llm            chat.Client
	promptTemplate string
	maxSubQueries  int
	logger         logging.Logger
	collector      observability.Collector
}

// DecomposerOption configures a Decomposer instance.
type DecomposerOption func(*Decomposer)

// WithDecompositionPromptTemplate overrides the default decomposition prompt.
func WithDecompositionPromptTemplate(tmpl string) DecomposerOption {
	return func(d *Decomposer) {
		if tmpl != "" {
			d.promptTemplate = tmpl
		}
	}
}

// WithMaxSubQueries sets the maximum number of sub-queries to generate.
func WithMaxSubQueries(max int) DecomposerOption {
	return func(d *Decomposer) {
		if max > 0 {
			d.maxSubQueries = max
		}
	}
}

// WithDecomposerLogger sets a structured logger.
func WithDecomposerLogger(logger logging.Logger) DecomposerOption {
	return func(d *Decomposer) {
		if logger != nil {
			d.logger = logger
		}
	}
}

// WithDecomposerCollector sets an observability collector.
func WithDecomposerCollector(collector observability.Collector) DecomposerOption {
	return func(d *Decomposer) {
		if collector != nil {
			d.collector = collector
		}
	}
}

// NewDecomposer creates a new query decomposer.
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
