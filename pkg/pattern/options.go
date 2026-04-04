package pattern

import (
	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/indexer"
	graphretriever "github.com/DotNetAge/gorag/pkg/retriever/graph"
)

// --- BGE Model Options (Auto-download) ---

type bgeOpt struct{ modelName string }

func (o bgeOpt) applyNative(opts *nativeOptions) {
	provider, err := embedding.WithBEG(o.modelName, "")
	if err == nil {
		opts.indexerOpts = append(opts.indexerOpts, indexer.WithEmbedding(provider))
	}
}
func (o bgeOpt) applyGraph(opts *graphOptions) {
	provider, err := embedding.WithBEG(o.modelName, "")
	if err == nil {
		opts.indexerOpts = append(opts.indexerOpts, indexer.WithEmbedding(provider))
	}
}

// WithBGE creates a BGE embedding provider with auto-download.
// If the model doesn't exist locally, it will be downloaded automatically.
// Applicable to: NativeRAG, GraphRAG
//
// Available models:
//   - "bge-small-zh-v1.5" - Chinese BGE small model (~48MB)
//   - "bge-base-zh-v1.5" - Chinese BGE base model (~100MB)
func WithBGE(modelName string) bgeOpt {
	return bgeOpt{modelName}
}

// --- BERT Model Options (Auto-download) ---

type bertOpt struct{ modelName string }

func (o bertOpt) applyNative(opts *nativeOptions) {
	provider, err := embedding.WithBERT(o.modelName, "")
	if err == nil {
		opts.indexerOpts = append(opts.indexerOpts, indexer.WithEmbedding(provider))
	}
}
func (o bertOpt) applyGraph(opts *graphOptions) {
	provider, err := embedding.WithBERT(o.modelName, "")
	if err == nil {
		opts.indexerOpts = append(opts.indexerOpts, indexer.WithEmbedding(provider))
	}
}

// WithBERT creates a Sentence-BERT embedding provider with auto-download.
// If the model doesn't exist locally, it will be downloaded automatically.
// Applicable to: NativeRAG, GraphRAG
//
// Available models:
//   - "all-MiniLM-L6-v2" - English Sentence-BERT small model (~45MB)
//   - "all-mpnet-base-v2" - English MPNet base model (~218MB)
//   - "bert-base-uncased" - English BERT base model (~200MB)
func WithBERT(modelName string) bertOpt {
	return bertOpt{modelName}
}

// --- CLIP Model Options (Auto-download) ---

type clipOpt struct{ modelName string }

func (o clipOpt) applyNative(opts *nativeOptions) {
	provider, err := embedding.WithCLIP(o.modelName, "")
	if err == nil {
		opts.indexerOpts = append(opts.indexerOpts, indexer.WithEmbedding(provider))
	}
}
func (o clipOpt) applyGraph(opts *graphOptions) {
	provider, err := embedding.WithCLIP(o.modelName, "")
	if err == nil {
		opts.indexerOpts = append(opts.indexerOpts, indexer.WithEmbedding(provider))
	}
}

// WithCLIP creates a CLIP multimodal embedding provider with auto-download.
// If the model doesn't exist locally, it will be downloaded automatically.
// Applicable to: NativeRAG, GraphRAG
//
// Available models:
//   - "clip-vit-base-patch32" - OpenAI CLIP base model (~300MB)
func WithCLIP(modelName string) clipOpt {
	return clipOpt{modelName}
}

// --- Internal Configuration Containers ---

type nativeOptions struct {
	indexerOpts []indexer.IndexerOption
	topK        int
	llm         chat.Client

	// Pre-Retrieval enhancements (can be combined)
	enableQueryRewrite bool
	enableStepBack     bool
	enableHyDE         bool
	enableFusion       bool // Fusion: Pre-Retrieval (Decompose) + Post-Retrieval (RRF)
	fusionCount        int

	// Post-Retrieval enhancements (can be combined)
	enableParentDoc      bool
	enableSentenceWindow bool
	enablePrune          bool
	enableRerank         bool

	// Cache
	enableCache bool
}

