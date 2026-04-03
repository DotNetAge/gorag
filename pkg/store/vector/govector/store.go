package govector

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/core"
	gvcore "github.com/DotNetAge/govector/core"
)

// ensure interface implementation

type Store struct {
	storage    *gvcore.Storage
	collection *gvcore.Collection
	colName    string
	dimension  int
	dbPath     string
	useHNSW    bool
}

type Option func(*Store)

// WithCollection sets the collection name
func WithCollection(name string) Option {
	return func(s *Store) {
		s.colName = name
	}
}

// WithDimension sets the vector dimension
func WithDimension(dim int) Option {
	return func(s *Store) {
		s.dimension = dim
	}
}

// WithDBPath sets the path for the local bolt database
func WithDBPath(path string) Option {
	return func(s *Store) {
		s.dbPath = path
	}
}

// WithHNSW enables or disables HNSW indexing
func WithHNSW(use bool) Option {
	return func(s *Store) {
		s.useHNSW = use
	}
}

// DefaultStore returns a govector store configured for local testing.
// It creates a "gorag_vectors.db" file in the current directory and uses a dimension of 1536 (OpenAI default).
func DefaultStore() (core.VectorStore, error) {
	return NewStore(
		WithDBPath("gorag_vectors.db"),
		WithDimension(1536),
		WithCollection("gorag"),
		WithHNSW(true),
	)
}

// NewStore initializes a new govector store
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

		vec := core.NewVector(pt.ID, nil, chunkID, metadata)
		outVectors = append(outVectors, vec)
		outScores = append(outScores, pt.Score)
	}

	return outVectors, outScores, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	_, err := s.collection.Delete([]string{id}, nil)
	return err
}

func (s *Store) Close(ctx context.Context) error {
	if s.storage != nil {
		return s.storage.Close()
	}
	return nil
}
