package retrieval

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// IntentType represents the classified intent of a query.
type IntentType string

const (
	IntentChat           IntentType = "chat"
	IntentDomainSpecific IntentType = "domain_specific"
	IntentFactCheck      IntentType = "fact_check"
)

// IntentResult holds the result of intent classification.
type IntentResult struct {
	Intent     IntentType
	Confidence float32
	Reason     string
}

// IntentClassifier classifies user queries into different intents.
type IntentClassifier interface {
	Classify(ctx context.Context, query *entity.Query) (*IntentResult, error)
}

// DecompositionResult holds the result of query decomposition.
type DecompositionResult struct {
	SubQueries []string
	Reasoning  string
	IsComplex  bool
}

// QueryDecomposer breaks down complex queries into simpler sub-queries.
type QueryDecomposer interface {
	Decompose(ctx context.Context, query *entity.Query) (*DecompositionResult, error)
}

// CRAGLabel represents the quality assessment label.
type CRAGLabel string

const (
	CRAGRelevant   CRAGLabel = "relevant"
	CRAGAmbiguous  CRAGLabel = "ambiguous"
	CRAGIrrelevant CRAGLabel = "irrelevant"
)

// CRAGEvaluation holds the result of CRAG quality evaluation.
type CRAGEvaluation struct {
	Relevance float32
	Label     CRAGLabel
	Reason    string
}

// CRAGEvaluator evaluates the quality of retrieved context.
type CRAGEvaluator interface {
	Evaluate(ctx context.Context, query *entity.Query, chunks []*entity.Chunk) (*CRAGEvaluation, error)
}

// RAGEScores is an alias for entity.RAGEScores.
// All score fields are defined on entity.RAGEScores as the single source of truth.
type RAGEScores = entity.RAGEScores

// RAGEvaluator evaluates the quality of generated answers.
type RAGEvaluator interface {
	Evaluate(ctx context.Context, query, answer, context string) (*RAGEScores, error)
}

// GenerationResult holds the generated answer.
type GenerationResult struct {
	Answer string
}

// Generator generates answers based on query and retrieved context.
type Generator interface {
	Generate(ctx context.Context, query *entity.Query, chunks []*entity.Chunk) (*GenerationResult, error)
}

// RetrievalResult holds the result of retrieval.
type RetrievalResult struct {
	Chunks []*entity.Chunk
	Scores []float32
}

// Retriever retrieves relevant chunks based on queries.
type Retriever interface {
	Retrieve(ctx context.Context, queries []string, topK int) ([]*RetrievalResult, error)
}

// EntityExtractionResult holds the result of entity extraction.
type EntityExtractionResult struct {
	Entities []string
}

// EntityExtractor extracts entities from queries.
type EntityExtractor interface {
	Extract(ctx context.Context, query *entity.Query) (*EntityExtractionResult, error)
}

// ---------------------------------------------------------------------------
// Agentic loop contracts
// ---------------------------------------------------------------------------

// ActionType is the decision made by the Agentic reasoner on each iteration.
type ActionType string

const (
	ActionRetrieve ActionType = "retrieve"
	ActionReflect  ActionType = "reflect"
	ActionFinish   ActionType = "finish"
)

// AgentAction is the output of the action-selection step.
type AgentAction struct {
	Type  ActionType // which branch to execute next
	Query string     // refined sub-query for the retrieve action (empty for reflect/finish)
}

// AgentReasoner analyses the current retrieval state and returns a natural-language
// reasoning trace that summarises what has been found and what is still missing.
type AgentReasoner interface {
	Reason(ctx context.Context, query string, retrieved [][]*entity.Chunk, answer string) (reasoning string, err error)
}

// AgentActionSelector chooses the next action (retrieve / reflect / finish) based on
// the reasoning trace and the current iteration count relative to the allowed maximum.
type AgentActionSelector interface {
	SelectAction(ctx context.Context, query, reasoning string, iteration, maxIterations int) (*AgentAction, error)
}
