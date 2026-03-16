package retrieval

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// QueryRewriter optimizes the user's raw query (e.g., coreference resolution).
type QueryRewriter interface {
	Rewrite(ctx context.Context, query *entity.Query) (*entity.Query, error)
}

// StepBackGenerator abstracts specific questions into broader background questions.
type StepBackGenerator interface {
	GenerateStepBackQuery(ctx context.Context, query *entity.Query) (*entity.Query, error)
}

// HyDEGenerator generates hypothetical documents to improve dense vector matching.
type HyDEGenerator interface {
	GenerateHypotheticalDocument(ctx context.Context, query *entity.Query) (*entity.Document, error)
}

// FilterExtractor uses LLM or rules to extract hard metadata constraints from natural language.
type FilterExtractor interface {
	ExtractFilters(ctx context.Context, query *entity.Query) (map[string]any, error)
}

// ResultEnhancer enhances retrieval results (e.g., reranking, pruning, expansion).
type ResultEnhancer interface {
	Enhance(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error)
}
