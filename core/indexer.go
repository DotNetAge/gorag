package core

import "context"

// Indexer defines the interface for indexers in the RAG system.
// Indexers are responsible for adding content to an index, searching the index, and removing content from the index.
type Indexer interface {
	// Name returns the name of the indexer.
	//
	// Returns:
	//   - string: The name of the indexer
	Name() string

	// Type returns the type of the indexer.
	//
	// Returns:
	//   - string: The type of the indexer
	Type() string

	// Add adds content to the index.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - content: The content to add to the index
	//
	// Returns:
	//   - *Chunk: The chunk created from the content
	//   - error: An error if the operation fails
	Add(ctx context.Context, content string) (*Chunk, error)

	AddFile(ctx context.Context, filePath string) (*Chunk, error)

	NewQuery(terms string) Query

	// Search searches the index for the given query.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - query: The query to search for
	//
	// Returns:
	//   - []Hit: The search results
	//   - error: An error if the operation fails
	Search(ctx context.Context, query Query) ([]Hit, error)

	// Remove removes a chunk from the index.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - chunkID: The ID of the chunk to remove
	//
	// Returns:
	//   - error: An error if the operation fails
	Remove(ctx context.Context, chunkID string) error
}
