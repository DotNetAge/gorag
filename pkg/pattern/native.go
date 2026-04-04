package pattern

import (
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexer"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/repository"
	nativeretriever "github.com/DotNetAge/gorag/pkg/retriever/native"
)

// ============================================================================
// NativeRAG - Configurable Vector-based RAG
// ============================================================================

type nativeRAG struct {
	basePattern
}

// NativeRAG creates a configurable Vector-based RAG pattern.
// NativeRAG supports various enhancement strategies through options:
//
// **Query Enhancement (Pre-retrieval):**
//   - WithQueryRewrite() - Clarify ambiguous queries
//   - WithStepBack()     - Generate high-level questions
//   - WithHyDE()         - Generate hypothetical documents
//
// **Retrieval Enhancement:**
//   - WithFusion(n) - Multi-query fusion with RRF
//
// Example - Simple RAG:
//
//	rag, err := pattern.NativeRAG("myapp", pattern.WithBGE("bge-small-zh-v1.5"))
//
// Example - With Query Enhancement:
//
//	rag, err := pattern.NativeRAG("myapp",
//	    pattern.WithBGE("bge-small-zh-v1.5"),
//	    pattern.WithLLM(myLLMClient),
//	    pattern.WithQueryRewrite(),
//	)
//
// Example - Full Advanced RAG:
//
//	rag, err := pattern.NativeRAG("myapp",
//	    pattern.WithBGE("bge-small-zh-v1.5"),
//	    pattern.WithLLM(myLLMClient),
//	    pattern.WithQueryRewrite(),  // Query enhancement
//	    pattern.WithFusion(5),        // Retrieval enhancement
//	)
//
// The following components are automatically configured:
//   - BoltDB document store: ~/.gorag/{name}/docs.bolt
//   - GoVector vector store: ~/.gorag/{name}/vectors.db
//   - Default chunker: character-based, 512 chars with 50 overlap
//   - Default embedder: bge-small-zh-v1.5 (if not specified)
func NativeRAG(name string, opts ...NativeOption) (RAGPattern, error) {
	o := &nativeOptions{topK: 5}
	for _, opt := range opts {
		opt.applyNative(o)
	}

	// Ensure name is set first
	if name != "" {
		o.indexerOpts = append([]indexer.IndexerOption{indexer.WithName(name)}, o.indexerOpts...)
	}

	idx, err := indexer.DefaultNativeIndexer(o.indexerOpts...)
	if err != nil {
		return nil, fmt.Errorf("NativeRAG: failed to create indexer: %w", err)
	}

	// Auto-configure embedder if not set
	if idx.Embedder() == nil {
		provider, err := embedding.WithBEG("bge-small-zh-v1.5", "")
		if err != nil {
			return nil, fmt.Errorf("NativeRAG: failed to create default embedder: %w (tip: use WithBGE or WithBERT)", err)
		}
		o.indexerOpts = append(o.indexerOpts, indexer.WithEmbedding(provider))
		idx, err = indexer.DefaultNativeIndexer(o.indexerOpts...)
		if err != nil {
			return nil, fmt.Errorf("NativeRAG: failed to create indexer: %w", err)
		}
	}

	// Build retriever with enhancement options - Pipeline assembly is handled internally
	ret := nativeretriever.NewRetriever(
		idx.VectorStore(),
		idx.Embedder(),
		o.llm,
		o.topK,
		buildRetrieverOptions(o, idx.DocStore())...,
	)

	// Create repository with index synchronization
	repo := repository.NewRepository(
		idx.DocStore(),
		idx.VectorStore(),
		idx.Embedder(),
		idx.Chunker(),
	)

	return &nativeRAG{
		basePattern: basePattern{
			idx:  idx,
			ret:  ret,
			repo: repo,
		},
	}, nil
}

// buildRetrieverOptions converts nativeOptions to retriever options
func buildRetrieverOptions(o *nativeOptions, docStore core.DocStore) []nativeretriever.Option {
	opts := []nativeretriever.Option{
		nativeretriever.WithLogger(logging.DefaultNoopLogger()),
		nativeretriever.WithTopK(o.topK),
		nativeretriever.WithDocStore(docStore),
	}

	// Pre-Retrieval enhancements
	if o.enableQueryRewrite {
		opts = append(opts, nativeretriever.WithQueryRewrite())
	}
	if o.enableStepBack {
		opts = append(opts, nativeretriever.WithStepBack())
	}
	if o.enableHyDE {
		opts = append(opts, nativeretriever.WithHyDE())
	}
	if o.enableFusion {
		opts = append(opts, nativeretriever.WithFusion(o.fusionCount))
	}

	// Post-Retrieval enhancements
	if o.enableParentDoc {
		opts = append(opts, nativeretriever.WithParentDoc(nil))
	}
	if o.enableSentenceWindow {
		opts = append(opts, nativeretriever.WithSentenceWindow(nil))
	}
	if o.enablePrune {
		opts = append(opts, nativeretriever.WithContextPrune(nil))
	}
	if o.enableRerank {
		opts = append(opts, nativeretriever.WithRerank())
	}

	// Cache
	if o.enableCache {
		opts = append(opts, nativeretriever.WithCache(nil))
	}

	return opts
}
