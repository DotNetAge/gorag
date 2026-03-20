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
	IndexFile(ctx context.Context, filePath string) (*core.IndexingContext, error)
}

type defaultIndexer struct {
	pipeline *pipeline.Pipeline[*core.IndexingContext]
}

func (idx *defaultIndexer) IndexFile(ctx context.Context, filePath string) (*core.IndexingContext, error) {
	state := core.NewIndexingContext(ctx, filePath)
	err := idx.pipeline.Execute(ctx, state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// NewVectorIndexer creates a simple text-vector pipeline.
func NewVectorIndexer(
	parsers []core.Parser,
	chunker core.SemanticChunker,
	embedder embedding.Provider,
	vectorStore core.VectorStore,
	docStore store.DocStore,
	logger logging.Logger,
	metrics core.Metrics,
) Indexer {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}

	p := pipeline.New[*core.IndexingContext]()
	
	p.AddSteps(
		stepinx.Discover(),
		stepinx.Multi(parsers...),
		stepinx.Chunk(chunker), 
		stepinx.Batch(embedder, metrics), 
		stepinx.MultiStore(vectorStore, docStore, nil, logger, metrics), 
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

	p := pipeline.New[*core.IndexingContext]()
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
