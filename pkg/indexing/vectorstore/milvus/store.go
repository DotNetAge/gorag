package milvus

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"encoding/json"
	"fmt"
	"time"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	milvusEntity "github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// ensure interface implementation
var _ core.VectorStore = (*Store)(nil)

type Store struct {
	client     client.Client
	collection string
	dimension  int
	indexType  milvusEntity.IndexType
	metricType milvusEntity.MetricType
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
		indexType:  milvusEntity.IvfFlat,
		metricType: milvusEntity.L2,
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
		schema := milvusEntity.NewSchema().WithName(store.collection).WithDescription("GoRAG vector store").
			WithField(milvusEntity.NewField().WithName("id").WithDataType(milvusEntity.FieldTypeVarChar).WithMaxLength(256).WithIsPrimaryKey(true)).
			WithField(milvusEntity.NewField().WithName("chunk_id").WithDataType(milvusEntity.FieldTypeVarChar).WithMaxLength(256)).
			WithField(milvusEntity.NewField().WithName("metadata").WithDataType(milvusEntity.FieldTypeVarChar).WithMaxLength(65535)).
			WithField(milvusEntity.NewField().WithName("vector").WithDataType(milvusEntity.FieldTypeFloatVector).WithDim(int64(store.dimension)))

		// Retry CreateCollection
		deadline = time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			err = store.client.CreateCollection(ctx, schema, milvusEntity.DefaultShardNumber)
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

func (s *Store) Add(ctx context.Context, vector *core.Vector) error {
	return s.AddBatch(ctx, []*core.Vector{vector})
}

func (s *Store) AddBatch(ctx context.Context, vectors []*core.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	ids := make([]string, len(vectors))
	chunkIDs := make([]string, len(vectors))
	metadataJSONs := make([]string, len(vectors))
	vecs := make([][]float32, len(vectors))

	for i, v := range vectors {
		ids[i] = v.ID
		chunkIDs[i] = v.ChunkID
		vecs[i] = v.Values

		// Serialize metadata to JSON
		metadataJSON, err := json.Marshal(v.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSONs[i] = string(metadataJSON)
	}

	_, err := s.client.Insert(ctx, s.collection, "",
		milvusEntity.NewColumnVarChar("id", ids),
		milvusEntity.NewColumnVarChar("chunk_id", chunkIDs),
		milvusEntity.NewColumnVarChar("metadata", metadataJSONs),
		milvusEntity.NewColumnFloatVector("vector", s.dimension, vecs),
	)
	if err != nil {
		return err
	}

	err = s.client.Flush(ctx, s.collection, false)
	if err != nil {
		return err
	}

	idx, err := milvusEntity.NewIndexIvfFlat(s.metricType, 2)
	if err != nil {
		return err
	}

	err = s.client.CreateIndex(ctx, s.collection, "vector", idx, false)
	if err != nil {
		return err
	}

	return s.client.LoadCollection(ctx, s.collection, false)
}

func (s *Store) Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*core.Vector, []float32, error) {
	if topK <= 0 {
		topK = 5
	}

	sp, err := milvusEntity.NewIndexIvfFlatSearchParam(2)
	if err != nil {
		return nil, nil, err
	}

	// Build expression for metadata search
	expr := ""
	if len(filter) > 0 {
		for key, value := range filter {
			if expr != "" {
				expr += " and "
			}
			// In Milvus, using JSON metadata field
			expr += fmt.Sprintf("contains(metadata, '%s:%v')", key, value)
		}
	}

	results, err := s.client.Search(ctx, s.collection, []string{}, expr, []string{"id", "chunk_id", "metadata"},
		[]milvusEntity.Vector{milvusEntity.FloatVector(query)}, "vector",
		s.metricType, topK, sp)
	if err != nil {
		return nil, nil, err
	}

	return s.parseResults(results)
}

func (s *Store) Delete(ctx context.Context, id string) error {
	return s.DeleteBatch(ctx, []string{id})
}

func (s *Store) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Because we use VarChar as PK now, format the expr properly
	idsStr := "'" + ids[0] + "'"
	for i := 1; i < len(ids); i++ {
		idsStr += ", '" + ids[i] + "'"
	}

	expr := fmt.Sprintf("id in [%s]", idsStr)
	return s.client.Delete(ctx, s.collection, "", expr)
}

func (s *Store) Close(ctx context.Context) error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *Store) parseResults(results []client.SearchResult) ([]*core.Vector, []float32, error) {
	var outVectors []*core.Vector
	var outScores []float32

	for _, result := range results {
		idCol, ok := result.Fields.GetColumn("id").(*milvusEntity.ColumnVarChar)
		if !ok || idCol == nil {
			continue
		}

		chunkIdCol, ok := result.Fields.GetColumn("chunk_id").(*milvusEntity.ColumnVarChar)
		if !ok || chunkIdCol == nil {
			continue
		}

		metadataCol, ok := result.Fields.GetColumn("metadata").(*milvusEntity.ColumnVarChar)
		if !ok || metadataCol == nil {
			continue
		}

		for i := 0; i < result.ResultCount; i++ {
			id, err := idCol.GetAsString(i)
			if err != nil {
				continue
			}

			chunkID, err := chunkIdCol.GetAsString(i)
			if err != nil {
				continue
			}

			metadataJSON, err := metadataCol.GetAsString(i)
			if err != nil {
				continue
			}

			var metadata map[string]any
			if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
				metadata = make(map[string]any)
			}

			vec := core.NewVector(id, nil, chunkID, metadata)
			outVectors = append(outVectors, vec)
			outScores = append(outScores, result.Scores[i])
		}
	}
	return outVectors, outScores, nil
}

func (s *Store) Upsert(ctx context.Context, vectors []*core.Vector) error { return s.AddBatch(ctx, vectors) }
