package qdrant

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/qdrant/go-client/qdrant"
)

var _ core.VectorStore = (*Store)(nil)

type Store struct {
	client     *qdrant.Client
	collection string
	dimension  int
	port       int
}

type Option func(*Store)

func WithCollection(name string) Option { return func(s *Store) { s.collection = name } }
func WithDimension(dim int)     Option { return func(s *Store) { s.dimension = dim } }
func WithPort(port int)         Option { return func(s *Store) { s.port = port } }

func NewStore(ctx context.Context, addr string, opts ...Option) (*Store, error) {
	store := &Store{collection: "gorag", dimension: 1536, port: 6334}
	for _, opt := range opts {
		opt(store)
	}
	host := addr
	client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: store.port})
	if err != nil {
		return nil, err
	}
	store.client = client
	return store, nil
}

func (s *Store) Upsert(ctx context.Context, vectors []*core.Vector) error {
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

func (s *Store) Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*core.Vector, []float32, error) {
	limit := uint64(topK)
	req := &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQuery(query...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	}
	res, err := s.client.Query(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	var outV []*core.Vector
	var outS []float32
	for _, hit := range res {
		outV = append(outV, core.NewVector(hit.Id.String(), nil, "", nil))
		outS = append(outS, float32(hit.Score))
	}
	return outV, outS, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{Ids: []*qdrant.PointId{qdrant.NewIDUUID(id)}},
			},
		},
	})
	return err
}

func (s *Store) Close(ctx context.Context) error { return nil }
