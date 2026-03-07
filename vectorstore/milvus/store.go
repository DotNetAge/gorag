package milvus

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// Store implements a vector store using Milvus
type Store struct {
	client     client.Client
	collection string
	dimension  int
	indexType  entity.IndexType
	metricType entity.MetricType
}

// Option configures the Milvus store
type Option func(*Store)

// WithCollection sets the collection name
func WithCollection(name string) Option {
	return func(s *Store) {
		s.collection = name
	}
}

// WithDimension sets the vector dimension
func WithDimension(dim int) Option {
	return func(s *Store) {
		s.dimension = dim
	}
}

// NewStore creates a new Milvus vector store
func NewStore(ctx context.Context, addr string, opts ...Option) (*Store, error) {
	// Create Milvus client
	c, err := client.NewClient(ctx, client.Config{
		Address: addr,
	})
	if err != nil {
		return nil, err
	}

	store := &Store{
		client:     c,
		collection: "gorag",
		dimension:  1536, // Default for OpenAI embeddings
		indexType:  entity.IvfFlat,
		metricType: entity.L2,
	}

	for _, opt := range opts {
		opt(store)
	}

	// Check if collection exists
	exists, err := store.client.HasCollection(ctx, store.collection)
	if err != nil {
		return nil, err
	}

	if !exists {
		// Create schema using the recommended way
		schema := entity.NewSchema().WithName(store.collection).WithDescription("GoRAG vector store").
			WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(true)).
			WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535)).
			WithField(entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(store.dimension)))

		err = store.client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
		if err != nil {
			return nil, err
		}
	}

	return store, nil
}

// Add adds chunks to the Milvus store
func (s *Store) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	if len(chunks) == 0 || len(embeddings) == 0 || len(chunks) != len(embeddings) {
		return nil
	}

	// Prepare data
	contents := make([]string, len(chunks))
	vectors := make([][]float32, len(embeddings))

	for i, chunk := range chunks {
		contents[i] = chunk.Content
		vectors[i] = embeddings[i]
	}

	// Insert data
	_, err := s.client.Insert(ctx, s.collection, "",
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnFloatVector("vector", s.dimension, vectors),
	)
	if err != nil {
		return err
	}

	// Flush to make data searchable
	err = s.client.Flush(ctx, s.collection, false)
	if err != nil {
		return err
	}

	// Create index after flush (following official example pattern)
	idx, err := entity.NewIndexIvfFlat(entity.L2, 2)
	if err != nil {
		return err
	}

	err = s.client.CreateIndex(ctx, s.collection, "vector", idx, false)
	if err != nil {
		return err
	}

	// Load collection to make new data visible
	return s.client.LoadCollection(ctx, s.collection, false)
}

// Search performs similarity search in Milvus
func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]core.Result, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Prepare search parameters
	sp, err := entity.NewIndexIvfFlatSearchParam(2)
	if err != nil {
		return nil, err
	}

	// Perform search
	results, err := s.client.Search(ctx, s.collection, []string{}, "", []string{"id", "content"},
		[]entity.Vector{entity.FloatVector(query)}, "vector",
		entity.L2, topK, sp)
	if err != nil {
		return nil, err
	}

	// Convert results
	var vectorResults []core.Result
	for _, result := range results {
		// Get content column
		contentCol, ok := result.Fields.GetColumn("content").(*entity.ColumnVarChar)
		if !ok || contentCol == nil {
			continue
		}

		// Get IDs
		idCol, ok := result.Fields.GetColumn("id").(*entity.ColumnInt64)
		if !ok || idCol == nil {
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

			vectorResults = append(vectorResults, core.Result{
				Chunk: core.Chunk{
					ID:      fmt.Sprintf("%d", id),
					Content: content,
				},
				Score: result.Scores[i],
			})
		}
	}

	return vectorResults, nil
}

// Delete removes chunks from the Milvus store
func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Convert string IDs to int64
	intIDs := make([]int64, len(ids))
	for i, id := range ids {
		var intID int64
		_, err := fmt.Sscanf(id, "%d", &intID)
		if err != nil {
			continue
		}
		intIDs[i] = intID
	}

	// Delete by ID using expression
	expr := fmt.Sprintf("id in [%s]", intIDsToString(intIDs))
	return s.client.Delete(ctx, s.collection, "", expr)
}

// intIDsToString converts int64 slice to comma-separated string
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

// Close closes the Milvus client
func (s *Store) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
