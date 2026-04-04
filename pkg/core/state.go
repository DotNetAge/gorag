package core

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// IndexingContext provides context for document indexing pipelines.
// It tracks the state and artifacts throughout the indexing process,
// from document parsing through chunking, embedding, and graph extraction.
//
// Fields:
//   - Ctx: Context for cancellation and timeout
//   - FilePath: Path to the source file being indexed
//   - Metadata: File metadata (source, name, size, etc.)
//   - Tracer: OpenTelemetry tracer for distributed tracing
//   - Span: Current tracing span
//   - Documents: Channel for streaming parsed documents
//   - Chunks: Channel for streaming chunks
//   - ProcessedChunks: Final processed chunks ready for storage
//   - Vectors: Generated embedding vectors
//   - Nodes: Extracted graph nodes (entities)
//   - Edges: Extracted graph edges (relationships)
//   - Triples: Extracted knowledge triples
//   - Communities: Detected graph communities
//   - TotalChunks: Count of total chunks processed
//   - Custom: Custom data for pipeline extensions
type IndexingContext struct {
	Ctx      context.Context `json:"-"`
	FilePath string          `json:"file_path,omitempty"`
	Metadata Metadata        `json:"metadata,omitempty"`

	Tracer observability.Tracer `json:"-"`
	Span   observability.Span   `json:"-"`

	Documents <-chan *Document `json:"-"`
	Chunks    <-chan *Chunk    `json:"-"`

	ProcessedChunks []*Chunk  `json:"processed_chunks,omitempty"`
	Vectors         []*Vector `json:"vectors,omitempty"`
	Nodes           []*Node   `json:"nodes,omitempty"`
	Edges           []*Edge   `json:"edges,omitempty"`

	Triples     []*Triple     `json:"triples,omitempty"`
	Communities []*Community  `json:"communities,omitempty"`

	TotalChunks int            `json:"total_chunks,omitempty"`
	Custom      map[string]any `json:"custom,omitempty"`
}

// NewIndexingContext creates a new indexing context with default values.
// It initializes the metadata with the file path and sets up a no-op tracer.
//
// Parameters:
//   - ctx: Parent context for cancellation
//   - filePath: Path to the file being indexed
//
// Returns:
//   - *IndexingContext: Initialized context ready for use
func NewIndexingContext(ctx context.Context, filePath string) *IndexingContext {
	return &IndexingContext{
		Ctx:      ctx,
		FilePath: filePath,
		Metadata: Metadata{Source: filePath, FileName: filePath},
		Tracer:   observability.DefaultNoopTracer(),
		Custom:   make(map[string]any),
	}
}

// RetrievalContext provides context for retrieval and generation pipelines.
// It tracks the state throughout the query processing, from initial query
// through retrieval, reranking, and answer generation.
//
// Fields:
//   - Ctx: Context for cancellation and timeout
//   - OriginalQuery: The user's original query text
//   - Query: Current query being processed (may be rewritten)
//   - Tracer: OpenTelemetry tracer for distributed tracing
//   - Span: Current tracing span
//   - RetrievedChunks: Chunks retrieved from multiple sources
//   - ParallelResults: Named intermediate results from parallel retrieval
//   - RerankScores: Scores from reranking step
//   - Filters: Extracted metadata filters
//   - SearchMode: GraphRAG search mode (local/global/hybrid)
//   - ExtractedEntities: Entities extracted from query
//   - GraphNodes: Retrieved graph nodes
//   - GraphEdges: Retrieved graph edges
//   - GraphContext: Formatted graph context for generation
//   - CommunityMatches: Matched communities for global search
//   - Agentic: Agentic RAG state (sub-queries, HyDE, etc.)
//   - Answer: Final generated answer
//   - Custom: Custom data for pipeline extensions
//   - Metrics: Performance and quality metrics
type RetrievalContext struct {
	Ctx context.Context `json:"-"`

	OriginalQuery string `json:"original_query"`
	Query         *Query `json:"query"`

	Tracer observability.Tracer `json:"-"`
	Span   observability.Span   `json:"-"`

	RetrievedChunks [][]*Chunk          `json:"retrieved_chunks"`
	ParallelResults map[string][]*Chunk `json:"parallel_results"`
	RerankScores    []float32           `json:"rerank_scores,omitempty"`
	Filters         map[string]any      `json:"filters,omitempty"`

	SearchMode        SearchMode          `json:"search_mode,omitempty"`
	ExtractedEntities []string            `json:"extracted_entities,omitempty"`
	GraphNodes        []*Node             `json:"graph_nodes,omitempty"`
	GraphEdges        []*Edge             `json:"graph_edges,omitempty"`
	GraphContext      string              `json:"graph_context,omitempty"`
	CommunityMatches  []*CommunityMatch   `json:"community_matches,omitempty"`

	Agentic *AgenticContext `json:"agentic,omitempty"`

	Answer *Result `json:"answer"`

	Custom map[string]any `json:"custom,omitempty"`

	Metrics map[string]any `json:"metrics,omitempty"`
}

