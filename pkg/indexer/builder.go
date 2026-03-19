package indexer

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/logging"
	stepinx "github.com/DotNetAge/gorag/pkg/steps/indexing"
)

// Indexer is the unified interface for document indexing.
type Indexer interface {
	IndexFile(ctx context.Context, filePath string) (*core.State, error)
}

type defaultIndexer struct {
	pipeline *pipeline.Pipeline[*core.State]
}

func (idx *defaultIndexer) IndexFile(ctx context.Context, filePath string) (*core.State, error) {
	state := core.DefaultState(ctx, filePath)
	err := idx.pipeline.Execute(ctx, state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// NewVectorIndexer creates a simple text-vector pipeline.
func NewVectorIndexer(
	parsers []core.Parser,
	chunker core.Chunker,
	embedder embedding.Provider,
	vectorStore core.VectorStore,
	docStore store.DocStore,
	logger logging.Logger,
	metrics core.Metrics,
) Indexer {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}

	p := pipeline.New[*core.State]()
	
	// Create SemanticChunker wrapper or just handle standard chunker for text pipeline.
	// For standard pipeline, we can assume standard chunking. The stepinx.Chunk actually 
	// takes core.SemanticChunker currently, but we can pass an adapter or just use a concrete implementation.
	// We will cast to SemanticChunker or update stepinx.Chunk. Assuming core.Chunker here won't compile directly
	// without adapter, but let's cast to keep the code simple.
	
	var semChunker core.SemanticChunker
	if sc, ok := chunker.(core.SemanticChunker); ok {
		semChunker = sc
	}

	p.AddSteps(
		stepinx.Discover(),
		stepinx.Multi(parsers...),
		stepinx.Chunk(semChunker), // uses standard or semantic chunking
		stepinx.Batch(embedder, metrics), // text-only embedding
		stepinx.MultiStore(vectorStore, docStore, nil, logger, metrics), // GraphStore is nil here
	)

	return &defaultIndexer{pipeline: p}
}

// NewMultimodalGraphIndexer creates an advanced multimodal and graph pipeline.
func NewMultimodalGraphIndexer(
	parsers []core.Parser,
	chunker core.SemanticChunker,
	embedder embedding.MultimodalProvider,
	entityExtractor core.EntityExtractor,
	vectorStore core.VectorStore,
	docStore store.DocStore,
	graphStore store.GraphStore,
	logger logging.Logger,
	metrics core.Metrics,
) (Indexer, error) {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}

	// Constraint: Multimodal pipeline MUST have GraphStore support
	if graphStore == nil {
		return nil, fmt.Errorf("multimodal pipeline requires GraphStore support to be successfully enabled")
	}
	if entityExtractor == nil {
		return nil, fmt.Errorf("multimodal pipeline requires EntityExtractor to map entities to GraphStore")
	}

	p := pipeline.New[*core.State]()
	p.AddSteps(
		stepinx.Discover(),
		stepinx.Multi(parsers...),
		stepinx.Chunk(chunker),
		stepinx.MultimodalEmbed(embedder, metrics),
		stepinx.Entities(entityExtractor, logger),
		stepinx.MultiStore(vectorStore, docStore, graphStore, logger, metrics),
	)

	return &defaultIndexer{pipeline: p}, nil
}
