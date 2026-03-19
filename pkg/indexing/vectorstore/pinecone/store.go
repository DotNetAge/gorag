package pinecone

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

// ensure interface implementation
var _ core.VectorStore = (*Store)(nil)

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

func (s *Store) Add(ctx context.Context, vector *core.Vector) error {
	return s.AddBatch(ctx, []*core.Vector{vector})
}

func (s *Store) AddBatch(ctx context.Context, vectors []*core.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	pcVectors := make([]*pinecone.Vector, len(vectors))
	for i, v := range vectors {
		metadata, err := s.buildMetadata(v)
		if err != nil {
			return fmt.Errorf("failed to build metadata for vector %s: %w", v.ID, err)
		}

		pcVectors[i] = &pinecone.Vector{
			Id:       v.ID,
			Values:   v.Values,
			Metadata: metadata,
		}
	}

	_, err := s.index.UpsertVectors(ctx, pcVectors)
	return err
}

func (s *Store) Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*core.Vector, []float32, error) {
	if topK <= 0 {
		topK = 5
	}

	var pbFilter *structpb.Struct
	var err error
	if len(filter) > 0 {
		pbFilter, err = structpb.NewStruct(filter)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid filter: %w", err)
		}
	}

	resp, err := s.index.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          query,
		TopK:            uint32(topK),
		IncludeMetadata: true,
		IncludeValues:   false,
		MetadataFilter:  pbFilter,
	})
	if err != nil {
		return nil, nil, err
	}

	return s.parseMatches(resp.Matches)
}

func (s *Store) Delete(ctx context.Context, id string) error {
	return s.DeleteBatch(ctx, []string{id})
}

func (s *Store) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.index.DeleteVectorsById(ctx, ids)
}

func (s *Store) Close(ctx context.Context) error {
	if s.index != nil {
		s.index.Close()
	}
	return nil
}

func (s *Store) buildMetadata(v *core.Vector) (*pinecone.Metadata, error) {
	fields := map[string]interface{}{
		"chunk_id": v.ChunkID,
	}
	for k, val := range v.Metadata {
		fields[k] = val
	}

	return structpb.NewStruct(fields)
}

func (s *Store) parseMatches(matches []*pinecone.ScoredVector) ([]*core.Vector, []float32, error) {
	var outVectors []*core.Vector
	var outScores []float32

	for _, match := range matches {
		metadata := make(map[string]any)
		chunkID := ""

		if match.Vector != nil && match.Vector.Metadata != nil {
			fields := match.Vector.Metadata.AsMap()
			if c, ok := fields["chunk_id"].(string); ok {
				chunkID = c
			}
			for k, v := range fields {
				if k != "chunk_id" {
					metadata[k] = v
				}
			}
		}

		id := ""
		if match.Vector != nil {
			id = match.Vector.Id
		}

		vec := core.NewVector(id, nil, chunkID, metadata)
		outVectors = append(outVectors, vec)
		outScores = append(outScores, match.Score)
	}
	return outVectors, outScores, nil
}

func (s *Store) Upsert(ctx context.Context, vectors []*core.Vector) error { return s.AddBatch(ctx, vectors) }
