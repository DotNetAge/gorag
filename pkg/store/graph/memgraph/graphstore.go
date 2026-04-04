package memgraph

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type memgraphStore struct {
	driver neo4j.DriverWithContext
	dbName string
}

type Options struct {
	URI      string
	Username string
	Password string
}

type Option func(*Options)

func WithURI(uri string) Option {
	return func(o *Options) {
		o.URI = uri
	}
}

func WithAuth(username, password string) Option {
	return func(o *Options) {
		o.Username = username
		o.Password = password
	}
}

func defaultOptions() *Options {
	return &Options{
		URI:      "bolt://localhost:7687",
		Username: "",
		Password: "",
	}
}

// DefaultGraphStore creates a Memgraph GraphStore using default connection settings.
func DefaultGraphStore(opts ...Option) (core.GraphStore, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return NewGraphStore(options.URI, options.Username, options.Password)
}

// NewGraphStore creates a new Memgraph based graph core.
func NewGraphStore(uri, username, password string) (core.GraphStore, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create memgraph driver: %w", err)
	}

	return &memgraphStore{
		driver: driver,
		dbName: "memgraph",
	}, nil
}

func (s *memgraphStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, node := range nodes {
			query := fmt.Sprintf("MERGE (n:%s {id: $id}) SET n += $props", node.Type)
			params := map[string]any{
				"id":    node.ID,
				"props": node.Properties,
			}
			if _, err := tx.Run(ctx, query, params); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (s *memgraphStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, edge := range edges {
			query := fmt.Sprintf(
				"MATCH (s {id: $source}), (t {id: $target}) "+
					"MERGE (s)-[r:%s {id: $id}]->(t) "+
					"SET r += $props", edge.Type)

			params := map[string]any{
				"source": edge.Source,
				"target": edge.Target,
				"id":     edge.ID,
				"props":  edge.Properties,
			}
			if _, err := tx.Run(ctx, query, params); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (s *memgraphStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := "MATCH (n {id: $id}) RETURN labels(n) as labels, properties(n) as props"
		res, err := tx.Run(ctx, query, map[string]any{"id": id})
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			record := res.Record()
			labels, _ := record.Get("labels")
			props, _ := record.Get("props")

			node := &core.Node{
				ID:         id,
				Properties: props.(map[string]any),
			}
			if l, ok := labels.([]any); ok && len(l) > 0 {
				node.Type = l[0].(string)
			}
			return node, nil
		}
		return nil, nil
	})

	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.(*core.Node), nil
}

func (s *memgraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	if depth <= 0 {
		depth = 1
	}

	type graphData struct {
		nodes []*core.Node
		edges []*core.Edge
	}

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// FIXED: Respect the depth parameter using variable length paths.
		query := fmt.Sprintf("MATCH (n {id: $id})-[r*1..%d]-(m) RETURN DISTINCT m, r LIMIT $limit", depth)
		res, err := tx.Run(ctx, query, map[string]any{"id": nodeID, "limit": limit})
		if err != nil {
			return nil, err
		}

		data := &graphData{}
		visitedNodes := make(map[string]bool)
		visitedEdges := make(map[string]bool)

		for res.Next(ctx) {
			record := res.Record()

			// Parse Node
			mRaw, _ := record.Get("m")
			if mRaw != nil {
				mNode := mRaw.(neo4j.Node)
				mID, _ := mNode.Props["id"].(string)

				if mID != "" && !visitedNodes[mID] {
					visitedNodes[mID] = true
					node := &core.Node{
						ID:         mID,
						Properties: mNode.Props,
					}
					if len(mNode.Labels) > 0 {
						node.Type = mNode.Labels[0]
					}
					data.nodes = append(data.nodes, node)
				}
			}

			// Parse Relationship (r can be a relationship or a list of relationships)
			rRaw, _ := record.Get("r")
			if rRaw != nil {
				processRel := func(rel neo4j.Relationship) {
					rID, _ := rel.Props["id"].(string)
					if rID != "" && !visitedEdges[rID] {
						visitedEdges[rID] = true
						edge := &core.Edge{
							ID:         rID,
							Type:       rel.Type,
							Properties: rel.Props,
						}
						data.edges = append(data.edges, edge)
					}
				}

				switch v := rRaw.(type) {
				case neo4j.Relationship:
					processRel(v)
				case []any:
					for _, item := range v {
						if rel, ok := item.(neo4j.Relationship); ok {
							processRel(rel)
						}
					}
				}
			}
		}
		return data, nil
	})

	if err != nil {
		return nil, nil, err
	}
	d := result.(*graphData)
	return d.nodes, d.edges, nil
}

func (s *memgraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var rows []map[string]any
		for res.Next(ctx) {
			rows = append(rows, res.Record().AsMap())
		}
		return rows, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]map[string]any), nil
}

func (s *memgraphStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	query := "MATCH (c:Community {level: $level}) RETURN c.id as id, c.summary as summary, c.nodes as nodes"
	return s.Query(ctx, query, map[string]any{"level": level})
}

func (s *memgraphStore) DeleteNode(ctx context.Context, id string) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, "MATCH (n {id: $id}) DETACH DELETE n", map[string]any{"id": id})
		return nil, err
	})
	return err
}

func (s *memgraphStore) DeleteEdge(ctx context.Context, id string) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, "MATCH ()-[r {id: $id}]-() DELETE r", map[string]any{"id": id})
		return nil, err
	})
	return err
}

func (s *memgraphStore) Close(ctx context.Context) error {
	return s.driver.Close(ctx)
}
