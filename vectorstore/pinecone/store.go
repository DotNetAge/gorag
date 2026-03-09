package pinecone

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

type Store struct {
	client      *pinecone.Client
	index       *pinecone.IndexConnection
	indexName   string
	environment string
	dimension   int
	namespace   string
}

type Option func(*Store)

func WithIndex(name string) Option {
	return func(s *Store) {
		s.indexName = name
	}
}

func WithEnvironment(env string) Option {
	return func(s *Store) {
		s.environment = env
	}
}

func WithDimension(dim int) Option {
	return func(s *Store) {
		s.dimension = dim
	}
}

func WithNamespace(ns string) Option {
	return func(s *Store) {
		s.namespace = ns
	}
}

func NewStore(apiKey string, opts ...Option) (*Store, error) {
	client, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: apiKey,
	})
	if err != nil {
		return nil, err
	}

	store := &Store{
		client:      client,
		indexName:   "gorag",
		environment: "gcp-starter",
		dimension:   1536,
		namespace:   "",
	}

	for _, opt := range opts {
		opt(store)
	}

	ctx := context.Background()
	idx, err := client.DescribeIndex(ctx, store.indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe index: %w", err)
	}

	indexConn, err := client.Index(pinecone.NewIndexConnParams{
		Host: idx.Host,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to index: %w", err)
	}

	store.index = indexConn

	return store, nil
}

func (s *Store) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	if len(chunks) == 0 || len(embeddings) == 0 || len(chunks) != len(embeddings) {
		return nil
	}

	vectors := make([]*pinecone.Vector, len(chunks))
	for i, chunk := range chunks {
		metadata, err := s.buildMetadata(chunk)
		if err != nil {
			return fmt.Errorf("failed to build metadata for chunk %s: %w", chunk.ID, err)
		}

		vectors[i] = &pinecone.Vector{
			Id:       chunk.ID,
			Values:   embeddings[i],
			Metadata: metadata,
		}
	}

	_, err := s.index.UpsertVectors(ctx, vectors)
	return err
}

func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]core.Result, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	resp, err := s.index.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          query,
		TopK:            uint32(topK),
		IncludeMetadata: true,
		IncludeValues:   false,
	})
	if err != nil {
		return nil, err
	}

	return s.parseMatches(resp.Matches), nil
}

func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	return s.index.DeleteVectorsById(ctx, ids)
}

func (s *Store) Close() error {
	return nil
}

// SearchByMetadata searches for chunks with matching metadata
func (s *Store) SearchByMetadata(ctx context.Context, metadata map[string]string) ([]core.Chunk, error) {
	if len(metadata) == 0 {
		return []core.Chunk{}, nil
	}

	// Create a dummy vector (Pinecone requires a vector for query)
	dummyVector := make([]float32, s.dimension)

	// Query without filter (we'll filter client-side)
	resp, err := s.index.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          dummyVector,
		TopK:            1000, // Use large limit to get all matching chunks
		IncludeMetadata: true,
		IncludeValues:   false,
	})
	if err != nil {
		return nil, err
	}

	// Parse and filter results client-side
	var chunks []core.Chunk
	for _, vectorMatch := range resp.Matches {
		content := ""
		chunkMetadata := make(map[string]string)

		if vectorMatch.Vector != nil && vectorMatch.Vector.Metadata != nil {
			fields := vectorMatch.Vector.Metadata.AsMap()
			if c, ok := fields["content"].(string); ok {
				content = c
			}
			for k, v := range fields {
				if k != "content" {
					if strVal, ok := v.(string); ok {
						chunkMetadata[k] = strVal
					}
				}
			}
		}

		// Check if all metadata matches
		isMatch := true
		for key, value := range metadata {
			if chunkValue, exists := chunkMetadata[key]; !exists || chunkValue != value {
				isMatch = false
				break
			}
		}

		if isMatch {
			id := ""
			if vectorMatch.Vector != nil {
				id = vectorMatch.Vector.Id
			}

			chunks = append(chunks, core.Chunk{
				ID:       id,
				Content:  content,
				Metadata: chunkMetadata,
			})
		}
	}

	return chunks, nil
}

func (s *Store) buildMetadata(chunk core.Chunk) (*pinecone.Metadata, error) {
	fields := map[string]interface{}{
		"content": chunk.Content,
	}
	for k, v := range chunk.Metadata {
		fields[k] = v
	}

	return structpb.NewStruct(fields)
}

func (s *Store) parseMatches(matches []*pinecone.ScoredVector) []core.Result {
	results := make([]core.Result, 0, len(matches))
	for _, match := range matches {
		content := ""
		metadata := make(map[string]string)

		if match.Vector != nil && match.Vector.Metadata != nil {
			fields := match.Vector.Metadata.AsMap()
			if c, ok := fields["content"].(string); ok {
				content = c
			}
			for k, v := range fields {
				if k != "content" {
					if strVal, ok := v.(string); ok {
						metadata[k] = strVal
					}
				}
			}
		}

		id := ""
		if match.Vector != nil {
			id = match.Vector.Id
		}

		results = append(results, core.Result{
			Chunk: core.Chunk{
				ID:       id,
				Content:  content,
				Metadata: metadata,
			},
			Score: match.Score,
		})
	}
	return results
}
