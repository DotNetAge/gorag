package falkordb

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/falkorDB/falkordb-go"
)

type falkorGraphStore struct {
	client *falkordb.FalkorDB
	graph  *falkordb.Graph
}

type Options struct {
	URI     string
	GraphID string
}

type Option func(*Options)

func WithURI(uri string) Option {
	return func(o *Options) {
		o.URI = uri
	}
}

func WithGraphID(id string) Option {
	return func(o *Options) {
		o.GraphID = id
	}
}

func defaultOptions() *Options {
	return &Options{
		URI:     "redis://localhost:6379",
		GraphID: "gorag",
	}
}

// DefaultGraphStore creates a FalkorDB GraphStore using default connection settings.
func DefaultGraphStore(ctx context.Context, opts ...Option) (store.GraphStore, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return NewGraphStore(ctx, options.URI, options.GraphID)
}

// NewGraphStore creates a new FalkorDB based graph store.
func NewGraphStore(ctx context.Context, uri string, graphID string) (store.GraphStore, error) {
	client, err := falkordb.NewClient(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to falkordb: %w", err)
	}

	graph := client.SelectGraph(graphID)

	return &falkorGraphStore{
		client: client,
		graph:  graph,
	}, nil
}

func (s *falkorGraphStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	// For production, we would use a batch Cypher query with UNWIND.
	// FalkorDB handles repeated queries well, but UNWIND is safer for large datasets.
	for _, node := range nodes {
		query := fmt.Sprintf("MERGE (n:%s {id: $id}) SET n += $props", node.Type)
		params := map[string]interface{}{
			"id":    node.ID,
			"props": node.Properties,
		}
		if _, err := s.graph.Query(query, params); err != nil {
			return fmt.Errorf("failed to upsert node %s: %w", node.ID, err)
		}
	}
	return nil
}

func (s *falkorGraphStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	for _, edge := range edges {
		query := fmt.Sprintf(
			"MATCH (s {id: $source}), (t {id: $target}) " +
			"MERGE (s)-[r:%s {id: $id}]->(t) " +
			"SET r += $props", edge.Type)
		
		params := map[string]interface{}{
			"source": edge.Source,
			"target": edge.Target,
			"id":     edge.ID,
			"props":  edge.Properties,
		}
		
		if _, err := s.graph.Query(query, params); err != nil {
			return fmt.Errorf("failed to upsert edge %s: %w", edge.ID, err)
		}
	}
	return nil
}

func (s *falkorGraphStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	query := "MATCH (n {id: $id}) RETURN n"
	params := map[string]interface{}{"id": id}
	
	res, err := s.graph.Query(query, params)
	if err != nil {
		return nil, err
	}

	if !res.Next() {
		return nil, nil
	}

	record := res.Record()
	n, ok := record.GetByIndex(0).(*falkordb.Node)
	if !ok {
		return nil, fmt.Errorf("unexpected result type for node")
	}

	node := &core.Node{
		ID:         id,
		Type:       n.Labels[0], 
		Properties: make(map[string]any),
	}

	for k, v := range n.Properties {
		node.Properties[k] = v
	}

	return node, nil
}

func (s *falkorGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	if depth <= 0 {
		depth = 1
	}
	// FIXED: Respect the depth parameter using variable length paths.
	query := fmt.Sprintf("MATCH (n {id: $id})-[r*1..%d]-(m) RETURN DISTINCT m, r LIMIT $limit", depth)
	params := map[string]interface{}{
		"id":    nodeID,
		"limit": limit,
	}

	res, err := s.graph.Query(query, params)
	if err != nil {
		return nil, nil, err
	}

	var nodes []*core.Node
	var edges []*core.Edge
	visitedNodes := make(map[string]bool)
	visitedEdges := make(map[string]bool)

	for res.Next() {
		record := res.Record()
		
		// Parse neighbor node
		mNodeAny := record.GetByIndex(0)
		if mNodeAny != nil {
			if mNode, ok := mNodeAny.(*falkordb.Node); ok {
				id, _ := mNode.Properties["id"].(string)
				if id != "" && !visitedNodes[id] {
					visitedNodes[id] = true
					node := &core.Node{
						ID:         id,
						Properties: mNode.Properties,
					}
					if len(mNode.Labels) > 0 {
						node.Type = mNode.Labels[0]
					}
					nodes = append(nodes, node)
				}
			}
		}

		// Parse edge (Note: r could be a slice if depth > 1)
		rAny := record.GetByIndex(1)
		if rAny != nil {
			processEdge := func(rel *falkordb.Edge) {
				id, _ := rel.Properties["id"].(string)
				if id != "" && !visitedEdges[id] {
					visitedEdges[id] = true
					edge := &core.Edge{
						ID:         id,
						Type:       rel.Relation,
						// source/target resolution in variable paths is more complex, 
						// but for GraphRAG context building, having the nodes and edges is usually enough.
						Properties: rel.Properties,
					}
					edges = append(edges, edge)
				}
			}

			switch v := rAny.(type) {
			case *falkordb.Edge:
				processEdge(v)
			case []interface{}:
				for _, e := range v {
					if rel, ok := e.(*falkordb.Edge); ok {
						processEdge(rel)
					}
				}
			}
		}
	}

	return nodes, edges, nil
}

func (s *falkorGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	res, err := s.graph.Query(query, params)
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for res.Next() {
		record := res.Record()
		row := make(map[string]any)
		keys := res.Columns()
		
		for i, key := range keys {
			val := record.GetByIndex(i)
			switch v := val.(type) {
			case *falkordb.Node:
				row[key] = map[string]any{
					"labels":     v.Labels,
					"properties": v.Properties,
				}
			case *falkordb.Edge:
				row[key] = map[string]any{
					"type":       v.Relation,
					"properties": v.Properties,
				}
			default:
				row[key] = v
			}
		}
		results = append(results, row)
	}

	return results, nil
}

func (s *falkorGraphStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	query := "MATCH (c:Community {level: $level}) RETURN c.id as id, c.summary as summary, c.nodes as nodes"
	params := map[string]interface{}{"level": level}
	
	return s.Query(ctx, query, params)
}

func (s *falkorGraphStore) Close(ctx context.Context) error {
	return nil
}
