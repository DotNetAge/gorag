// Package rewrite provides query rewriting steps for RAG retrieval pipelines.
package rewrite

import (
	"context"
	"fmt"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// rewrite rewrites ambiguous queries to improve retrieval precision.
type rewrite struct {
	llm     chat.Client
	logger  logging.Logger
	metrics core.Metrics
}

// Rewrite creates a new query rewriting step with logger and metrics.
//
// Parameters:
//   - llm: LLM client for query rewriting
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(rewrite.Rewrite(llm, logger, metrics))
func Rewrite(
	llm core.Client,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &rewrite{
		llm:     llm,
		logger:  logger,
		metrics: metrics,
	}
}

// Name returns the step name
func (s *rewrite) Name() string {
	return "QueryRewrite"
}

// Execute rewrites the query to eliminate ambiguity and improve clarity.
func (s *rewrite) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if state.Query == nil || state.Query.Text == "" {
		return nil
	}

	originalQuery := state.Query.Text

	// Build prompt for query rewriting
	prompt := s.buildRewritePrompt(originalQuery)

	// Call LLM to rewrite the query
	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}

	response, err := s.llm.Chat(ctx, messages)
	if err != nil {
		s.logger.Warn("query rewrite failed", map[string]interface{}{
			"step":  "QueryRewrite",
			"error": err.Error(),
			"query": originalQuery,
		})
		// Continue with original query if rewrite fails
		return nil
	}

	rewrittenQuery := response.Content
	if rewrittenQuery == "" {
		s.logger.Warn("query rewrite returned empty result", map[string]interface{}{
			"step":  "QueryRewrite",
			"query": originalQuery,
		})
		return nil
	}

	// Update the query in state
	state.Query.Text = rewrittenQuery

	// Store original query for reference
	if state.OriginalQuery == "" {
		state.OriginalQuery = originalQuery
	}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("rewrite", 1)
	}

	s.logger.Debug("query rewritten", map[string]interface{}{
		"step":      "QueryRewrite",
		"original":  originalQuery,
		"rewritten": rewrittenQuery,
	})

	return nil
}

// buildRewritePrompt constructs the prompt for query rewriting.
func (s *rewrite) buildRewritePrompt(query string) string {
	return fmt.Sprintf(`You are an expert at clarifying ambiguous queries for search and retrieval systems.

Your task is to rewrite the following query to make it clearer and more specific, while preserving its original intent.

Guidelines:
1. Replace pronouns (it, they, this, that) with specific entities
2. Expand abbreviations to their full forms
3. Clarify vague expressions
4. Keep the query concise but complete
5. Preserve domain-specific terminology
6. Do NOT add information not implied in the original query

Original Query: "%s"

Rewritten Query:`, query)
}
