package milvus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type Store struct {
	client     client.Client
	collection string
	dimension  int
	indexType  entity.IndexType
	metricType entity.MetricType
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

func NewStore(ctx context.Context, addr string, opts ...Option) (*Store, error) {
	// Create client with retry
	var c client.Client
	var err error

	// Retry for up to 30 seconds
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		c, err = client.NewClient(ctx, client.Config{
			Address: addr,
		})
		if err == nil {
			// Test if client is ready
			_, err = c.ListCollections(ctx)
			if err == nil {
				break
			}
			// Client created but not ready, close and retry
			c.Close()
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Milvus client: %w", err)
	}

	store := &Store{
		client:     c,
		collection: "gorag",
		dimension:  1536,
		indexType:  entity.IvfFlat,
		metricType: entity.L2,
	}

	for _, opt := range opts {
		opt(store)
	}

	// Retry HasCollection
	var exists bool
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		exists, err = store.client.HasCollection(ctx, store.collection)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to check collection existence: %w", err)
	}

	if !exists {
		schema := entity.NewSchema().WithName(store.collection).WithDescription("GoRAG vector store").
			WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(true)).
			WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535)).
			WithField(entity.NewField().WithName("metadata").WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535)).
			WithField(entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(store.dimension)))

		// Retry CreateCollection
		deadline = time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			err = store.client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create collection: %w", err)
		}
	}

	return store, nil
}

func (s *Store) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	if len(chunks) == 0 || len(embeddings) == 0 || len(chunks) != len(embeddings) {
		return nil
	}

	contents := make([]string, len(chunks))
	metadataJSONs := make([]string, len(chunks))
	vectors := make([][]float32, len(embeddings))

	for i, chunk := range chunks {
		contents[i] = chunk.Content
		vectors[i] = embeddings[i]

		// Serialize metadata to JSON
		metadataJSON, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSONs[i] = string(metadataJSON)
	}

	_, err := s.client.Insert(ctx, s.collection, "",
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnVarChar("metadata", metadataJSONs),
		entity.NewColumnFloatVector("vector", s.dimension, vectors),
	)
	if err != nil {
		return err
	}

	err = s.client.Flush(ctx, s.collection, false)
	if err != nil {
		return err
	}

	idx, err := entity.NewIndexIvfFlat(entity.L2, 2)
	if err != nil {
		return err
	}

	err = s.client.CreateIndex(ctx, s.collection, "vector", idx, false)
	if err != nil {
		return err
	}

	return s.client.LoadCollection(ctx, s.collection, false)
}

func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]core.Result, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	sp, err := entity.NewIndexIvfFlatSearchParam(2)
	if err != nil {
		return nil, err
	}

	results, err := s.client.Search(ctx, s.collection, []string{}, "", []string{"id", "content", "metadata"},
		[]entity.Vector{entity.FloatVector(query)}, "vector",
		entity.L2, topK, sp)
	if err != nil {
		return nil, err
	}

	return s.parseResults(results), nil
}

func (s *Store) SearchStructured(ctx context.Context, query *vectorstore.StructuredQuery, embedding []float32) ([]core.Result, error) {
	topK := query.TopK
	if topK <= 0 {
		topK = 5
	}

	sp, err := entity.NewIndexIvfFlatSearchParam(2)
	if err != nil {
		return nil, err
	}

	// Perform search without filters (Milvus doesn't support filtering on JSON metadata)
	results, err := s.client.Search(ctx, s.collection, []string{}, "", []string{"id", "content", "metadata"},
		[]entity.Vector{entity.FloatVector(embedding)}, "vector",
		entity.L2, topK*2, sp) // Get more results to filter
	if err != nil {
		return nil, err
	}

	// Parse results
	parsedResults := s.parseResults(results)

	// Filter results on client side
	var filteredResults []core.Result
	for _, result := range parsedResults {
		match := true
		for _, filter := range query.Filters {
			if value, ok := result.Chunk.Metadata[filter.Field]; !ok || value != filter.Value {
				match = false
				break
			}
		}
		if match {
			filteredResults = append(filteredResults, result)
			if len(filteredResults) >= topK {
				break
			}
		}
	}

	return filteredResults, nil
}

func (s *Store) GetByMetadata(ctx context.Context, metadata map[string]string) ([]core.Result, error) {
	sp, err := entity.NewIndexIvfFlatSearchParam(2)
	if err != nil {
		return nil, err
	}

	dummyVector := make([]float32, s.dimension)
	// Perform search without filters (Milvus doesn't support filtering on JSON metadata)
	results, err := s.client.Search(ctx, s.collection, []string{}, "", []string{"id", "content", "metadata"},
		[]entity.Vector{entity.FloatVector(dummyVector)}, "vector",
		entity.L2, 100, sp) // Get more results to filter
	if err != nil {
		return nil, err
	}

	// Parse results
	parsedResults := s.parseResults(results)

	// Filter results on client side
	var filteredResults []core.Result
	for _, result := range parsedResults {
		match := true
		for k, v := range metadata {
			if value, ok := result.Chunk.Metadata[k]; !ok || value != v {
				match = false
				break
			}
		}
		if match {
			result.Score = 1.0
			filteredResults = append(filteredResults, result)
		}
	}

	return filteredResults, nil
}

func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	intIDs := make([]int64, len(ids))
	for i, id := range ids {
		var intID int64
		_, err := fmt.Sscanf(id, "%d", &intID)
		if err != nil {
			continue
		}
		intIDs[i] = intID
	}

	expr := fmt.Sprintf("id in [%s]", intIDsToString(intIDs))
	return s.client.Delete(ctx, s.collection, "", expr)
}

func intIDsToString(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	result := fmt.Sprintf("%d", ids[0])
	for i := 1; i < len(ids); i++ {
		result += fmt.Sprintf(",%d", ids[i])
	}
	return result
}

func (s *Store) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *Store) parseResults(results []client.SearchResult) []core.Result {
	var vectorResults []core.Result
	for _, result := range results {
		contentCol, ok := result.Fields.GetColumn("content").(*entity.ColumnVarChar)
		if !ok || contentCol == nil {
			continue
		}

		idCol, ok := result.Fields.GetColumn("id").(*entity.ColumnInt64)
		if !ok || idCol == nil {
			continue
		}

		metadataCol, ok := result.Fields.GetColumn("metadata").(*entity.ColumnVarChar)
		if !ok || metadataCol == nil {
			continue
		}

		for i := 0; i < result.ResultCount; i++ {
			content, err := contentCol.GetAsString(i)
			if err != nil {
				continue
			}

			id, err := idCol.GetAsInt64(i)
			if err != nil {
				continue
			}

			metadataJSON, err := metadataCol.GetAsString(i)
			if err != nil {
				continue
			}

			var metadata map[string]string
			if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
				metadata = make(map[string]string)
			}

			vectorResults = append(vectorResults, core.Result{
				Chunk: core.Chunk{
					ID:       fmt.Sprintf("%d", id),
					Content:  content,
					Metadata: metadata,
				},
				Score: result.Scores[i],
			})
		}
	}
	return vectorResults
}
