package milvus

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/DotNetAge/gorag/vectorstore"
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
		metricType: entity.IP,
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
		// Create schema
		schema := &entity.Schema{
			CollectionName: store.collection,
			Description:    "GoRAG vector store",
			Fields: []*entity.Field{
				{
					Name:       "id",
					DataType:   entity.FieldTypeInt64,
					PrimaryKey: true,
					AutoID:     true,
				},
				{
					Name:     "content",
					DataType: entity.FieldTypeVarChar,
					TypeParams: map[string]string{
						"max_length": "65535",
					},
				},
				{
					Name:     "vector",
					DataType: entity.FieldTypeFloatVector,
					TypeParams: map[string]string{
						"dim": fmt.Sprintf("%d", store.dimension),
					},
				},
			},
		}

		err = store.client.CreateCollection(ctx, schema, 2)
		if err != nil {
			return nil, err
		}

		// Create index
		idx, err := entity.NewIndexIvfFlat(entity.IP, 128)
		if err != nil {
			return nil, err
		}

		err = store.client.CreateIndex(ctx, store.collection, "vector", idx, false)
		if err != nil {
			return nil, err
		}
	}

	// Load collection
	err = store.client.LoadCollection(ctx, store.collection, false)
	if err != nil {
		return nil, err
	}

	return store, nil
}

// Add adds chunks to the Milvus store
func (s *Store) Add(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
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

	return err
}

// Search performs similarity search in Milvus
func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Prepare search parameters
	sp, err := entity.NewIndexIvfFlatSearchParam(10)
	if err != nil {
		return nil, err
	}

	// Perform search
	results, err := s.client.Search(ctx, s.collection, []string{}, "", []string{"content"},
		[]entity.Vector{entity.FloatVector(query)}, "vector",
		entity.IP, topK, sp)
	if err != nil {
		return nil, err
	}

	// Convert results
	var vectorResults []vectorstore.Result
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

			vectorResults = append(vectorResults, vectorstore.Result{
				Chunk: vectorstore.Chunk{
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
