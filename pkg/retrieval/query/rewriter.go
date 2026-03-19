package query

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
)

// Rewriter uses an LLM to rewrite and expand the user's query.
type Rewriter struct {
	llm chat.Client
}

// NewRewriter creates a new query rewriter.
func NewRewriter(llm chat.Client) *Rewriter {
	return &Rewriter{llm: llm}
}

// Rewrite rewrites the user's query to improve search quality.
func (r *Rewriter) Rewrite(ctx context.Context, query *core.Query) (*core.Query, error) {
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
