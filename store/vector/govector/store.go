package govector

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/DotNetAge/gorag/v2/core"
	gvcore "github.com/DotNetAge/govector/core"
)

// Store is an implementation of core.VectorStore using govector.
type Store struct {
	sync.RWMutex
	storage    *gvcore.Storage
	collection *gvcore.Collection
	colName    string
	dimension  int
	dbPath     string
	useHNSW    bool
	readOnly   bool
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
func WithHNSW(use bool) Option {
	return func(s *Store) {
		s.useHNSW = use
	}
}

// WithReadOnly opens the underlying BoltDB in read-only mode (shared lock).
// This allows coexistence with another process (e.g. Daemon) that holds
// the write lock on the same .db file.
func WithReadOnly(readOnly bool) Option {
	return func(s *Store) {
		s.readOnly = readOnly
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

	storage, err := gvcore.NewStorageWithQuantization(store.dbPath, false, gvcore.Quantizer(nil), store.readOnly)
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
	s.Lock()
	defer s.Unlock()

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
	s.RLock()
	defer s.RUnlock()

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
	s.Lock()
	defer s.Unlock()

	if id == "" {
		return nil
	}

	// 尝试按 vector UUID 删除
	deleted, err := s.collection.Delete([]string{id}, nil)
	if err != nil {
		return err
	}
	if deleted > 0 {
		return nil
	}

	// UUID 未命中，回退到按 chunk_id 元数据过滤删除
	filter := &gvcore.Filter{
		Must: []gvcore.Condition{{
			Key:   "chunk_id",
			Match: gvcore.MatchValue{Value: id},
		}},
	}
	_, err = s.collection.Delete(nil, filter)
	return err
}

// Count returns the total number of vectors in the store.
func (s *Store) Count(ctx context.Context) (int, error) {
	s.RLock()
	defer s.RUnlock()
	return s.collection.Count(), nil
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

// GetByDocID retrieves all vectors belonging to the same document by doc_id.
// Results are sorted by chunk_meta.index to enable document reconstruction.
//
// Parameters:
//   - ctx: Context for cancellation
//   - docID: The document ID to search for
//
// Returns:
//   - []*core.Vector: All vectors belonging to the document, sorted by chunk index
//   - error: Any error that occurred
func (s *Store) GetByDocID(ctx context.Context, docID string) ([]*core.Vector, error) {
	s.RLock()
	defer s.RUnlock()

	if docID == "" {
		return nil, fmt.Errorf("docID cannot be empty")
	}

	filter := &gvcore.Filter{
		Must: []gvcore.Condition{{
			Key:   "doc_id",
			Match: gvcore.MatchValue{Value: docID},
		}},
	}

	points, err := s.collection.GetPointsByFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get points by doc_id: %w", err)
	}

	vectors := make([]*core.Vector, 0, len(points))
	for _, pt := range points {
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

		vectors = append(vectors, &core.Vector{
			ID:       pt.ID,
			Values:   pt.Vector,
			ChunkID:  chunkID,
			Metadata: metadata,
		})
	}

	// Sort by chunk_meta.index for document reconstruction
	sort.Slice(vectors, func(i, j int) bool {
		return extractChunkIndex(vectors[i]) < extractChunkIndex(vectors[j])
	})

	return vectors, nil
}

// extractChunkIndex extracts the chunk index from a Vector's Metadata["chunk_meta"].map["index"].
func extractChunkIndex(v *core.Vector) int {
	if v == nil || v.Metadata == nil {
		return 0
	}
	cm, ok := v.Metadata["chunk_meta"].(map[string]any)
	if !ok {
		return 0
	}
	index, ok := cm["index"].(float64)
	if !ok {
		return 0
	}
	return int(index)
}

// List returns paginated vectors from the store.
// Uses an empty filter to retrieve all points, then applies offset/limit.
//
// Parameters:
//   - ctx: Context for cancellation
//   - offset: Number of vectors to skip (0-based)
//   - limit: Maximum number of vectors to return
//
// Returns:
//   - []*core.Vector: The paginated vectors
//   - error: Any error that occurred during retrieval
func (s *Store) List(ctx context.Context, offset, limit int) ([]*core.Vector, error) {
	vectors, _, err := s.ListFiltered(ctx, offset, limit, nil)
	return vectors, err
}

// ListFiltered returns paginated vectors filtered by metadata conditions.
// Each FilterCondition is ANDed together (all must match).
// Returns the filtered vectors, total count before pagination, and any error.
func (s *Store) ListFiltered(ctx context.Context, offset, limit int, filters []core.FilterCondition) ([]*core.Vector, int, error) {
	s.RLock()
	defer s.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// Build govector filter from generic filter conditions
	gvFilter := &gvcore.Filter{}
	for _, fc := range filters {
		cond := gvcore.Condition{
			Key:  fc.Key,
			Type: gvcore.ConditionType(fc.Type),
		}
		// Normalize filter value: JSON numbers come as float64, but protobuf
		// stores int metadata as int64. Direct comparison fails in Go.
		val := fc.Value
		if f64, ok := val.(float64); ok && f64 == float64(int64(f64)) {
			val = int64(f64)
		}
		switch fc.Type {
		case "exact":
			cond.Match = gvcore.MatchValue{Value: val}
		case "prefix":
			if s, ok := val.(string); ok {
				cond.Match = gvcore.MatchValue{Value: s}
			}
		}
		gvFilter.Must = append(gvFilter.Must, cond)
	}

	points, err := s.collection.GetPointsByFilter(gvFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list filtered vectors: %w", err)
	}

	total := len(points)

	// Apply pagination
	end := offset + limit
	if end > len(points) {
		end = len(points)
	}
	if offset >= len(points) {
		return []*core.Vector{}, total, nil
	}

	vectors := make([]*core.Vector, 0, end-offset)
	for _, pt := range points[offset:end] {
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

		vectors = append(vectors, &core.Vector{
			ID:       pt.ID,
			Values:   pt.Vector,
			ChunkID:  chunkID,
			Metadata: metadata,
		})
	}

	return vectors, total, nil
}
