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

// type Option func(*Store)
// func WithCollection(name string) Option { return func(s *Store) { s.collection = name } }
// func WithDimension(dim int) Option      { return func(s *Store) { s.dimension = dim } }
// func WithPort(port int) Option          { return func(s *Store) { s.port = port } }

// DefaultStore creates a Qdrant store pointing to localhost:6334 with a default collection "gorag" and dimension 1536.
func DefaultStore() (core.VectorStore, error) {
	return NewStore("gorag", 1536, "localhost", 6334)
}

func NewStore(collection string, dimension int, host string, port int) (core.VectorStore, error) {
	store := &Store{collection: collection, dimension: dimension, port: port}
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

func (s *Store) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	limit := uint64(topK)

	var qFilter *qdrant.Filter
	if len(filters) > 0 {
		var must []*qdrant.Condition
		for k, v := range filters {
			switch val := v.(type) {
			case string:
				must = append(must, qdrant.NewMatch(k, val))
			case int:
				must = append(must, qdrant.NewMatchInt(k, int64(val)))
			case int64:
				must = append(must, qdrant.NewMatchInt(k, val))
			case bool:
				must = append(must, qdrant.NewMatchBool(k, val))
			}
		}
		qFilter = &qdrant.Filter{Must: must}
	}

	req := &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQuery(query...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
		Filter:         qFilter,
	}
	res, err := s.client.Query(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	var outV []*core.Vector
	var outS []float32
	for _, hit := range res {
		metadata := make(map[string]any)
		chunkID := ""
		if hit.Payload != nil {
			for k, v := range hit.Payload {
				if k == "chunk_id" {
					chunkID = v.GetStringValue()
				} else {
					// Extract value based on type
					switch x := v.GetKind().(type) {
					case *qdrant.Value_StringValue:
						metadata[k] = x.StringValue
					case *qdrant.Value_IntegerValue:
						metadata[k] = x.IntegerValue
					case *qdrant.Value_DoubleValue:
						metadata[k] = x.DoubleValue
					case *qdrant.Value_BoolValue:
						metadata[k] = x.BoolValue
					}
				}
			}
		}

		outV = append(outV, core.NewVector(hit.Id.String(), nil, chunkID, metadata))
		outS = append(outS, float32(hit.Score))
	}
	return outV, outS, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
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