type graphOptions struct {
	indexerOpts        []indexer.IndexerOption
	topK               int
	llm                chat.Client
	extractionStrategy graphretriever.ExtractionStrategy
}

// --- Option Interfaces ---

// NativeOption configures a NativeRAG instance.
type NativeOption interface {
	applyNative(*nativeOptions)
}

// GraphOption configures a GraphRAG instance.
type GraphOption interface {
	applyGraph(*graphOptions)
}

// --- Shared Options (Applicable to multiple RAG types) ---

type nameOpt string

func (o nameOpt) applyNative(opts *nativeOptions) { opts.indexerOpts = append(opts.indexerOpts, indexer.WithName(string(o))) }
func (o nameOpt) applyGraph(opts *graphOptions)   { opts.indexerOpts = append(opts.indexerOpts, indexer.WithName(string(o))) }

// WithName sets the name of the RAG pattern instance.
// Applicable to: NativeRAG, GraphRAG
func WithName(name string) nameOpt {
	return nameOpt(name)
}

type topKOpt int

func (o topKOpt) applyNative(opts *nativeOptions) { opts.topK = int(o) }
func (o topKOpt) applyGraph(opts *graphOptions)   { opts.topK = int(o) }

// WithTopK sets the number of top semantic results to retrieve.
// Applicable to: NativeRAG, GraphRAG
func WithTopK(k int) topKOpt {
	return topKOpt(k)
}

type openAIOpt struct{ apiKey, model string }

func (o openAIOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithOpenAI(o.apiKey, o.model))
}
func (o openAIOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithOpenAI(o.apiKey, o.model))
}

// WithOpenAI sets the OpenAI embedding model.
// Applicable to: NativeRAG, GraphRAG
func WithOpenAI(apiKey string, model string) openAIOpt {
	return openAIOpt{apiKey, model}
}

type embedderOpt struct{ provider embedding.Provider }

func (o embedderOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithEmbedding(o.provider))
}
func (o embedderOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithEmbedding(o.provider))
}

// WithEmbedding sets a custom embedding provider.
// Applicable to: NativeRAG, GraphRAG
func WithEmbedding(provider embedding.Provider) embedderOpt {
	return embedderOpt{provider}
}

type llmOpt struct{ client chat.Client }

func (o llmOpt) applyNative(opts *nativeOptions) { opts.llm = o.client }
func (o llmOpt) applyGraph(opts *graphOptions)   { opts.llm = o.client }

// WithLLM sets the LLM client for retrieval enhancement strategies.
// Required for: QueryRewrite, StepBack, HyDE, Fusion strategies in NativeRAG
// Required for: GraphRAG with LLM-based entity extraction
func WithLLM(client chat.Client) llmOpt {
	return llmOpt{client}
}

type goVectorOpt struct {
	collection, path string
	dimension        int
}

func (o goVectorOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithGoVector(o.collection, o.path, o.dimension))
}
func (o goVectorOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithGoVector(o.collection, o.path, o.dimension))
}

// WithGoVector configures local in-memory/file GoVector storage.
// Applicable to: NativeRAG, GraphRAG
func WithGoVector(collection string, path string, dimension int) goVectorOpt {
	return goVectorOpt{collection, path, dimension}
}

type milvusOpt struct {
	collection, addr string
	dimension        int
}

func (o milvusOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithMilvus(o.collection, o.addr, o.dimension))
}
func (o milvusOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithMilvus(o.collection, o.addr, o.dimension))
}

// WithMilvus configures remote Milvus vector storage.
// Applicable to: NativeRAG, GraphRAG
func WithMilvus(collection string, addr string, dimension int) milvusOpt {
	return milvusOpt{collection, addr, dimension}
}

type sqliteDocOpt struct{ path string }

