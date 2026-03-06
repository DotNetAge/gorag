package weaviate

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

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
				Name:     "content",
				DataType: []string{"text"},
			},
		},
	}

	return s.client.Schema().ClassCreator().WithClass(class).Do(ctx)
}

func (s *Store) Add(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
	if len(chunks) == 0 || len(embeddings) == 0 || len(chunks) != len(embeddings) {
		return nil
	}

	for i, chunk := range chunks {
		properties := map[string]interface{}{
			"content": chunk.Content,
		}

		for k, v := range chunk.Metadata {
			properties[k] = v
		}

		_, err := s.client.Data().Creator().
			WithClassName(s.collection).
			WithID(chunk.ID).
			WithProperties(properties).
			WithVector(embeddings[i]).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to add chunk %s: %w", chunk.ID, err)
		}
	}

	return nil
}

func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
	nearVector := s.client.GraphQL().NearVectorArgBuilder().
		WithVector(query)

	fields := []graphql.Field{
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "id"},
			{Name: "certainty"},
		}},
		{Name: "content"},
	}

	result, err := s.client.GraphQL().Get().
		WithClassName(s.collection).
		WithNearVector(nearVector).
		WithLimit(opts.TopK).
		WithFields(fields...).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %v", result.Errors)
	}

	data := result.Data["Get"].(map[string]interface{})
	objects, ok := data[s.collection].([]interface{})
	if !ok {
		return []vectorstore.Result{}, nil
	}

	results := make([]vectorstore.Result, 0, len(objects))
	for _, obj := range objects {
		object := obj.(map[string]interface{})

		additional := object["_additional"].(map[string]interface{})
		id := additional["id"].(string)
		certainty := float32(additional["certainty"].(float64))

		content := ""
		if c, ok := object["content"].(string); ok {
			content = c
		}

		results = append(results, vectorstore.Result{
			Chunk: vectorstore.Chunk{
				ID:      id,
				Content: content,
			},
			Score: certainty,
		})
	}

	return results, nil
}

func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		err := s.client.Data().Deleter().
			WithClassName(s.collection).
			WithID(id).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete chunk %s: %w", id, err)
		}
	}

	return nil
}

func (s *Store) Close() error {
	return nil
}
