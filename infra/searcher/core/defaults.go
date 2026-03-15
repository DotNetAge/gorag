// Package core provides shared default factory functions used by all
// searcher sub-packages (native, rerank, hybrid, multimodal, agentic,
// graphlocal, graphglobal, multiagent).
package core

import (
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/infra/fusion"
	"github.com/DotNetAge/gorag/infra/vectorstore"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// DefaultMetrics returns a no-op Metrics implementation that discards all
// recorded data. It is used as the built-in default so that every Searcher
// is always instrumented without requiring the caller to provide a collector.
func DefaultMetrics() abstraction.Metrics {
	return abstraction.NoopMetrics{}
}

// DefaultEmbedder returns the built-in local BGE embedding model.
// It requires no API key and runs entirely on-device, making it suitable
// as a zero-config fallback for development and prototyping.
func DefaultEmbedder() embedding.Provider {
	provider, err := embedding.WithBEG("bge-small-zh-v1.5", "")
	if err != nil {
		panic("searcher/core: failed to initialize default BEG embedder: " + err.Error())
	}
	return provider
}

// DefaultVectorStore returns a local govector store persisted at
// "./data/searcher/govector". No external services are required.
func DefaultVectorStore() abstraction.VectorStore {
	store, err := vectorstore.DefaultVectorStore("./data/searcher/govector")
	if err != nil {
		panic("searcher/core: failed to initialize default vector store: " + err.Error())
	}
	return store
}

// DefaultFusionEngine returns the built-in RRF (Reciprocal Rank Fusion) engine
// with k=60 as per the original paper. It is a pure in-process computation
// with no external dependencies.
func DefaultFusionEngine() retrieval.FusionEngine {
	return fusion.NewRRFFusionEngine()
}