func (o sqliteDocOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithSQLDoc(o.path))
}
func (o sqliteDocOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithSQLDoc(o.path))
}

// WithSQLiteDoc configures SQLite as the document store.
// Applicable to: NativeRAG, GraphRAG
func WithSQLiteDoc(path string) sqliteDocOpt {
	return sqliteDocOpt{path}
}

type boltDocOpt struct{ path string }

func (o boltDocOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithBoltDoc(o.path))
}
func (o boltDocOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithBoltDoc(o.path))
}

// WithBoltDoc configures BoltDB as the document store.
// Applicable to: NativeRAG, GraphRAG
func WithBoltDoc(path string) boltDocOpt {
	return boltDocOpt{path}
}

type charChunkerOpt struct{ size, overlap int }

func (o charChunkerOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithCharacterChunker(o.size, o.overlap))
}
func (o charChunkerOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithCharacterChunker(o.size, o.overlap))
}

// WithCharacterChunker configures a character-based text chunker.
// Applicable to: NativeRAG, GraphRAG
func WithCharacterChunker(size, overlap int) charChunkerOpt {
	return charChunkerOpt{size, overlap}
}

type tokenChunkerOpt struct {
	size, overlap int
	model         string
}

func (o tokenChunkerOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithTokenChunker(o.size, o.overlap, o.model))
}
func (o tokenChunkerOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithTokenChunker(o.size, o.overlap, o.model))
}

// WithTokenChunker configures a token-based chunker.
// Applicable to: NativeRAG, GraphRAG
func WithTokenChunker(size, overlap int, model string) tokenChunkerOpt {
	return tokenChunkerOpt{size, overlap, model}
}

type watchDirOpt []string

func (o watchDirOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithWatchDir(o...))
}
func (o watchDirOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithWatchDir(o...))
}

type consoleLoggerOpt struct{}

func (o consoleLoggerOpt) applyNative(opts *nativeOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithConsoleLogger())
}
func (o consoleLoggerOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithConsoleLogger())
}

// WithConsoleLogger enables console logging.
func WithConsoleLogger() consoleLoggerOpt {
	return consoleLoggerOpt{}
}

// --- Query Enhancement Options (NativeRAG Only) ---

type queryRewriteOpt struct{}

func (o queryRewriteOpt) applyNative(opts *nativeOptions) {
	opts.enableQueryRewrite = true
}

// WithQueryRewrite enables query rewriting for ambiguous queries.
// Uses LLM to clarify and rephrase queries before retrieval.
// Requires: WithLLM() to be set
// Applicable to: NativeRAG
func WithQueryRewrite() NativeOption {
	return queryRewriteOpt{}
}

type stepBackOpt struct{}

func (o stepBackOpt) applyNative(opts *nativeOptions) {
	opts.enableStepBack = true
}

// WithStepBack enables step-back questioning for complex queries.
// Generates abstracted high-level questions to retrieve broader context.
// Requires: WithLLM() to be set
// Applicable to: NativeRAG
func WithStepBack() NativeOption {
	return stepBackOpt{}
}

type hydeOpt struct{}

func (o hydeOpt) applyNative(opts *nativeOptions) {
	opts.enableHyDE = true
}

// WithHyDE enables hypothetical document embedding.
// Generates hypothetical documents to improve retrieval for ambiguous queries.
// Requires: WithLLM() to be set
// Applicable to: NativeRAG
func WithHyDE() NativeOption {
	return hydeOpt{}
}

// --- Retrieval Enhancement Options (NativeRAG Only) ---

type fusionOpt struct{ count int }

func (o fusionOpt) applyNative(opts *nativeOptions) {
	opts.enableFusion = true
	opts.fusionCount = o.count
}

// WithFusion enables multi-query fusion retrieval.
// Decomposes complex queries into sub-queries and fuses results using RRF.
// Requires: WithLLM() to be set
// count: number of sub-queries to generate (default: 5)
// Applicable to: NativeRAG
func WithFusion(count int) NativeOption {
	return fusionOpt{count}
}

