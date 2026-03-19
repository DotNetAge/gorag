package weaviate

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

// ensure interface implementation
var _ core.VectorStore = (*Store)(nil)

type Store struct {
	client     *weaviate.Client
	collection string
	dimension  int
}

type Option func(*Store)

func WithCollection(name string) Option {
	return func(s *Store) {
		s.collection = name
	}
}

func WithDimension(dim int) Option {
	return func(s *Store) {
		s.dimension = dim
	}
}

func NewStore(addr string, apiKey string, opts ...Option) (*Store, error) {
	config := weaviate.Config{
		Scheme: "http",
		Host:   addr,
	}

	if apiKey != "" {
		config.AuthConfig = auth.ApiKey{
			Value: apiKey,
		}
	}

	client, err := weaviate.NewClient(config)
	if err != nil {
		return nil, err
	}

	store := &Store{
		client:     client,
		collection: "GoRAG",
		dimension:  1536,
	}

	for _, opt := range opts {
		opt(store)
	}

	err = store.ensureCollectionExists(context.Background())
	if err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) ensureCollectionExists(ctx context.Context) error {
	exists, err := s.client.Schema().ClassExistenceChecker().WithClassName(s.collection).Do(ctx)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	class := &models.Class{
		Class:      s.collection,
		Vectorizer: "none",
		Properties: []*models.Property{
			{
				Name:     "chunk_id",
				DataType: []string{"text"},
			},
			{
				Name:     "metadata_json",
				DataType: []string{"text"},
			},
		},
	}

	return s.client.Schema().ClassCreator().WithClass(class).Do(ctx)
}

func (s *Store) Add(ctx context.Context, vector *core.Vector) error {
	return s.AddBatch(ctx, []*core.Vector{vector})
}

func (s *Store) AddBatch(ctx context.Context, vectors []*core.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	for _, v := range vectors {
		properties := map[string]interface{}{
			"chunk_id": v.ChunkID,
		}

		// Simple implementation for Weaviate metadata, we dump it as json or flat properties
		for k, val := range v.Metadata {
			properties[k] = val
		}

		// Use the given UUID or generate a new one
		weaviateID := v.ID
		if _, err := uuid.Parse(weaviateID); err != nil {
			weaviateID = uuid.New().String()
		}

		_, err := s.client.Data().Creator().
			WithClassName(s.collection).
			WithID(weaviateID).
			WithProperties(properties).
			WithVector(v.Values).
			Do(ctx)
			
		if err != nil {
			return fmt.Errorf("failed to add vector %s: %w", v.ID, err)
		}
	}

	return nil
}

func (s *Store) Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*core.Vector, []float32, error) {
	if topK <= 0 {
		topK = 5
	}

	nearVector := s.client.GraphQL().NearVectorArgBuilder().WithVector(query)

	fields := []graphql.Field{
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "id"},
			{Name: "certainty"},
		}},
		{Name: "chunk_id"},
	}

	req := s.client.GraphQL().Get().
		WithClassName(s.collection).
		WithNearVector(nearVector).
		WithLimit(topK).
		WithFields(fields...)

	if len(filter) > 0 {
		// Weaviate filter builder
		var wheres []*filters.WhereBuilder
		for k, v := range filter {
			wheres = append(wheres, filters.Where().
				WithPath([]string{k}).
				WithOperator(filters.Equal).
				WithValueString(fmt.Sprintf("%v", v)))
		}
		
		if len(wheres) == 1 {
			req = req.WithWhere(wheres[0])
		} else if len(wheres) > 1 {
			req = req.WithWhere(filters.Where().WithOperator(filters.And).WithOperands(wheres))
		}
	}

	result, err := req.Do(ctx)
	if err != nil {
		return nil, nil, err
	}

	return s.parseGraphQLResult(result)
}

func (s *Store) Delete(ctx context.Context, id string) error {
	return s.DeleteBatch(ctx, []string{id})
}

func (s *Store) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		err := s.client.Data().Deleter().
			WithClassName(s.collection).
			WithID(id).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete vector %s: %w", id, err)
		}
	}

	return nil
}

func (s *Store) Close(ctx context.Context) error {
	return nil
}

func (s *Store) parseGraphQLResult(result *models.GraphQLResponse) ([]*core.Vector, []float32, error) {
	if len(result.Errors) > 0 {
		return nil, nil, fmt.Errorf("graphql error: %v", result.Errors)
	}

	data, ok := result.Data["Get"].(map[string]interface{})
	if !ok {
		return nil, nil, nil
	}
	
	objects, ok := data[s.collection].([]interface{})
	if !ok {
		return nil, nil, nil
	}

	var outVectors []*core.Vector
	var outScores []float32

	for _, obj := range objects {
		object := obj.(map[string]interface{})

		additional := object["_additional"].(map[string]interface{})
		var certainty float32
		if c, ok := additional["certainty"].(float64); ok {
			certainty = float32(c)
		}

		chunkID := ""
		if idVal, ok := object["chunk_id"].(string); ok {
			chunkID = idVal
		}

		weaviateID := ""
		if idVal, ok := additional["id"].(string); ok {
			weaviateID = idVal
		}

		vec := core.NewVector(weaviateID, nil, chunkID, nil)
		outVectors = append(outVectors, vec)
		outScores = append(outScores, certainty)
	}

	return outVectors, outScores, nil
}

func (s *Store) Upsert(ctx context.Context, vectors []*core.Vector) error { return s.AddBatch(ctx, vectors) }
