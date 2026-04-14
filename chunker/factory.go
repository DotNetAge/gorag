package chunker

import (
	"fmt"
	"sync"

	"github.com/DotNetAge/gorag/core"
)

// ChunkerCreator is a function that creates a chunker instance
type ChunkerCreator func(opts ...Option) core.Chunker

// ChunkingFactory creates chunker instances based on strategy type
type ChunkingFactory struct {
	mu       sync.RWMutex
	creators map[core.ChunkStrategy]ChunkerCreator
}

// NewChunkingFactory creates a new ChunkingFactory with default chunkers registered
func NewChunkingFactory() *ChunkingFactory {
	factory := &ChunkingFactory{
		creators: make(map[core.ChunkStrategy]ChunkerCreator),
	}

	// Register default chunkers
	factory.RegisterChunker(StrategyFixedSize, func(opts ...Option) core.Chunker {
		return NewFixedSizeChunker(opts...)
	})

	factory.RegisterChunker(StrategyRecursive, func(opts ...Option) core.Chunker {
		return NewRecursiveChunker(opts...)
	})

	factory.RegisterChunker(StrategySentence, func(opts ...Option) core.Chunker {
		return NewSentenceChunker(opts...)
	})

	factory.RegisterChunker(StrategyParagraph, func(opts ...Option) core.Chunker {
		return NewParagraphChunker(opts...)
	})

	factory.RegisterChunker(StrategyCode, func(opts ...Option) core.Chunker {
		return NewCodeChunker(opts...)
	})

	factory.RegisterChunker(StrategyParentDoc, func(opts ...Option) core.Chunker {
		return NewParentDocChunker(opts...)
	})

	// SemanticChunker requires an embedder, registered with nil by default
	// Users should manually register a version with an embedder
	factory.RegisterChunker(StrategySemantic, func(opts ...Option) core.Chunker {
		return NewSemanticChunker(nil, opts...)
	})

	return factory
}

// CreateChunker creates a chunker based on strategy
func (f *ChunkingFactory) CreateChunker(strategy core.ChunkStrategy, opts ...Option) (core.Chunker, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	creator, exists := f.creators[strategy]
	if !exists {
		return nil, fmt.Errorf("unsupported chunk strategy: %s", strategy)
	}

	return creator(opts...), nil
}

// RegisterChunker registers a chunker creator for a strategy
func (f *ChunkingFactory) RegisterChunker(strategy core.ChunkStrategy, creator ChunkerCreator) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.creators[strategy] = creator
}

// UnregisterChunker removes a chunker registration
func (f *ChunkingFactory) UnregisterChunker(strategy core.ChunkStrategy) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.creators, strategy)
}

// GetSupportedStrategies returns all registered strategies
func (f *ChunkingFactory) GetSupportedStrategies() []core.ChunkStrategy {
	f.mu.RLock()
	defer f.mu.RUnlock()

	strategies := make([]core.ChunkStrategy, 0, len(f.creators))
	for strategy := range f.creators {
		strategies = append(strategies, strategy)
	}

	return strategies
}

// IsStrategySupported checks if a strategy is registered
func (f *ChunkingFactory) IsStrategySupported(strategy core.ChunkStrategy) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, exists := f.creators[strategy]
	return exists
}

// MustCreateChunker creates a chunker, panics on error
func (f *ChunkingFactory) MustCreateChunker(strategy core.ChunkStrategy, opts ...Option) core.Chunker {
	chunker, err := f.CreateChunker(strategy, opts...)
	if err != nil {
		panic(err)
	}
	return chunker
}

// Global factory instance
var globalFactory = NewChunkingFactory()

// CreateChunker creates a chunker using the global factory
func CreateChunker(strategy core.ChunkStrategy, opts ...Option) (core.Chunker, error) {
	return globalFactory.CreateChunker(strategy, opts...)
}

// RegisterChunker registers a chunker with the global factory
func RegisterChunker(strategy core.ChunkStrategy, creator ChunkerCreator) {
	globalFactory.RegisterChunker(strategy, creator)
}

// GetSupportedStrategies returns supported strategies from global factory
func GetSupportedStrategies() []core.ChunkStrategy {
	return globalFactory.GetSupportedStrategies()
}