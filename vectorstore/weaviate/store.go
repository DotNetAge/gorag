package weaviate

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
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
			{
				Name:     "chunk_id",
				DataType: []string{"text"},
			},
		},
	}

	return s.client.Schema().ClassCreator().WithClass(class).Do(ctx)
}

func (s *Store) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	if len(chunks) == 0 || len(embeddings) == 0 || len(chunks) != len(embeddings) {
		return nil
	}

	for i, chunk := range chunks {
		properties := map[string]interface{}{
			"content":  chunk.Content,
			"chunk_id": chunk.ID,
		}

		for k, v := range chunk.Metadata {
			properties[k] = v
		}

		// Generate UUID for Weaviate
		weaviateID := uuid.New().String()

		_, err := s.client.Data().Creator().
			WithClassName(s.collection).
			WithID(weaviateID).
			WithProperties(properties).
			WithVector(embeddings[i]).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to add chunk %s: %w", chunk.ID, err)
		}
	}

	return nil
}

func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]core.Result, error) {
	nearVector := s.client.GraphQL().NearVectorArgBuilder().
		WithVector(query)

	fields := []graphql.Field{
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "id"},
			{Name: "certainty"},
		}},
		{Name: "content"},
		{Name: "chunk_id"},
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

	return s.parseGraphQLResult(result)
}

func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// For each ID, we need to first query the object by its id property
	// and then delete it by its Weaviate-generated UUID
	for _, id := range ids {
		// Build where filter to find objects by chunk_id property
		where := filters.Where().
			WithPath([]string{"chunk_id"}).
			WithOperator(filters.Equal).
			WithValueString(id)

		// Query objects matching the filter
		fields := []graphql.Field{
			{Name: "_additional", Fields: []graphql.Field{
				{Name: "id"},
			}},
		}

		result, err := s.client.GraphQL().Get().
			WithClassName(s.collection).
			WithWhere(where).
			WithFields(fields...).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to find chunk %s: %w", id, err)
		}

		// Parse the result to get the Weaviate-generated UUID
		if len(result.Errors) > 0 {
			return fmt.Errorf("graphql error: %v", result.Errors)
		}

		data := result.Data["Get"].(map[string]interface{})
		objects, ok := data[s.collection].([]interface{})
		if !ok || len(objects) == 0 {
			continue // No object found with this id, skip
		}

		// Delete each object found
		for _, obj := range objects {
			object := obj.(map[string]interface{})
			additional := object["_additional"].(map[string]interface{})
			weaviateID := additional["id"].(string)

			err := s.client.Data().Deleter().
				WithClassName(s.collection).
				WithID(weaviateID).
				Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to delete chunk %s: %w", id, err)
			}
		}
	}

	return nil
}

func (s *Store) Close() error {
	return nil
}

// SearchByMetadata searches for chunks with matching metadata
func (s *Store) SearchByMetadata(ctx context.Context, metadata map[string]string) ([]core.Chunk, error) {
	if len(metadata) == 0 {
		return []core.Chunk{}, nil
	}

	// Build where filter with multiple conditions
	where := filters.Where()
	first := true
	for key, value := range metadata {
		filter := filters.Where().
			WithPath([]string{key}).
			WithOperator(filters.Equal).
			WithValueString(value)

		if first {
			where = filter
			first = false
		} else {
			// For multiple conditions, we need to use a different approach
			// Weaviate Go client doesn't have And method, so we need to build a complex filter
			where = filters.Where().
				WithOperator(filters.And).
				WithOperands([]*filters.WhereBuilder{where, filter})
		}
	}

	// Define fields to retrieve
	fields := []graphql.Field{
		{Name: "content"},
		{Name: "chunk_id"},
	}

	// Add metadata fields to retrieval
	for key := range metadata {
		fields = append(fields, graphql.Field{Name: key})
	}

	// Execute GraphQL query
	result, err := s.client.GraphQL().Get().
		WithClassName(s.collection).
		WithWhere(where).
		WithLimit(1000). // Use large limit to get all matching chunks
		WithFields(fields...).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	// Parse results
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %v", result.Errors)
	}

	data := result.Data["Get"].(map[string]interface{})
	objects, ok := data[s.collection].([]interface{})
	if !ok {
		return []core.Chunk{}, nil
	}

	var chunks []core.Chunk
	for _, obj := range objects {
		object := obj.(map[string]interface{})

		content := ""
		if c, ok := object["content"].(string); ok {
			content = c
		}

		id := ""
		if idVal, ok := object["chunk_id"].(string); ok {
			id = idVal
		}

		// Extract metadata
		chunkMetadata := make(map[string]string)
		for key := range metadata {
			if val, ok := object[key].(string); ok {
				chunkMetadata[key] = val
			}
		}

		chunks = append(chunks, core.Chunk{
			ID:       id,
			Content:  content,
			Metadata: chunkMetadata,
		})
	}

	return chunks, nil
}

func (s *Store) parseGraphQLResult(result *models.GraphQLResponse) ([]core.Result, error) {
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %v", result.Errors)
	}

	data := result.Data["Get"].(map[string]interface{})
	objects, ok := data[s.collection].([]interface{})
	if !ok {
		return []core.Result{}, nil
	}

	results := make([]core.Result, 0, len(objects))
	for _, obj := range objects {
		object := obj.(map[string]interface{})

		additional := object["_additional"].(map[string]interface{})
		var certainty float32
		if c, ok := additional["certainty"].(float64); ok {
			certainty = float32(c)
		}

		content := ""
		if c, ok := object["content"].(string); ok {
			content = c
		}

		id := ""
		if idVal, ok := object["chunk_id"].(string); ok {
			id = idVal
		}

		results = append(results, core.Result{
			Chunk: core.Chunk{
				ID:      id,
				Content: content,
			},
			Score: certainty,
		})
	}

	return results, nil
}