// NewRetrievalContext creates a new retrieval context with default values.
// It initializes all required maps and sets up a no-op tracer.
//
// Parameters:
//   - ctx: Parent context for cancellation
//   - queryText: The user's query text
//
// Returns:
//   - *RetrievalContext: Initialized context ready for use
func NewRetrievalContext(ctx context.Context, queryText string) *RetrievalContext {
	return &RetrievalContext{
		Ctx:             ctx,
		OriginalQuery:   queryText,
		Query:           &Query{Text: queryText},
		Tracer:          observability.DefaultNoopTracer(),
		RetrievedChunks: make([][]*Chunk, 0),
		ParallelResults: make(map[string][]*Chunk),
		Filters:         make(map[string]any),
		Agentic:         NewAgenticState(),
		Answer:          &Result{},
		Custom:          make(map[string]any),
		Metrics:         make(map[string]any),
	}
}

// AgenticContext stores intermediate state for agentic reasoning and advanced RAG techniques.
// It tracks the state for techniques like query decomposition, HyDE, step-back prompting, etc.
//
// Fields:
//   - NextStep: The next action to take in the agent loop
//   - Intent: Classified intent of the query
//   - History: Conversation history for multi-turn context
//   - Metadata: Additional metadata for agent reasoning
//   - SubQueries: Decomposed sub-queries for multi-hop reasoning
//   - HypotheticalDocument: Generated hypothetical document for HyDE
//   - HydeApplied: Whether HyDE has been applied
//   - StepBackQuery: Generated step-back query for abstract reasoning
//   - CacheHit: Whether a cache hit occurred
//   - Filters: Extracted filters for refined search
//   - Custom: Custom data for agent extensions
type AgenticContext struct {
	NextStep             string         `json:"next_step,omitempty"`
	Intent               string         `json:"intent,omitempty"`
	History              []chat.Message `json:"history,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	SubQueries           []string       `json:"sub_queries,omitempty"`
	HypotheticalDocument string         `json:"hypothetical_document,omitempty"`
	HydeApplied          bool           `json:"hyde_applied,omitempty"`
	StepBackQuery        string         `json:"step_back_query,omitempty"`
	CacheHit             bool           `json:"cache_hit,omitempty"`
	Filters              map[string]any `json:"filters,omitempty"`
	Custom               map[string]any `json:"custom,omitempty"`
}

// NewAgenticState creates a new agentic context with initialized maps.
//
// Returns:
//   - *AgenticContext: Initialized agentic context
func NewAgenticState() *AgenticContext {
	return &AgenticContext{
		Metadata: make(map[string]any),
		SubQueries: make([]string, 0),
		Custom:     make(map[string]any),
	}
}

// Metadata represents file metadata for indexing operations.
//
// Fields:
//   - Source: Source location (file path or URL)
//   - FileName: Name of the file
//   - Size: File size in bytes
//   - ModTime: Last modification time
type Metadata struct {
	Source   string `json:"source"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	ModTime  any    `json:"mod_time,omitempty"`
}

// Result represents a unified result object for RAG operations.
// It contains the generated answer and a confidence score.
//
// Fields:
//   - Answer: The generated answer text
//   - Score: Confidence or relevance score (0.0 to 1.0)
type Result struct {
	Answer string  `json:"answer"`
	Score  float32 `json:"score"`
}
