package core

import (
	"context"
	"io"

	chat "github.com/DotNetAge/gochat/pkg/core"
)

// Retriever retrieves relevant chunks based on queries.
type Retriever interface {
	Retrieve(ctx context.Context, queries []string, topK int) ([]*RetrievalResult, error)
}

// Generator generates answers based on query and retrieved context.
type Generator interface {
	Generate(ctx context.Context, query *Query, chunks []*Chunk) (*Result, error)
	GenerateHypotheticalDocument(ctx context.Context, query *Query) (string, error)
}

// EntityExtractor extracts entities from queries.
type EntityExtractor interface {
	Extract(ctx context.Context, query *Query) (*EntityExtractionResult, error)
}

// ResultEnhancer enhances retrieval results.
type ResultEnhancer interface {
	Enhance(ctx context.Context, query *Query, chunks []*Chunk) ([]*Chunk, error)
}

// Parser defines the interface for document parsing.
type Parser interface {
	Parse(ctx context.Context, content []byte, metadata map[string]any) (*Document, error)
	Supports(contentType string) bool
	// 使用 io.Reader 以支持流式解析大文件
	ParseStream(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *Document, error)
	GetSupportedTypes() []string
}

// VectorStore defines the interface for vector storage.
type VectorStore interface {
	Upsert(ctx context.Context, vectors []*Vector) error
	Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*Vector, []float32, error)
	Delete(ctx context.Context, id string) error // 统一为单 ID 删除以匹配实现
	Close(ctx context.Context) error
}

// Metrics defines the interface for observability.
type Metrics interface {
	RecordSearchDuration(engine string, duration any)
	RecordSearchResult(engine string, count int)
	RecordSearchError(engine string, err error)
	RecordIndexingDuration(parser string, duration any)
	RecordIndexingResult(parser string, count int)
	RecordEmbeddingCount(count int)
	RecordVectorStoreOperations(op string, count int)
}

// Reranker is a specialized enhancer for reranking.
type Reranker interface {
	Rerank(ctx context.Context, query *Query, chunks []*Chunk) ([]*Chunk, error)
}

// IntentClassifier classifies query intent.
type IntentClassifier interface {
	Classify(ctx context.Context, query *Query) (*IntentResult, error)
}

// QueryDecomposer decomposes complex queries.
type QueryDecomposer interface {
	Decompose(ctx context.Context, query *Query) (*DecompositionResult, error)
}

// CRAGEvaluator evaluates retrieval relevance for Corrective RAG.
type CRAGEvaluator interface {
	Evaluate(ctx context.Context, query *Query, chunks []*Chunk) (*CRAGEvaluation, error)
}

// WebSearcher defines the interface for external web search (used in CRAG).
type WebSearcher interface {
	Search(ctx context.Context, query string, topK int) ([]*Chunk, error)
}

// RAGEvaluator evaluates overall RAG quality.
type RAGEvaluator interface {
	Evaluate(ctx context.Context, query string, answer string, context string) (*RAGEvaluation, error)
}

// FusionEngine fuses multiple retrieval results.
type FusionEngine interface {
	Fuse(ctx context.Context, results [][]*Chunk, topK int) ([]*Chunk, error)
	ReciprocalRankFusion(ctx context.Context, results [][]*Chunk, topK int) ([]*Chunk, error)
}

// Client is a generic LLM client interface (wrapper for gochat).
type Client interface {
	chat.Client
}

// HyDEGenerator generates hypothetical documents for HyDE.
type HyDEGenerator interface {
	Generate(ctx context.Context, query *Query) (string, error)
	GenerateHypotheticalDocument(ctx context.Context, query *Query) (string, error)
}

// FilterExtractor extracts search filters from queries.
type FilterExtractor interface {
	Extract(ctx context.Context, query *Query) (map[string]any, error)
}

// AgentActionType represents the type of action in agentic RAG.
type AgentActionType string

const (
	ActionRetrieve AgentActionType = "retrieve"
	ActionReflect  AgentActionType = "reflect"
	ActionFinish   AgentActionType = "finish"
)

// AgentAction represents an action decided by the agent.
type AgentAction struct {
	Type  AgentActionType
	Query string // Used when Type is ActionRetrieve
}

// AgentReasoner reasons about the current retrieval state.
type AgentReasoner interface {
	Reason(ctx context.Context, query string, retrievedChunks [][]*Chunk, answer string) (string, error)
}

// AgentActionSelector selects the next action based on reasoning.
type AgentActionSelector interface {
	Select(ctx context.Context, query string, reasoning string) (*AgentAction, error)
}
