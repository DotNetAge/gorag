package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step = (*QueryRewriteStep)(nil)

// QueryRewriteStep is a pipeline step that rewrites the user's raw query
// to make it more suitable for vector search.
type QueryRewriteStep struct {
	rewriter retrieval.QueryRewriter
}

// NewQueryRewriteStep creates a new query rewrite step.
func NewQueryRewriteStep(rewriter retrieval.QueryRewriter) *QueryRewriteStep {
	return &QueryRewriteStep{rewriter: rewriter}
}

func (s *QueryRewriteStep) Execute(ctx context.Context, state *pipeline.State) error {
	rawQuery, ok := state.Get("query").(*entity.Query)
	if !ok {
		// If query is just a string, convert it to entity.Query
		if queryStr, ok := state.Get("query").(string); ok {
			rawQuery = entity.NewQuery("", queryStr, nil)
		} else {
			return fmt.Errorf("QueryRewriteStep: 'query' not found in state or invalid type")
		}
	}

	rewrittenQuery, err := s.rewriter.Rewrite(ctx, rawQuery)
	if err != nil {
		return fmt.Errorf("QueryRewriteStep failed to rewrite query: %w", err)
	}

	// Update the state with the rewritten query, but keep original for reference
	state.Set("original_query", rawQuery)
	state.Set("query", rewrittenQuery)

	return nil
}
