package indexer

import (
	"context"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/indexing/chunker"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/config/types"
	"github.com/DotNetAge/gorag/pkg/indexing/store/bolt"
	"github.com/DotNetAge/gorag/pkg/indexing/store/neo4j"
	"github.com/DotNetAge/gorag/pkg/indexing/store/sqlite"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/milvus"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/pinecone"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/qdrant"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/weaviate"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// IndexerOption defines a function to configure the indexer.
type IndexerOption func(*defaultIndexer)

// WithName sets a unique name for the indexer instance, used for resource isolation.
func WithName(name string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.name = name
	}
}

// WithConcurrency enables or disables concurrent indexing.
func WithConcurrency(enabled bool) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.config.Concurrency = enabled
	}
}

// WithWorkers sets the number of workers for concurrent indexing.
func WithWorkers(workers int) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.config.Workers = workers
	}
}

// WithParsers adds custom parsers to the registry.
func WithParsers(parsers ...core.Parser) IndexerOption {
	return func(idx *defaultIndexer) {
		if idx.registry == nil {
			idx.registry = types.NewParserRegistry()
		}
		for _, parser := range parsers {
			p := parser // capture
			// Register custom parser dynamically
			idx.registry.Register(func() core.Parser { return p })
		}
	}
}

// WithAllParsers enables all available builtin parsers using the global factory registry.
func WithAllParsers() IndexerOption {
	return func(idx *defaultIndexer) {
		idx.registry = types.DefaultRegistry
	}
}

// ClearParsers clears the current parser registry.
func ClearParsers() IndexerOption {
	return func(idx *defaultIndexer) {
		idx.registry = types.NewParserRegistry()
	}
}

// WithVectorStore sets a custom vector store.
func WithVectorStore(s core.VectorStore) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore = s
	}
}

// WithDocStore sets a custom document store.
func WithDocStore(s store.DocStore) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.docStore = s
	}
}

// WithWatchDir adds directories to watch for changes.
func WithWatchDir(dirs ...string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.watchDirs = append(idx.watchDirs, dirs...)
	}
}

// --- Default Component Options (Zero Configuration) ---

// WithDefaultGoVector configures the indexer with an out-of-the-box local GoVector store.
func WithDefaultGoVector() IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore, _ = govector.DefaultStore()
	}
}

// WithDefaultSQLiteDoc configures the indexer with an out-of-the-box local SQLite doc store.
func WithDefaultSQLiteDoc() IndexerOption {
	return func(idx *defaultIndexer) {
		idx.docStore, _ = sqlite.DefaultDocStore()
	}
}

// WithDefaultSemanticChunker configures the indexer with an out-of-the-box Semantic chunker.
func WithDefaultSemanticChunker() IndexerOption {
	return func(idx *defaultIndexer) {
		idx.chunker, _ = chunker.DefaultSemanticChunker()
	}
}

// --- Specific Chunker Options ---

// WithCharacterChunker sets a simple character-based chunker.
func WithCharacterChunker(size, overlap int) IndexerOption {
	return func(idx *defaultIndexer) {
		base := chunker.NewCharacterChunker(size, overlap)
		idx.chunker = chunker.NewSemanticChunker(base, size, size/4, overlap)
	}
}

// WithTokenChunker sets an accurate token-based chunker.
func WithTokenChunker(size, overlap int, model string) IndexerOption {
	return func(idx *defaultIndexer) {
		base, _ := chunker.NewTokenChunker(size, overlap, model)
		idx.chunker = chunker.NewSemanticChunker(base, size, size/4, overlap)
	}
}

// WithConsoleLogger configures the indexer to output logs to standard output.
func WithConsoleLogger() IndexerOption {
	return func(idx *defaultIndexer) {
		idx.logger = logging.DefaultConsoleLogger()
	}
}

// WithFileLogger configures the indexer to output logs to a specific file.
func WithFileLogger(path string) IndexerOption {
	return func(idx *defaultIndexer) {
		if l, err := logging.DefaultFileLogger(path); err == nil {
			idx.logger = l
		}
	}
}

