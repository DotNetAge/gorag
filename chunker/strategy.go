package chunker

import "github.com/DotNetAge/gorag/core"

// Chunk strategy constants
const (
	// StrategyFixedSize uses fixed-size chunking
	StrategyFixedSize core.ChunkStrategy = "fixed_size"
	// StrategySentence splits by sentences
	StrategySentence core.ChunkStrategy = "sentence"
	// StrategyParagraph splits by paragraphs
	StrategyParagraph core.ChunkStrategy = "paragraph"
	// StrategyRecursive uses recursive intelligent splitting
	StrategyRecursive core.ChunkStrategy = "recursive"
	// StrategySemantic uses semantic similarity for splitting
	StrategySemantic core.ChunkStrategy = "semantic"
	// StrategyCode splits code by structure
	StrategyCode core.ChunkStrategy = "code"
	// StrategyParentDoc uses two-level chunking with parent-child relationships
	StrategyParentDoc core.ChunkStrategy = "parent_doc"
)

// Default configuration constants
const (
	DefaultChunkSize          = 800   // default chunk size in characters
	DefaultOverlap            = 100   // default overlap in characters
	MinChunkSize              = 50    // minimum chunk size
	MaxChunkSize              = 2000  // maximum chunk size
	DefaultMaxSentences       = 5     // default max sentences per chunk
	DefaultMaxParagraphs      = 3     // default max paragraphs per chunk
	DefaultSimilarityThreshold = 0.7   // default similarity threshold for semantic chunking
	DefaultParentSize         = 1500  // default parent chunk size
	DefaultChildSize           = 400   // default child chunk size
)
