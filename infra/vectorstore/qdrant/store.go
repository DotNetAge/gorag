package qdrant

import (
	"context"
	"fmt"
	"strconv"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/qdrant/go-client/qdrant"
)

// ensure interface implementation
var _ abstraction.VectorStore = (*Store)(nil)

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

	host := addr
	port := store.port

	if len(addr) > 0 && addr[0] != ':' {
		for i := len(addr) - 1; i >= 0; i-- {
			if addr[i] == ':' {
				host = addr[:i]
				portStr := addr[i+1:]
				if portStr != "" {
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

func (s *Store) Add(ctx context.Context, vector *entity.Vector) error {
	return s.AddBatch(ctx, []*entity.Vector{vector})
}

func (s *Store) AddBatch(ctx context.Context, vectors []*entity.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	points := make([]*qdrant.PointStruct, len(vectors))
	for i, v := range vectors {
		payload := make(map[string]any)
		if v.Metadata != nil {
			for k, val := range v.Metadata {
				payload[k] = val
			}
		}
		
		payload["chunk_id"] = v.ChunkID

		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(v.ID), 
			Vectors: qdrant.NewVectors(v.Values...),
			Payload: qdrant.NewValueMap(payload),
		}
	}

	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Points:         points,
	})

	return err
}

func (s *Store) Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
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

	if len(filter) > 0 {
		conditions := make([]*qdrant.Condition, 0, len(filter))
		for k, v := range filter {
			conditions = append(conditions, qdrant.NewMatchKeyword(k, fmt.Sprintf("%v", v)))
		}
		queryRequest.Filter = &qdrant.Filter{
			Must: conditions,
		}
	}

	response, err := s.client.Query(ctx, queryRequest)
	if err != nil {
		return nil, nil, err
	}

	var outVectors []*entity.Vector
	var outScores []float32

	for _, hit := range response {
		meta := make(map[string]any)
		chunkID := ""

		if hit.Payload != nil {
			for k, v := range hit.Payload {
				if k == "chunk_id" {
					if strV, ok := v.GetKind().(*qdrant.Value_StringValue); ok {
						chunkID = strV.StringValue
					}
					continue
				}
				if strV, ok := v.GetKind().(*qdrant.Value_StringValue); ok {
					meta[k] = strV.StringValue
				}
			}
		}

		id := ""
		if hit.Id != nil {
			if uuidVal, ok := hit.Id.PointIdOptions.(*qdrant.PointId_Uuid); ok {
				id = uuidVal.Uuid
			} else if numVal, ok := hit.Id.PointIdOptions.(*qdrant.PointId_Num); ok {
				id = fmt.Sprintf("%d", numVal.Num)
			}
		}

		vec := entity.NewVector(id, nil, chunkID, meta)
		
		outVectors = append(outVectors, vec)
		outScores = append(outScores, float32(hit.Score))
	}

	return outVectors, outScores, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	return s.DeleteBatch(ctx, []string{id})
}

func (s *Store) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	points := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		points[i] = qdrant.NewIDUUID(id)
	}

	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: points,
				},
			},
		},
	})

	return err
}

func (s *Store) Close(ctx context.Context) error {
	s.client = nil
	return nil
}
