package query

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
)

// Rewriter uses an LLM to rewrite and improve user queries.
// Query rewriting enhances search quality by:
//   - Removing conversational filler words
//   - Resolving ambiguous pronouns and references
//   - Making queries more specific and searchable
//   - Expanding abbreviated terms
//
// This is particularly useful for:
//   - Conversational queries: "What about their revenue?" → "What is [Company X]'s revenue?"
//   - Vague queries: "the thing" → specific entity name
//   - Informal language: "how do I fix it" → "How do I fix [specific error]"
//
// Example:
//
//	llm := openai.NewClient(apiKey)
//	rewriter := query.NewRewriter(llm)
//	rewritten, err := rewriter.Rewrite(ctx, originalQuery)
//	// rewritten.Text contains the improved query
type Rewriter struct {
	llm chat.Client
}

// NewRewriter creates a new query rewriter with the given LLM client.
//
// Parameters:
//   - llm: LLM client for query rewriting
//
// Returns:
//   - *Rewriter: Configured rewriter instance
//
// Example:
//
//	rewriter := query.NewRewriter(llm)
//	rewritten, err := rewriter.Rewrite(ctx, query)
func NewRewriter(llm chat.Client) *Rewriter {
	return &Rewriter{llm: llm}
}

// Rewrite rewrites the user's query to improve search quality.
// It uses an LLM to transform the query into a clearer, more specific form
// that is better suited for vector database search.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - query: The original query to rewrite
//
// Returns:
//   - *core.Query: New query object with rewritten text
//   - error: Any error that occurred during rewriting
//
// The rewritten query:
//   - Has a new unique ID
//   - Contains improved, clearer text
//   - Is better suited for semantic search
//
// If rewriting fails, returns the original query text in a new query object.
func (r *Rewriter) Rewrite(ctx context.Context, query *core.Query) (*core.Query, error) {
	if query == nil || query.Text == "" {
		return nil, fmt.Errorf("query is nil or empty")
	}

	prompt := fmt.Sprintf(`You are an AI assistant helping to rewrite a search query.
Please rewrite the following query to make it clearer, more specific, and better suited for a vector database search.
Remove conversational filler words. Resolve pronouns if context permits.
Only return the rewritten query, nothing else.

Original query: "%s"`, query.Text)

	messages := []chat.Message{chat.NewUserMessage(prompt)}
	response, err := r.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	rewrittenText := strings.TrimSpace(response.Content)
	if rewrittenText == "" {
		rewrittenText = query.Text
	}

	newQuery := core.NewQuery(uuid.New().String(), rewrittenText, nil)
	return newQuery, nil
}
