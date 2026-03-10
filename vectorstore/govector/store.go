package govector

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	gvcore "github.com/DotNetAge/govector/core"
)

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

// NewStore initializes a new govector store
func NewStore(ctx context.Context, opts ...Option) (*Store, error) {
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

func (s *Store) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("number of chunks and embeddings must match")
	}

	var points []gvcore.PointStruct
	for i, chunk := range chunks {
		payload := gvcore.Payload{
			"content": chunk.Content,
		}
		for k, v := range chunk.Metadata {
			payload[k] = v
		}

		points = append(points, gvcore.PointStruct{
			ID:      chunk.ID,
			Vector:  embeddings[i],
			Payload: payload,
		})
	}

	if len(points) == 0 {
		return nil
	}

	return s.collection.Upsert(points)
}

func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]core.Result, error) {
	var filter *gvcore.Filter

	if opts.Filter != nil {
		filter = &gvcore.Filter{}
		for k, v := range opts.Filter {
			filter.Must = append(filter.Must, gvcore.Condition{
				Key:   k,
				Match: gvcore.MatchValue{Value: v},
			})
		}
	}

	if opts.Metadata != nil {
		if filter == nil {
			filter = &gvcore.Filter{}
		}
		for k, v := range opts.Metadata {
			filter.Must = append(filter.Must, gvcore.Condition{
				Key:   k,
				Match: gvcore.MatchValue{Value: v},
			})
		}
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	scoredPoints, err := s.collection.Search(query, filter, topK)
	if err != nil {
		return nil, err
	}

	var results []core.Result
	for _, pt := range scoredPoints {
		if pt.Score < opts.MinScore {
			continue // skip those below the minimum score
		}

		content := ""
		if c, ok := pt.Payload["content"].(string); ok {
			content = c
		}

		metadata := make(map[string]string)
		for k, v := range pt.Payload {
			if k != "content" {
				if str, ok := v.(string); ok {
					metadata[k] = str
				}
			}
		}

		results = append(results, core.Result{
			Chunk: core.Chunk{
				ID:       pt.ID,
				Content:  content,
				Metadata: metadata,
			},
			Score: pt.Score,
		})
	}

	return results, nil
}

func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.collection.Delete(ids, nil)
	return err
}

func (s *Store) SearchByMetadata(ctx context.Context, metadata map[string]string) ([]core.Chunk, error) {
	// govector search requires a query vector. We use a zero vector.
	zeroVec := make([]float32, s.dimension)

	var filter *gvcore.Filter
	if len(metadata) > 0 {
		filter = &gvcore.Filter{}
		for k, v := range metadata {
			filter.Must = append(filter.Must, gvcore.Condition{
				Key:   k,
				Match: gvcore.MatchValue{Value: v},
			})
		}
	}

	topK := s.collection.Count()
	if topK == 0 {
		return []core.Chunk{}, nil
	}

	scoredPoints, err := s.collection.Search(zeroVec, filter, topK)
	if err != nil {
		return nil, err
	}

	var results []core.Chunk
	for _, pt := range scoredPoints {
		content := ""
		if c, ok := pt.Payload["content"].(string); ok {
			content = c
		}

		meta := make(map[string]string)
		for k, v := range pt.Payload {
			if k != "content" {
				if str, ok := v.(string); ok {
					meta[k] = str
				}
			}
		}

		results = append(results, core.Chunk{
			ID:       pt.ID,
			Content:  content,
			Metadata: meta,
		})
	}

	return results, nil
}

func (s *Store) Close() error {
	if s.storage != nil {
		return s.storage.Close()
	}
	return nil
}
