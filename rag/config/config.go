package config

import (
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedding"
	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/rag/retrieval"
	"github.com/DotNetAge/gorag/vectorstore"
)

// EngineConfig represents the configuration for the RAG engine
type EngineConfig struct {
	Parsers             map[string]parser.Parser   // Map of file extensions to parsers
	DefaultParser       parser.Parser              // Default parser for unknown file types
	Embedder            embedding.Provider         // Embedding model provider
	Store               vectorstore.Store          // Vector storage backend
	LLM                 llm.Client                 // LLM client for generation
	Retriever           *retrieval.HybridRetriever // Hybrid retrieval (vector + keyword)
	Reranker            *retrieval.Reranker        // LLM-based reranker
	Hydration           *HyDE                      // Hypothetical Document Embeddings
	Compressor          *ContextCompressor         // Context compression
	ConversationManager *ConversationManager       // Multi-turn conversation support
	MultiHopRAG         *retrieval.MultiHopRAG     // Multi-hop RAG for complex questions
	AgenticRAG          *retrieval.AgenticRAG      // Agentic RAG with autonomous retrieval
	Cache               Cache                      // Query result cache
	Router              Router                     // Query router
	Metrics             observability.Metrics      // Metrics collector
	Logger              observability.Logger       // Logger
	Tracer              observability.Tracer       // Tracer
}

// Option configures the Engine
type Option func(*EngineConfig)

// WithParser sets the default document parser
func WithParser(p parser.Parser) Option {
	return func(cfg *EngineConfig) {
		cfg.DefaultParser = p
	}
}

// WithParsers sets multiple parsers for different formats
func WithParsers(parsers map[string]parser.Parser) Option {
	return func(cfg *EngineConfig) {
		cfg.Parsers = parsers
	}
}

// WithVectorStore sets the vector store
func WithVectorStore(s vectorstore.Store) Option {
	return func(cfg *EngineConfig) {
		cfg.Store = s
	}
}

// WithEmbedder sets the embedding provider
func WithEmbedder(e embedding.Provider) Option {
	return func(cfg *EngineConfig) {
		cfg.Embedder = e
	}
}

// WithLLM sets the LLM client
func WithLLM(l llm.Client) Option {
	return func(cfg *EngineConfig) {
		cfg.LLM = l
	}
}

// WithRetriever sets the hybrid retriever
func WithRetriever(r *retrieval.HybridRetriever) Option {
	return func(cfg *EngineConfig) {
		cfg.Retriever = r
	}
}

// WithReranker sets the reranker
func WithReranker(r *retrieval.Reranker) Option {
	return func(cfg *EngineConfig) {
		cfg.Reranker = r
	}
}

// WithCache sets the query cache
func WithCache(c Cache) Option {
	return func(cfg *EngineConfig) {
		cfg.Cache = c
	}
}

// WithRouter sets the query router
func WithRouter(r Router) Option {
	return func(cfg *EngineConfig) {
		cfg.Router = r
	}
}

// WithMetrics sets the metrics collector
func WithMetrics(m observability.Metrics) Option {
	return func(cfg *EngineConfig) {
		cfg.Metrics = m
	}
}

// WithLogger sets the logger
func WithLogger(l observability.Logger) Option {
	return func(cfg *EngineConfig) {
		cfg.Logger = l
	}
}

// WithTracer sets the tracer
func WithTracer(t observability.Tracer) Option {
	return func(cfg *EngineConfig) {
		cfg.Tracer = t
	}
}

// WithHyDE sets the HyDE instance for query enhancement
func WithHyDE(h *HyDE) Option {
	return func(cfg *EngineConfig) {
		cfg.Hydration = h
	}
}

// WithContextCompressor sets the context compressor for optimizing context
func WithContextCompressor(c *ContextCompressor) Option {
	return func(cfg *EngineConfig) {
		cfg.Compressor = c
	}
}

// WithConversationManager sets the conversation manager
func WithConversationManager(cm *ConversationManager) Option {
	return func(cfg *EngineConfig) {
		cfg.ConversationManager = cm
	}
}

// WithMultiHopRAG sets the multi-hop RAG component
func WithMultiHopRAG(multiHop *retrieval.MultiHopRAG) Option {
	return func(cfg *EngineConfig) {
		cfg.MultiHopRAG = multiHop
	}
}

// WithAgenticRAG sets the agentic RAG component
func WithAgenticRAG(agentic *retrieval.AgenticRAG) Option {
	return func(cfg *EngineConfig) {
		cfg.AgenticRAG = agentic
	}
}

// QueryOptions configures query behavior
type QueryOptions struct {
	TopK              int
	PromptTemplate    string
	Stream            bool
	UseMultiHopRAG    bool   // Use multi-hop RAG for complex questions
	UseAgenticRAG     bool   // Use agentic RAG with autonomous retrieval
	MaxHops           int    // Maximum number of hops for multi-hop RAG
	AgentInstructions string // Instructions for agentic RAG
}

// StreamResponse represents a streaming RAG query response
type StreamResponse struct {
	Chunk   string
	Sources []core.Result
	Done    bool
	Error   error
}

// Response represents the RAG query response
type Response struct {
	Answer  string
	Sources []core.Result
}

// Source represents a document source for indexing
//
// Source defines the input for the indexing process. It can represent:
// 1. A text string (Content field)
// 2. A file path (Path field)
// 3. A reader interface (Reader field)
//
// The Type field specifies the document format (e.g., ".txt", ".pdf", ".docx")
// and is used to select the appropriate parser.
//
// Example:
//
//	// Index a text string
//	source1 := rag.Source{
//	    Type:    "text",
//	    Content: "Go is an open source programming language...",
//	}
//
//	// Index a file
//	source2 := rag.Source{
//	    Type: ".pdf",
//	    Path: "/path/to/document.pdf",
//	}
type Source struct {
	Type    string      // Document type/format (e.g., "text", ".pdf", ".docx")
	Path    string      // File path (if indexing a file)
	Content string      // Text content (if indexing a string)
	Reader  interface{} // Reader interface (if indexing from a reader)
}

// Cache defines the interface for query result caching
type Cache interface {
	Get(ctx interface{}, key string) (*Response, bool)
	Set(ctx interface{}, key string, value *Response, expiration interface{})
}

// Router defines the interface for query routing
type Router interface {
	Route(ctx interface{}, question string) (RouteResult, error)
}

// RouteResult represents the result of query routing
type RouteResult struct {
	Type   string
	Params map[string]interface{}
}

// HyDE represents Hypothetical Document Embeddings for query enhancement
type HyDE struct {
	llm llm.Client
}

// NewHyDE creates a new HyDE instance
func NewHyDE(llm llm.Client) *HyDE {
	return &HyDE{llm: llm}
}

// EnhanceQuery enhances the query using HyDE
func (h *HyDE) EnhanceQuery(ctx interface{}, question string) (string, error) {
	// Implementation will be added later
	return question, nil
}

// ContextCompressor represents context compression for optimizing context window usage
type ContextCompressor struct {
	llm llm.Client
}

// NewContextCompressor creates a new context compressor
func NewContextCompressor(llm llm.Client) *ContextCompressor {
	return &ContextCompressor{llm: llm}
}

// Compress compresses the context
func (c *ContextCompressor) Compress(ctx interface{}, question string, results []core.Result) ([]core.Result, error) {
	// Implementation will be added later
	return results, nil
}

// ConversationManager manages multi-turn conversations
type ConversationManager struct {
	// Implementation will be added later
}

// NewConversationManager creates a new conversation manager
func NewConversationManager() *ConversationManager {
	return &ConversationManager{}
}
