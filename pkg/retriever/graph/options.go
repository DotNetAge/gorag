package graph

import (
	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// Options configures the GraphRAG retriever.
type Options struct {
	name              string
	topK              int
	depth             int
	limit             int
	promptTemplate    string
	docStore          core.DocStore
	logger            logging.Logger
	tracer            observability.Tracer
	embedder          embedding.Provider
	llm               chat.Client
	vectorStore       core.VectorStore
	graphStore        core.GraphStore
	customSteps       []pipeline.Step[*core.RetrievalContext]
	entityExtractor   core.EntityExtractor
	extractionStrategy ExtractionStrategy
	searchMode        core.SearchMode // New: Local, Global, or Hybrid
}

// ExtractionStrategy defines the entity extraction strategy.
type ExtractionStrategy string

const (
	ExtractionStrategyLLM     ExtractionStrategy = "llm"
	ExtractionStrategyVector  ExtractionStrategy = "vector"
	ExtractionStrategyKeyword ExtractionStrategy = "keyword"
	ExtractionStrategyAuto    ExtractionStrategy = "auto"
)

type Option func(*Options)

func defaultOptions() *Options {
	return &Options{
		topK:              5,
		depth:             2,
		limit:             10,
		promptTemplate:    defaultGraphRAGPrompt,
		tracer:            observability.DefaultNoopTracer(),
		customSteps:       make([]pipeline.Step[*core.RetrievalContext], 0),
		extractionStrategy: ExtractionStrategyAuto,
		searchMode:        core.SearchModeLocal, // Default to local search
	}
}

// WithName sets the retriever name.
func WithName(name string) Option {
	return func(o *Options) { o.name = name }
}

// WithVectorStore sets the vector store.
func WithVectorStore(s core.VectorStore) Option {
	return func(o *Options) { o.vectorStore = s }
}

// WithGraphStore sets the graph store (required).
func WithGraphStore(s core.GraphStore) Option {
	return func(o *Options) { o.graphStore = s }
}

// WithEmbedder sets the embedding provider.
func WithEmbedder(e embedding.Provider) Option {
	return func(o *Options) { o.embedder = e }
}

// WithLLM sets the LLM client for generation.
func WithLLM(l chat.Client) Option {
	return func(o *Options) { o.llm = l }
}

// WithTopK sets the number of results to return.
func WithTopK(k int) Option {
	return func(o *Options) { o.topK = k }
}

// WithDepth sets the graph traversal depth for local search.
func WithDepth(d int) Option {
	return func(o *Options) { o.depth = d }
}

// WithLimit sets the maximum nodes per traversal.
func WithLimit(l int) Option {
	return func(o *Options) { o.limit = l }
}

// WithDocStore sets the document store for chunk enrichment.
func WithDocStore(s core.DocStore) Option {
	return func(o *Options) { o.docStore = s }
}

// WithPromptTemplate sets the generation prompt template.
func WithPromptTemplate(t string) Option {
	return func(o *Options) {
		if t != "" {
			o.promptTemplate = t
		}
	}
}

// WithLogger sets the logger.
func WithLogger(l logging.Logger) Option {
	return func(o *Options) { o.logger = l }
}

// WithTracer sets the tracer.
func WithTracer(t observability.Tracer) Option {
	return func(o *Options) { o.tracer = t }
}

// WithCustomStep adds a custom pipeline step.
func WithCustomStep(step pipeline.Step[*core.RetrievalContext]) Option {
	return func(o *Options) {
		if step != nil {
			o.customSteps = append(o.customSteps, step)
		}
	}
}

// WithEntityExtractor sets a custom entity extractor.
func WithEntityExtractor(extractor core.EntityExtractor) Option {
	return func(o *Options) { o.entityExtractor = extractor }
}

// WithExtractionStrategy sets the entity extraction strategy.
func WithExtractionStrategy(strategy ExtractionStrategy) Option {
	return func(o *Options) { o.extractionStrategy = strategy }
}

// WithSearchMode sets the GraphRAG search mode.
// Options: SearchModeLocal, SearchModeGlobal, SearchModeHybrid.
func WithSearchMode(mode core.SearchMode) Option {
	return func(o *Options) { o.searchMode = mode }
}
