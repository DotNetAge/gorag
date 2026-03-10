package rag

import (
	"time"

	"github.com/DotNetAge/gorag/circuitbreaker"
	"github.com/DotNetAge/gorag/utils/llmutil"
	"github.com/DotNetAge/gorag/embedding"
	gochatcore "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/rag/retrieval"
	"github.com/DotNetAge/gorag/vectorstore"
)

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

// Option configures the Engine
type Option func(*Engine)

// WithParser sets the default document parser
func WithParser(p parser.Parser) Option {
	return func(e *Engine) {
		e.defaultParser = p
	}
}

// WithParsers sets multiple parsers for different formats
func WithParsers(parsers map[string]parser.Parser) Option {
	return func(e *Engine) {
		e.parsers = parsers
	}
}

// WithVectorStore sets the vector store
func WithVectorStore(s vectorstore.Store) Option {
	return func(e *Engine) {
		e.store = s
	}
}

// WithEmbedder sets the embedding provider
func WithEmbedder(e embedding.Provider) Option {
	return func(engine *Engine) {
		engine.embedder = e
	}
}

// WithLLM sets the LLM client
func WithLLM(l gochatcore.Client) Option {
	return func(e *Engine) {
		e.llm = l
	}
}

// WithRetriever sets the hybrid retriever
func WithRetriever(r *retrieval.HybridRetriever) Option {
	return func(e *Engine) {
		e.retriever = r
	}
}

// WithReranker sets the reranker
func WithReranker(r *retrieval.Reranker) Option {
	return func(e *Engine) {
		e.reranker = r
	}
}

// WithCache sets the query cache
func WithCache(c Cache) Option {
	return func(e *Engine) {
		e.cache = c
	}
}

// WithRouter sets the query router
func WithRouter(r Router) Option {
	return func(e *Engine) {
		e.router = r
	}
}

// WithMetrics sets the metrics collector
func WithMetrics(m observability.Metrics) Option {
	return func(e *Engine) {
		e.metrics = m
	}
}

// WithLogger sets the logger
func WithLogger(l observability.Logger) Option {
	return func(e *Engine) {
		e.logger = l
	}
}

// WithTracer sets the tracer
func WithTracer(t observability.Tracer) Option {
	return func(e *Engine) {
		e.tracer = t
	}
}

// WithHyDE sets the HyDE instance for query enhancement
func WithHyDE(h *HyDE) Option {
	return func(e *Engine) {
		e.hydration = h
	}
}

// WithContextCompressor sets the context compressor for optimizing context
func WithContextCompressor(c *ContextCompressor) Option {
	return func(e *Engine) {
		e.compressor = c
	}
}

// WithConversationManager sets the conversation manager
func WithConversationManager(cm *ConversationManager) Option {
	return func(e *Engine) {
		e.conversationManager = cm
	}
}

// WithMultiHopRAG sets the multi-hop RAG component
func WithMultiHopRAG(multiHop *retrieval.MultiHopRAG) Option {
	return func(e *Engine) {
		e.multiHopRAG = multiHop
	}
}

// WithAgenticRAG sets the agentic RAG component
func WithAgenticRAG(agentic *retrieval.AgenticRAG) Option {
	return func(e *Engine) {
		e.agenticRAG = agentic
	}
}

// WithPluginDirectory sets the plugin directory for the engine
func WithPluginDirectory(directory string) Option {
	return func(e *Engine) {
		e.pluginOptions.PluginDirectory = directory
	}
}

// WithPluginConfig sets the plugin configuration for the engine
func WithPluginConfig(config map[string]interface{}) Option {
	return func(e *Engine) {
		e.pluginOptions.PluginConfig = config
	}
}

// WithConnectionPool wraps the vector store with a connection pool
//
// Parameters:
// - maxConns: Maximum number of connections
// - idleConns: Maximum number of idle connections
// - idleTimeout: Idle connection timeout
//
// Returns:
// - Option: Engine option
func WithConnectionPool(maxConns, idleConns int, idleTimeout time.Duration) Option {
	return func(e *Engine) {
		if e.store != nil {
			e.store = vectorstore.NewPooledStore(e.store, vectorstore.PoolOptions{
				MaxConns:    maxConns,
				IdleConns:   idleConns,
				IdleTimeout: idleTimeout,
			})
		}
	}
}

// WithQueryCache enables query result caching
//
// Parameters:
// - ttl: Cache time-to-live
//
// Returns:
// - Option: Engine option
func WithQueryCache(ttl time.Duration, maxSize int) Option {
	return func(e *Engine) {
		e.cache = NewMemoryCache(ttl)
	}
}

// WithBatchProcessor optimizes embedding batch processing
//
// Parameters:
// - batchSize: Size of each batch
// - maxWorkers: Maximum number of concurrent workers
// - rateLimit: Minimum time between requests
//
// Returns:
// - Option: Engine option
func WithBatchProcessor(batchSize, maxWorkers int, rateLimit time.Duration) Option {
	return func(e *Engine) {
		if e.embedder != nil {
			e.embedder = embedding.NewBatchProvider(e.embedder, embedding.BatchOptions{
				BatchSize:  batchSize,
				MaxWorkers: maxWorkers,
				RateLimit:  rateLimit,
			})
		}
	}
}

// WithCircuitBreaker adds circuit breaker protection to the LLM client
//
// Parameters:
// - maxFailures: Maximum number of failures before opening the circuit
// - timeout: Time to wait before attempting to close the circuit
// - halfOpenMax: Maximum number of requests in half-open state
//
// Returns:
// - Option: Engine option
func WithCircuitBreaker(maxFailures int, timeout time.Duration, halfOpenMax int) Option {
	return func(e *Engine) {
		if e.llm != nil {
			breaker := circuitbreaker.New(circuitbreaker.Config{
				MaxFailures: maxFailures,
				Timeout:     timeout,
				HalfOpenMax: halfOpenMax,
			})
			e.llm = llmutil.NewCircuitBreakerClient(e.llm, breaker)
		}
	}
}
