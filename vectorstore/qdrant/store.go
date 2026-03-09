package qdrant

import (
	"context"
	"strconv"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/qdrant/go-client/qdrant"
)

type Store struct {
	client     *qdrant.Client
	collection string
	dimension  int
	port       int
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

func WithPort(port int) Option {
	return func(s *Store) {
		s.port = port
	}
}

func NewStore(ctx context.Context, addr string, opts ...Option) (*Store, error) {
	store := &Store{
		collection: "gorag",
		dimension:  1536,
		port:       6334,
	}

	for _, opt := range opts {
		opt(store)
	}

	// Parse host and port from addr
	host := addr
	port := store.port

	// If addr contains a port, split it
	if len(addr) > 0 && addr[0] != ':' {
		for i := len(addr) - 1; i >= 0; i-- {
			if addr[i] == ':' {
				host = addr[:i]
				// Extract port from addr
				portStr := addr[i+1:]
				if portStr != "" {
					// Convert port string to int
					if p, err := strconv.Atoi(portStr); err == nil {
						port = p
					}
				}
				break
			}
		}
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, err
	}

	store.client = client

	exists, err := store.client.CollectionExists(ctx, store.collection)
	if err != nil {
		return nil, err
	}

	if !exists {
		err = store.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: store.collection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     uint64(store.dimension),
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			return nil, err
		}
	}

	return store, nil
}

func (s *Store) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	if len(chunks) == 0 || len(embeddings) == 0 || len(chunks) != len(embeddings) {
		return nil
	}

	points := make([]*qdrant.PointStruct, len(chunks))
	for i, chunk := range chunks {
		payload := make(map[string]any)
		if chunk.Metadata != nil {
			for k, v := range chunk.Metadata {
				payload[k] = v
			}
		}
		payload["content"] = chunk.Content
		payload["id"] = chunk.ID

		// Use integer ID for Qdrant
		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(uint64(i + 1)),
			Vectors: qdrant.NewVectors(embeddings[i]...),
			Payload: qdrant.NewValueMap(payload),
		}
	}

	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Points:         points,
	})

	return err
}

func (s *Store) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]core.Result, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	limit := uint64(topK)

	queryRequest := &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQuery(query...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	}

	if opts.Filter != nil {
		queryRequest.Filter = &qdrant.Filter{
			Should: []*qdrant.Condition{},
		}
	}

	response, err := s.client.Query(ctx, queryRequest)
	if err != nil {
		return nil, err
	}

	return s.parseResults(response), nil
}

func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Create filter for ids
	conditions := make([]*qdrant.Condition, len(ids))
	for i, id := range ids {
		conditions[i] = qdrant.NewMatchKeyword("id", id)
	}

	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{
					Should: conditions,
				},
			},
		},
	})

	return err
}

func (s *Store) Close() error {
	return nil
}

// SearchByMetadata searches for chunks with matching metadata
func (s *Store) SearchByMetadata(ctx context.Context, metadata map[string]string) ([]core.Chunk, error) {
	if len(metadata) == 0 {
		return []core.Chunk{}, nil
	}

	// Build filter conditions
	conditions := make([]*qdrant.Condition, 0, len(metadata))
	for key, value := range metadata {
		conditions = append(conditions, qdrant.NewMatchKeyword(key, value))
	}

	// Create filter
	filter := &qdrant.Filter{
		Must: conditions,
	}

	// Search with filter
	limit := uint64(1000) // Use large limit to get all matching chunks
	queryRequest := &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQuery(make([]float32, s.dimension)...), // Use dummy vector
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
		Filter:         filter,
	}

	response, err := s.client.Query(ctx, queryRequest)
	if err != nil {
		return nil, err
	}

	// Parse results
	var chunks []core.Chunk
	for _, hit := range response {
		var content string
		if hit.Payload != nil {
			if c, ok := hit.Payload["content"]; ok {
				if strV, ok := c.GetKind().(*qdrant.Value_StringValue); ok {
					content = strV.StringValue
				}
			}
		}

		chunkMetadata := make(map[string]string)
		if hit.Payload != nil {
			for k, v := range hit.Payload {
				if k != "content" && k != "id" {
					if strV, ok := v.GetKind().(*qdrant.Value_StringValue); ok {
						chunkMetadata[k] = strV.StringValue
					}
				}
			}
		}

		id := ""
		if hit.Payload != nil {
			if idVal, ok := hit.Payload["id"]; ok {
				if strV, ok := idVal.GetKind().(*qdrant.Value_StringValue); ok {
					id = strV.StringValue
				}
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

func (s *Store) parseResults(response []*qdrant.ScoredPoint) []core.Result {
	var results []core.Result
	for _, hit := range response {
		var content string
		if hit.Payload != nil {
			if c, ok := hit.Payload["content"]; ok {
				if strV, ok := c.GetKind().(*qdrant.Value_StringValue); ok {
					content = strV.StringValue
				}
			}
		}

		metadata := make(map[string]string)
		if hit.Payload != nil {
			for k, v := range hit.Payload {
				if k != "content" {
					if strV, ok := v.GetKind().(*qdrant.Value_StringValue); ok {
						metadata[k] = strV.StringValue
					}
				}
			}
		}

		id := ""
		if hit.Payload != nil {
			if idVal, ok := hit.Payload["id"]; ok {
				if strV, ok := idVal.GetKind().(*qdrant.Value_StringValue); ok {
					id = strV.StringValue
				}
			}
		}

		results = append(results, core.Result{
			Chunk: core.Chunk{
				ID:       id,
				Content:  content,
				Metadata: metadata,
			},
			Score: float32(hit.Score),
		})
	}
	return results
}
