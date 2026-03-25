package core

import (
	"context"
	"io"

	chat "github.com/DotNetAge/gochat/pkg/core"
)

// Retriever defines the interface for retrieving relevant chunks based on queries.
// It is the core component responsible for finding and ranking relevant information
type Retriever interface {
	Retrieve(ctx context.Context, queries []string, topK int) ([]*RetrievalResult, error)
}

// Generator defines the interface for generating answers based on query and retrieved context.
// It combines LLM capabilities with retrieved knowledge to produce accurate responses.
type Generator interface {
	Generate(ctx context.Context, query *Query, chunks []*Chunk) (*Result, error)
	GenerateHypotheticalDocument(ctx context.Context, query *Query) (string, error)
}

// EntityExtractor defines the interface for extracting entities from queries.
// It identifies key terms, names, and concepts to improve retrieval precision.
type EntityExtractor interface {
	Extract(ctx context.Context, query *Query) (*EntityExtractionResult, error)
}

// ResultEnhancer defines the interface for enhancing retrieval results.
// It applies post-processing techniques like reranking, expansion, or filtering to improve result quality.
type ResultEnhancer interface {
	Enhance(ctx context.Context, query *Query, chunks []*Chunk) ([]*Chunk, error)
}

// Parser defines the interface for document parsing implementations.
// Parsers convert various file formats (PDF, DOCX, Markdown, etc.) into structured Document objects.
// They support both batch and streaming parsing modes for handling files of any size.
type Parser interface {
	Parse(ctx context.Context, content []byte, metadata map[string]any) (*Document, error)
	Supports(contentType string) bool
	// 使用 io.Reader 以支持流式解析大文件
	ParseStream(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *Document, error)
	GetSupportedTypes() []string
}

// VectorStore defines the interface for vector storage and similarity search.
// It provides methods for storing embedding vectors and performing efficient nearest neighbor searches.
type VectorStore interface {
	Upsert(ctx context.Context, vectors []*Vector) error
	Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*Vector, []float32, error)
	Delete(ctx context.Context, id string) error // 统一为单 ID 删除以匹配实现
	Close(ctx context.Context) error
}

// Metrics defines the interface for observability and performance monitoring.
// It tracks key metrics like search duration, indexing time, and error rates for system monitoring.
type Metrics interface {
	// Infrastructure Metrics
	RecordSearchDuration(engine string, duration any)
	RecordSearchResult(engine string, count int)
	RecordSearchError(engine string, err error)
	RecordIndexingDuration(parser string, duration any)
	RecordIndexingResult(parser string, count int)
	RecordEmbeddingCount(count int)
	RecordVectorStoreOperations(op string, count int)

	// RAG Business Metrics
	RecordQueryCount(engine string)                            // For QPS
	RecordLLMTokenUsage(model string, prompt int, completion int) // For Cost
	RecordRAGEvaluation(metric string, score float32)           // For Quality (Faithfulness, Relevance, etc.)
}

// Reranker is a specialized ResultEnhancer for reranking retrieved chunks.
// It applies cross-encoder models or other sophisticated scoring methods to reorder results by relevance.
type Reranker interface {
	Rerank(ctx context.Context, query *Query, chunks []*Chunk) ([]*Chunk, error)
}

// IntentClassifier defines the interface for classifying query intent.
// It determines the type of query (chat, fact-check, relational, etc.) to route to appropriate retrievers.
type IntentClassifier interface {
	Classify(ctx context.Context, query *Query) (*IntentResult, error)
}

// QueryDecomposer defines the interface for decomposing complex queries into simpler sub-queries.
// It breaks down multi-hop or compound questions into atomic queries for better retrieval coverage.
type QueryDecomposer interface {
	Decompose(ctx context.Context, query *Query) (*DecompositionResult, error)
}

// CRAGEvaluator defines the interface for evaluating retrieval relevance in Corrective RAG.
// It determines whether retrieved context is relevant, irrelevant, or ambiguous to decide on web search fallback.
type CRAGEvaluator interface {
	Evaluate(ctx context.Context, query *Query, chunks []*Chunk) (*CRAGEvaluation, error)
}

// WebSearcher defines the interface for external web search capabilities (used in CRAG).
// It provides fallback to web search when internal knowledge base is insufficient.
type WebSearcher interface {
	Search(ctx context.Context, query string, topK int) ([]*Chunk, error)
}

// RAGEvaluator defines the interface for evaluating overall RAG quality.
// It assesses faithfulness, relevance, and answer quality using metrics like RAGAS.
type RAGEvaluator interface {
	Evaluate(ctx context.Context, query string, answer string, context string) (*RAGEvaluation, error)
}

// FusionEngine defines the interface for fusing multiple retrieval results.
// It combines results from different retrievers or strategies using techniques like Reciprocal Rank Fusion.
type FusionEngine interface {
	Fuse(ctx context.Context, results [][]*Chunk, topK int) ([]*Chunk, error)
	ReciprocalRankFusion(ctx context.Context, results [][]*Chunk, topK int) ([]*Chunk, error)
}

// Client is a generic LLM client interface (wrapper for gochat).
// It provides the core chat completion capabilities used by generators and other components.
type Client interface {
	chat.Client
}

// HyDEGenerator defines the interface for generating hypothetical documents for HyDE (Hypothetical Document Embeddings).
// It creates pseudo-documents that represent ideal answers, used for improving retrieval alignment.
type HyDEGenerator interface {
	Generate(ctx context.Context, query *Query) (string, error)
	GenerateHypotheticalDocument(ctx context.Context, query *Query) (string, error)
}

// FilterExtractor defines the interface for extracting search filters from queries.
// It identifies metadata constraints (date ranges, categories, etc.) to refine vector searches.
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

// AgentReasoner defines the interface for reasoning about the current retrieval state in agentic RAG.
// It analyzes retrieved information and determines if additional search or reflection is needed.
type AgentReasoner interface {
	Reason(ctx context.Context, query string, retrievedChunks [][]*Chunk, answer string) (string, error)
}

// AgentActionSelector defines the interface for selecting the next action based on reasoning in agentic RAG.
// It decides whether to retrieve more information, reflect on current findings, or finish with an answer.
type AgentActionSelector interface {
	Select(ctx context.Context, query string, reasoning string) (*AgentAction, error)
}

// SemanticCache provides semantic-based caching for queries.
// It stores and retrieves cached responses based on query similarity.
type SemanticCache interface {
	// CheckCache checks if a cached response exists for the given query.
	// Returns CacheResult with Hit=true and Answer if similarity exceeds threshold.
	// Returns CacheResult with Hit=false if no match found or similarity below threshold.
	CheckCache(ctx context.Context, query *Query) (*CacheResult, error)

	// CacheResponse stores the query and its response in the cache.
	// The cache may use query text or embedding for similarity matching.
	CacheResponse(ctx context.Context, query *Query, answer *Result) error
}

// CacheResult holds the result of a cache check operation.
type CacheResult struct {
	Hit    bool
	Answer string
}
