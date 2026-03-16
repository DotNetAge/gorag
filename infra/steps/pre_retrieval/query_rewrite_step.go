// Package pre_retrieval provides steps that process and optimize queries before retrieval.
package pre_retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// QueryRewriteStep rewrites ambiguous or unclear queries to improve retrieval precision.
type QueryRewriteStep struct {
	llm    core.Client
	logger logging.Logger
}

// NewQueryRewriteStep creates a new query rewrite step.
func NewQueryRewriteStep(llm core.Client, logger logging.Logger) *QueryRewriteStep {
	return &QueryRewriteStep{
		llm:    llm,
		logger: logger,
	}
}

// Name returns the step name.
func (s *QueryRewriteStep) Name() string {
	return "QueryRewriteStep"
}

// Execute rewrites the query to eliminate ambiguity and improve clarity.
func (s *QueryRewriteStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return nil
	}

	originalQuery := state.Query.Text

	// Build prompt for query rewriting
	prompt := s.buildRewritePrompt(originalQuery)

	// Call LLM to rewrite the query
	messages := []core.Message{
		{Role: core.RoleUser, Content: []core.ContentBlock{{Type: core.ContentTypeText, Text: prompt}}},
	}

	response, err := s.llm.Chat(ctx, messages)
	if err != nil {
		s.logger.Warn("Query rewrite failed", map[string]interface{}{
			"error": err.Error(),
			"query": originalQuery,
		})
		// Continue with original query if rewrite fails
		return nil
	}

	rewrittenQuery := response.Content
	if rewrittenQuery == "" {
		s.logger.Warn("Query rewrite returned empty result", map[string]interface{}{
			"query": originalQuery,
		})
		return nil
	}

	// Update the query in state
	state.Query.Text = rewrittenQuery

	// Store original query for reference
	if state.OriginalQuery == nil {
		state.OriginalQuery = entity.NewQuery(state.Query.ID, originalQuery, state.Query.Metadata)
	}

	s.logger.Debug("Query rewritten", map[string]interface{}{
		"original":  originalQuery,
		"rewritten": rewrittenQuery,
	})

	return nil
}

// buildRewritePrompt constructs the prompt for query rewriting.
func (s *QueryRewriteStep) buildRewritePrompt(query string) string {
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
