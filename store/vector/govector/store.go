package govector

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/core"
	gvcore "github.com/DotNetAge/govector/core"
)

// Store is an implementation of core.VectorStore using govector.
type Store struct {
	// storage is the underlying govector storage
	storage *gvcore.Storage
	// collection is the govector collection
	collection *gvcore.Collection
	// colName is the collection name
	colName string
	// dimension is the vector dimension
	dimension int
	// dbPath is the path to the database file
	dbPath string
	// useHNSW indicates whether to use HNSW indexing
	useHNSW bool
}

// Option is a function that configures a Store.
type Option func(*Store)

// WithCollection sets the collection name.
//
// Parameters:
//   - name: The collection name
//
// Returns:
//   - Option: A configuration function
func WithCollection(name string) Option {
	return func(s *Store) {
		s.colName = name
	}
}

// WithDimension sets the vector dimension.
//
// Parameters:
//   - dim: The vector dimension
//
// Returns:
//   - Option: A configuration function
func WithDimension(dim int) Option {
	return func(s *Store) {
		s.dimension = dim
	}
}

// WithDBPath sets the path for the local bolt database.
//
// Parameters:
//   - path: The database path
//
// Returns:
//   - Option: A configuration function
func WithDBPath(path string) Option {
	return func(s *Store) {
		s.dbPath = path
	}
}

// WithHNSW enables or disables HNSW indexing.
//
// Parameters:
//   - use: Whether to use HNSW indexing
//
// Returns:
//   - Option: A configuration function
func WithHNSW(use bool) Option {
	return func(s *Store) {
		s.useHNSW = use
	}
}

// DefaultStore returns a govector store configured for local testing.
// It creates a "gorag_vectors.db" file in the current directory and uses a dimension of 1536 (OpenAI default).
//
// Returns:
//   - core.VectorStore: The vector store
//   - error: Any error that occurred
func DefaultStore() (core.VectorStore, error) {
	return NewStore(
		WithDBPath("gorag_vectors.db"),
		WithDimension(1536),
		WithCollection("gorag"),
		WithHNSW(true),
	)
}

// NewStore initializes a new govector store.
//
// Parameters:
//   - opts: Configuration options
//
// Returns:
//   - core.VectorStore: The vector store
//   - error: Any error that occurred
func NewStore(opts ...Option) (core.VectorStore, error) {
	store := &Store{
		colName:   "gorag",
		dimension: 1536,
		dbPath:    "gorag_vectors.db",
		useHNSW:   true,
	}

	for _, opt := range opts {
		opt(store)
	}

	storage, err := gvcore.NewStorage(store.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open govector storage: %w", err)
	}
	store.storage = storage

	col, err := gvcore.NewCollection(store.colName, store.dimension, gvcore.Cosine, storage, store.useHNSW)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize collection: %w", err)
	}
	store.collection = col

	return store, nil
}

// Upsert inserts or updates vectors in the store.
//
// Parameters:
//   - ctx: Context for cancellation
//   - vectors: The vectors to upsert
//
// Returns:
//   - error: Any error that occurred
func (s *Store) Upsert(ctx context.Context, vectors []*core.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	var points []gvcore.PointStruct
	for _, v := range vectors {
		payload := make(gvcore.Payload)
		for key, val := range v.Metadata {
			payload[key] = val
		}
		// Inject the chunk_id into payload to map it back later
		payload["chunk_id"] = v.ChunkID

		points = append(points, gvcore.PointStruct{
			ID:      v.ID,
			Vector:  v.Values,
			Payload: payload,
		})
	}

	if len(points) == 0 {
		return nil
	}

	return s.collection.Upsert(points)
}

// Search searches for vectors similar to the query vector.
//
// Parameters:
//   - ctx: Context for cancellation
//   - query: The query vector
//   - topK: The maximum number of results
//   - filters: Metadata filters
//
// Returns:
//   - []*core.Vector: The similar vectors
//   - []float32: The similarity scores
//   - error: Any error that occurred
func (s *Store) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	var gvFilter *gvcore.Filter

	if len(filters) > 0 {
		gvFilter = &gvcore.Filter{}
		for k, v := range filters {
			gvFilter.Must = append(gvFilter.Must, gvcore.Condition{
				Key:   k,
				Match: gvcore.MatchValue{Value: v},
			})
		}
	}

	if topK <= 0 {
		topK = 5
	}

	scoredPoints, err := s.collection.Search(query, gvFilter, topK)
	if err != nil {
		return nil, nil, err
	}

	var outVectors []*core.Vector
	var outScores []float32

	for _, pt := range scoredPoints {
		chunkID := ""
		if c, ok := pt.Payload["chunk_id"].(string); ok {
			chunkID = c
		}

		metadata := make(map[string]any)
		for k, v := range pt.Payload {
			if k != "chunk_id" {
				metadata[k] = v
			}
		}

		vec := &core.Vector{
			ID:       pt.ID,
			Values:   nil,
			ChunkID:  chunkID,
			Metadata: metadata,
		}
		outVectors = append(outVectors, vec)
		outScores = append(outScores, pt.Score)
	}

	return outVectors, outScores, nil
}

// Delete deletes a vector by ID or chunk_id.
//
// Parameters:
//   - ctx: Context for cancellation
//   - id: The vector ID (UUID format), or chunk_id (chunk_{docID}_{index}_{hash} format)
//
// Returns:
//   - error: Any error that occurred
func (s *Store) Delete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}

	// If id starts with "chunk_", it's a chunk_id - use filter to delete
	if strings.HasPrefix(id, "chunk_") {
		filter := &gvcore.Filter{
			Must: []gvcore.Condition{{
				Key:   "chunk_id",
				Match: gvcore.MatchValue{Value: id},
			}},
		}
		_, err := s.collection.Delete(nil, filter)
		return err
	}

	// Otherwise treat as vector UUID
	_, err := s.collection.Delete([]string{id}, nil)
	return err
}

// Close closes the vector store.
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns:
//   - error: Any error that occurred
func (s *Store) Close(ctx context.Context) error {
	if s.storage != nil {
		return s.storage.Close()
	}
	return nil
}
