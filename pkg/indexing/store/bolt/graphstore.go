package bolt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	bolt "go.etcd.io/bbolt"
)

var (
	nodeBucket          = []byte("nodes")
	edgeBucket          = []byte("edges")
	outEdgesBucket      = []byte("out_edges")      // source -> []edgeID
	inEdgesBucket       = []byte("in_edges")       // target -> []edgeID
	communityBucket      = []byte("communities")
)

type boltGraphStore struct {
	db *bolt.DB
}

// DefaultGraphStore creates a Bolt GraphStore using a default local file "gorag_graph.bolt".
func DefaultGraphStore() (store.GraphStore, error) {
	return NewGraphStore("gorag_graph.bolt")
}

// NewGraphStore creates a new BoltDB based graph store.
func NewGraphStore(path string) (store.GraphStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt db: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{nodeBucket, edgeBucket, outEdgesBucket, inEdgesBucket, communityBucket}
		for _, b := range buckets {
			_, err := tx.CreateBucketIfNotExists(b)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return &boltGraphStore{db: db}, nil
}

func (s *boltGraphStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(nodeBucket)
		for _, node := range nodes {
			data, err := json.Marshal(node)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(node.ID), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *boltGraphStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		eb := tx.Bucket(edgeBucket)
		ob := tx.Bucket(outEdgesBucket)
		ib := tx.Bucket(inEdgesBucket)

		for _, edge := range edges {
			data, err := json.Marshal(edge)
			if err != nil {
				return err
			}
			if err := eb.Put([]byte(edge.ID), data); err != nil {
				return err
			}

			// Update adjacency lists
			if err := s.updateAdjacency(ob, edge.Source, edge.ID); err != nil {
				return err
			}
			if err := s.updateAdjacency(ib, edge.Target, edge.ID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *boltGraphStore) updateAdjacency(b *bolt.Bucket, nodeID, edgeID string) error {
	var edgeIDs []string
	v := b.Get([]byte(nodeID))
	if v != nil {
		if err := json.Unmarshal(v, &edgeIDs); err != nil {
			return err
		}
	}

	// Check for duplicates
	exists := false
	for _, id := range edgeIDs {
		if id == edgeID {
			exists = true
			break
		}
	}

	if !exists {
		edgeIDs = append(edgeIDs, edgeID)
		data, err := json.Marshal(edgeIDs)
		if err != nil {
			return err
		}
		return b.Put([]byte(nodeID), data)
	}
	return nil
}

func (s *boltGraphStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	var node core.Node
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(nodeBucket)
		v := b.Get([]byte(id))
		if v == nil {
			return nil // Not found is not an error here based on implementation
		}
		return json.Unmarshal(v, &node)
	})
	if err != nil {
		return nil, err
	}
	if node.ID == "" {
		return nil, nil
	}
	return &node, nil
}

func (s *boltGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	if depth <= 0 {
		depth = 1
	}
	if limit <= 0 {
		limit = 100
	}

	visitedNodes := make(map[string]bool)
	visitedEdges := make(map[string]bool)
	nodes := make([]*core.Node, 0)
	edges := make([]*core.Edge, 0)

	queue := []string{nodeID}
	visitedNodes[nodeID] = true
	
	currentDepth := 0
	for len(queue) > 0 && currentDepth < depth && len(nodes) < limit {
		nextQueue := []string{}
		for _, currentID := range queue {
			// Get out edges and in edges
			connectedEdgeIDs := []string{}
			err := s.db.View(func(tx *bolt.Tx) error {
				ob := tx.Bucket(outEdgesBucket)
				ib := tx.Bucket(inEdgesBucket)

				if v := ob.Get([]byte(currentID)); v != nil {
					var ids []string
					if err := json.Unmarshal(v, &ids); err == nil {
						connectedEdgeIDs = append(connectedEdgeIDs, ids...)
					}
				}
				if v := ib.Get([]byte(currentID)); v != nil {
					var ids []string
					if err := json.Unmarshal(v, &ids); err == nil {
						connectedEdgeIDs = append(connectedEdgeIDs, ids...)
					}
				}
				return nil
			})
			if err != nil {
				return nil, nil, err
			}

			for _, edgeID := range connectedEdgeIDs {
				if visitedEdges[edgeID] {
					continue
				}
				visitedEdges[edgeID] = true

				var edge core.Edge
				err := s.db.View(func(tx *bolt.Tx) error {
					eb := tx.Bucket(edgeBucket)
					v := eb.Get([]byte(edgeID))
					if v != nil {
						return json.Unmarshal(v, &edge)
					}
					return nil
				})
				if err != nil {
					return nil, nil, err
				}

				if edge.ID != "" {
					edges = append(edges, &edge)
					
					neighborID := edge.Source
					if neighborID == currentID {
						neighborID = edge.Target
					}

					if !visitedNodes[neighborID] {
						visitedNodes[neighborID] = true
						node, err := s.GetNode(ctx, neighborID)
						if err != nil {
							return nil, nil, err
						}
						if node != nil {
							nodes = append(nodes, node)
							nextQueue = append(nextQueue, neighborID)
						}
					}
				}
				
				if len(nodes) >= limit {
					break
				}
			}
			if len(nodes) >= limit {
				break
			}
		}
		queue = nextQueue
		currentDepth++
	}

	return nodes, edges, nil
}

func (s *boltGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	// BoltDB is a KV store, so it doesn't support complex queries natively.
	// This would require a custom query engine or a Cypher parser.
	return nil, fmt.Errorf("complex queries not supported in BoltDB implementation yet")
}

func (s *boltGraphStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	var results []map[string]any
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(communityBucket)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var m map[string]any
			if err := json.Unmarshal(v, &m); err != nil {
				continue
			}
			if lvl, ok := m["level"].(float64); ok && int(lvl) == level {
				results = append(results, m)
			}
		}
		return nil
	})
	return results, err
}

func (s *boltGraphStore) Close(ctx context.Context) error {
	return s.db.Close()
}