// WithZapLogger configures the indexer to use a production-grade Zap logger with log rotation.
func WithZapLogger(path string, maxSizeMB, maxDays, maxBackups int, console bool) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.logger = logging.DefaultZapLogger(logging.ZapConfig{
			Filename:   path,
			MaxSize:    maxSizeMB,
			MaxAge:     maxDays,
			MaxBackups: maxBackups,
			Compress:   true,
			Console:    console,
		})
	}
}

// --- Specific VectorStore Options ---

func WithNeoGraph(uri, username, password, dbName string) IndexerOption {
	return func(idx *defaultIndexer) {
		gs, err := neo4j.NewGraphStore(uri, username, password, dbName)
		if err != nil {
			return
		}
		idx.graphStore = gs
	}
}

func WithBoltDoc(path string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.docStore, _ = bolt.NewDocStore(path)
	}
}

func WithSQLDoc(path string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.docStore, _ = sqlite.NewDocStore(path)
	}
}

func WithGoVector(collection string, path string, dimension int) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore, _ = govector.NewStore(
			govector.WithCollection(collection),
			govector.WithDimension(dimension),
			govector.WithDBPath(path),
		)
	}
}

func WithMilvus(collection string, addr string, dimension int) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore, _ = milvus.NewStore(addr,
			milvus.WithCollection(collection),
			milvus.WithDimension(dimension))
	}
}

func WithQdrant(collection string, host string, port int, dimension int) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore, _ = qdrant.NewStore(collection, dimension, host, port)
	}
}

func WithWeaviate(collection string, addr string, apiKey string, dimension int) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore, _ = weaviate.NewStore(collection, dimension, addr, apiKey)
	}
}

func WithPinecone(indexName string, apiKey string, dimension int) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore, _ = pinecone.NewStore(apiKey,
			pinecone.WithIndex(indexName),
			pinecone.WithDimension(dimension))
	}
}

// --- Explicit Component Options (Advanced Use) ---

// WithStore sets the vector and document stores explicitly.
func WithStore(vectorStore core.VectorStore, docStore store.DocStore) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore = vectorStore
		idx.docStore = docStore
	}
}

// WithGraph sets the graph store explicitly.
func WithGraph(graphStore store.GraphStore) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.graphStore = graphStore
	}
}

// --- Observability Options ---

// WithPrometheusMetrics configures the indexer to collect and expose metrics via Prometheus.
// It will start an HTTP server on the given address (e.g., ":8080") to serve the /metrics endpoint.
func WithPrometheusMetrics(addr string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.metrics = observability.DefaultPrometheusMetrics(addr)
	}
}

// WithOpenTelemetryTracer configures the indexer to send distributed traces to an OTel exporter.
// endpoint is the gRPC endpoint of the collector (e.g., "localhost:4317").
func WithOpenTelemetryTracer(ctx context.Context, endpoint string, serviceName string) IndexerOption {
	return func(idx *defaultIndexer) {
		_, _ = observability.DefaultOpenTelemetryTracer(ctx, endpoint, serviceName)
		// Tracer is set globally in OTel, but can also be assigned if Indexer accepts it.
	}
}

func WithBGE(modelPath string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.embedder, _ = embedding.NewBGEProvider(modelPath)
	}
}

func WithBert(modelPath string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.embedder, _ = embedding.NewSentenceBERTProvider(modelPath)
	}
}

func WithClip(modelPath string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.embedder, _ = embedding.NewCLIPProvider(modelPath)
	}
}

func WithOpenAI(apiKey string, model string) IndexerOption {
	return func(idx *defaultIndexer) {
		// Note: Requires gochat openai client
		// Using a generic way or specialized if available
		// For now using placeholder as NewOpenAIEmbedder was missing earlier
		// In real case, we'd wrap the specific provider from gochat
	}
}

// WithEmbedding sets the embedding provider explicitly.
func WithEmbedding(embedder embedding.Provider) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.embedder = embedder
	}
}

// WithMetrics sets the metrics recorder.
func WithMetrics(metrics core.Metrics) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.metrics = metrics
	}
}

// WithLogger sets the logger.
func WithLogger(logger logging.Logger) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.logger = logger
	}
}

// WithChunker sets the semantic chunker.
func WithChunker(chunker core.SemanticChunker) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.chunker = chunker
	}
}

// WithExtractor sets the entity extractor.
func WithExtractor(extractor core.EntityExtractor) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.extractor = extractor
	}
}
