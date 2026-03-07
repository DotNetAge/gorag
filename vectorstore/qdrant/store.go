package qdrant

import (
	"context"

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

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: addr,
		Port: store.port,
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

		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(chunk.ID),
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

	var vectorResults []core.Result
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
		if hit.Id != nil {
			id = hit.Id.GetUuid()
		}

		vectorResults = append(vectorResults, core.Result{
			Chunk: core.Chunk{
				ID:       id,
				Content:  content,
				Metadata: metadata,
			},
			Score: float32(hit.Score),
		})
	}

	return vectorResults, nil
}

func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = qdrant.NewIDUUID(id)
	}

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

func (s *Store) Close() error {
	return nil
}