// --- Post-Retrieval Enhancement Options (NativeRAG Only) ---

type parentDocOpt struct{}

func (o parentDocOpt) applyNative(opts *nativeOptions) {
	opts.enableParentDoc = true
}

// WithParentDoc enables parent document expansion.
// Retrieves the parent document for each chunk, providing broader context.
// Applicable to: NativeRAG
func WithParentDoc() NativeOption {
	return parentDocOpt{}
}

type sentenceWindowOpt struct{}

func (o sentenceWindowOpt) applyNative(opts *nativeOptions) {
	opts.enableSentenceWindow = true
}

// WithSentenceWindow enables sentence window expansion.
// Expands each chunk with surrounding sentences for better context.
// Applicable to: NativeRAG
func WithSentenceWindow() NativeOption {
	return sentenceWindowOpt{}
}

type contextPruneOpt struct{}

func (o contextPruneOpt) applyNative(opts *nativeOptions) {
	opts.enablePrune = true
}

// WithContextPrune enables context pruning.
// Removes irrelevant chunks to improve generation quality.
// Applicable to: NativeRAG
func WithContextPrune() NativeOption {
	return contextPruneOpt{}
}

type rerankOpt struct{}

func (o rerankOpt) applyNative(opts *nativeOptions) {
	opts.enableRerank = true
}

// WithRerank enables result reranking.
// Re-scores and reorders retrieved results for better relevance.
// Applicable to: NativeRAG
func WithRerank() NativeOption {
	return rerankOpt{}
}

// --- Cache Options (NativeRAG Only) ---

type cacheOpt struct{}

func (o cacheOpt) applyNative(opts *nativeOptions) {
	opts.enableCache = true
}

// WithCache enables semantic caching.
// Caches query results to avoid redundant LLM calls and vector searches.
// Applicable to: NativeRAG
func WithCache() NativeOption {
	return cacheOpt{}
}

// --- GraphRAG Specific Options ---

type neoGraphOpt struct{ uri, username, password, dbName string }

func (o neoGraphOpt) applyGraph(opts *graphOptions) {
	opts.indexerOpts = append(opts.indexerOpts, indexer.WithNeoGraph(o.uri, o.username, o.password, o.dbName))
}

// WithNeoGraph configures Neo4j graph database for GraphRAG.
// Applicable exclusively to: GraphRAG
func WithNeoGraph(uri, username, password, dbName string) GraphOption {
	return neoGraphOpt{uri, username, password, dbName}
}

type extractionStrategyOpt struct{ strategy graphretriever.ExtractionStrategy }

func (o extractionStrategyOpt) applyGraph(opts *graphOptions) {
	opts.extractionStrategy = o.strategy
}

// WithExtractionStrategy sets the entity extraction strategy for GraphRAG.
//
// Available strategies:
//   - "llm": Use LLM for entity extraction (most accurate, requires LLM)
//   - "vector": Use vector similarity matching (requires embedder)
//   - "keyword": Use keyword heuristics (no dependencies, fastest)
//   - "auto": Automatically select best available strategy (default)
//
// Applicable exclusively to: GraphRAG
func WithExtractionStrategy(strategy graphretriever.ExtractionStrategy) GraphOption {
	return extractionStrategyOpt{strategy}
}

// ExtractionStrategy constants for convenience
const (
	// ExtractionStrategyLLM uses LLM for entity extraction
	ExtractionStrategyLLM = graphretriever.ExtractionStrategyLLM
	// ExtractionStrategyVector uses vector similarity matching
	ExtractionStrategyVector = graphretriever.ExtractionStrategyVector
	// ExtractionStrategyKeyword uses keyword heuristics
	ExtractionStrategyKeyword = graphretriever.ExtractionStrategyKeyword
	// ExtractionStrategyAuto automatically selects the best available strategy
	ExtractionStrategyAuto = graphretriever.ExtractionStrategyAuto
)
