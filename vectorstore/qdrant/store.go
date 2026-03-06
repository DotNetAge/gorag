package qdrant

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"github.com/DotNetAge/gorag/vectorstore"
)

// Store implements a vector store using Qdrant
type Store struct {
	client     *qdrant.Client
	collection string
	dimension  int
}

// Option configures the Qdrant store
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

// NewStore creates a new Qdrant vector store
func NewStore(ctx context.Context, addr string, opts ...Option) (*Store, error) {
	// Create Qdrant client
	// Parse address to get host and port
	// For simplicity, assume default port 6334
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: addr,
		Port: 6334,
	})
	if err != nil {
		return nil, err
	}

	store := &Store{
		client:     client,
		collection: "gorag",
		dimension:  1536, // Default for OpenAI embeddings
	}

	for _, opt := range opts {
		opt(store)
	}

	// Check if collection exists
	exists, err := store.client.CollectionExists(ctx, store.collection)
	if err != nil {
		return nil, err
	}

	if !exists {
		// Create collection
		err = store.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: store.collection,
			VectorsConfig: &qdrant.VectorsConfig{
				Config: &qdrant.VectorsConfig_Params{
					Params: &qdrant.VectorParams{
						Size:     uint64(store.dimension),
						Distance: qdrant.Distance_Cosine,
					},
				},
			},
		})
		if err != nil {
			return nil, err
		}
	}

	return store, nil
}

// Add adds chunks to the Qdrant store
func (s *Store) Add(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
	if len(chunks) == 0 || len(embeddings) == 0 || len(chunks) != len(embeddings) {
		return nil
	}

	// Prepare points
	points := make([]*qdrant.PointStruct, len(chunks))
	for i, chunk := range chunks {
		// Convert metadata to payload
		payload := make(map[string]*qdrant.Value)
		if chunk.Metadata != nil {
			for k, v := range chunk.Metadata {
				payload[k] = &qdrant.Value{
					Kind: &qdrant.Value_StringValue{StringValue: v},
				}
			}
		}
		payload["content"] = &qdrant.Value{
			Kind: &qdrant.Value_StringValue{StringValue: chunk.Content},
		}

		// Convert embedding to Vector type
		vector := &qdrant.Vector{
			Data: embeddings[i],
		}

		points[i] = &qdrant.PointStruct{
			Id: &qdrant.PointId{
				PointIdOptions: &qdrant.PointId_Uuid{Uuid: chunk.ID},
			},
			Vectors: &qdrant.Vectors{
				VectorsOptions: &qdrant.Vectors_Vector{Vector: vector},
			},
			Payload: payload,
		}
	}

	// Upsert points
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Points:         points,
	})

	return err
}

// Search performs similarity search in Qdrant
func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	hnswEf := uint64(128)
	exact := false
	limit := uint64(topK)

	// Prepare query request
	queryRequest := &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query: &qdrant.Query{
			Variant: &qdrant.Query_Nearest{
				Nearest: qdrant.NewVectorInputDense(query),
			},
		},
		Limit: &limit,
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true},
		},
		Params: &qdrant.SearchParams{
			HnswEf: &hnswEf,
			Exact:  &exact,
		},
	}

	// Add filter if provided
	if opts.Filter != nil {
		// Convert filter to Qdrant format
		// This is a simplification - in a real implementation, we'd need to handle complex filters
		queryRequest.Filter = &qdrant.Filter{
			Should: []*qdrant.Condition{},
		}
	}

	// Perform search
	response, err := s.client.Query(ctx, queryRequest)
	if err != nil {
		return nil, err
	}

	// Convert results
	var vectorResults []vectorstore.Result
	for _, hit := range response {
		// Extract content from payload
		var content string
		if hit.Payload != nil {
			if c, ok := hit.Payload["content"]; ok {
				if strV, ok := c.Kind.(*qdrant.Value_StringValue); ok {
					content = strV.StringValue
				}
			}
		}

		// Extract metadata
		metadata := make(map[string]string)
		if hit.Payload != nil {
			for k, v := range hit.Payload {
				if k != "content" {
					if strV, ok := v.Kind.(*qdrant.Value_StringValue); ok {
						metadata[k] = strV.StringValue
					}
				}
			}
		}

		// Get point ID
		id := ""
		if hit.Id != nil {
			switch idOpt := hit.Id.PointIdOptions.(type) {
			case *qdrant.PointId_Uuid:
				id = idOpt.Uuid
			case *qdrant.PointId_Num:
				id = fmt.Sprintf("%d", idOpt.Num)
			}
		}

		vectorResults = append(vectorResults, vectorstore.Result{
			Chunk: vectorstore.Chunk{
				ID:       id,
				Content:  content,
				Metadata: metadata,
			},
			Score: float32(hit.Score),
		})
	}

	return vectorResults, nil
}

// Delete removes chunks from the Qdrant store
func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Prepare point IDs
	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = &qdrant.PointId{PointIdOptions: &qdrant.PointId_Uuid{Uuid: id}}
	}

	// Delete points
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: pointIDs,
				},
			},
		},
	})

	return err
}

// Close closes the Qdrant client
func (s *Store) Close() error {
	// Qdrant client doesn't have a Close method in the current SDK
	// We'll just return nil for now
	return nil
}
