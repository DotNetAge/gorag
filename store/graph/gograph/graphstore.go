package gograph

import (
	"context"
	"fmt"
	"math"

	api "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gograph/pkg/graph"
	"github.com/DotNetAge/gorag/core"
)

// propertyValueToAny converts a graph.PropertyValue to an any type.
//
// Parameters:
//   - pv: The property value to convert
//
// Returns:
//   - any: The converted value
func propertyValueToAny(pv graph.PropertyValue) any {
	if pv.String != nil {
		return *pv.String
	}
	if pv.Int != nil {
		if *pv.Int >= math.MinInt && *pv.Int <= math.MaxInt {
			return int(*pv.Int)
		}
		return *pv.Int
	}
	if pv.Float != nil {
		return *pv.Float
	}
	if pv.Bool != nil {
		return *pv.Bool
	}
	return nil
}

// gographStore is an implementation of core.GraphStore using gograph.
type gographStore struct {
	// db is the underlying gograph database
	db *api.DB
	// gs is the gograph graph store
	gs *api.GraphStore
}

// Options contains configuration options for the gograph store.
type Options struct {
	// Path is the path to the gograph database file
	Path string
}

// Option is a function that configures Options.
type Option func(*Options)

// WithPath returns an Option that sets the database path.
//
// Parameters:
//   - path: The path to the database file
//
// Returns:
//   - Option: A configuration function
func WithPath(path string) Option {
	return func(o *Options) {
		o.Path = path
	}
}

// defaultOptions returns the default options for the gograph store.
//
// Returns:
//   - *Options: The default options
func defaultOptions() *Options {
	return &Options{
		Path: "gograph.db",
	}
}

// DefaultGraphStore creates a new gograph store with default options.
//
// Parameters:
//   - opts: Configuration options
//
// Returns:
//   - core.GraphStore: The graph store
//   - error: Any error that occurred
func DefaultGraphStore(opts ...Option) (core.GraphStore, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return NewGraphStore(options.Path)
}

// NewGraphStore creates a new gograph store with the specified path.
//
// Parameters:
//   - path: The path to the database file
//
// Returns:
//   - core.GraphStore: The graph store
//   - error: Any error that occurred
func NewGraphStore(path string) (core.GraphStore, error) {
	db, err := api.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open gograph database: %w", err)
	}
	gs := api.NewGraphStore(db)
	return &gographStore{
		db: db,
		gs: gs,
	}, nil
}

// UpsertNodes inserts or updates nodes in the graph store.
//
// Parameters:
//   - ctx: Context for cancellation
//   - nodes: The nodes to upsert
//
// Returns:
//   - error: Any error that occurred
func (s *gographStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	nodeDataList := make([]*api.NodeData, 0, len(nodes))
	for _, node := range nodes {
		labels := []string{node.Type}
		if node.Type == "" {
			labels = []string{"Entity"}
		}

		props := make(map[string]interface{})
		props["ID"] = node.ID
		for k, v := range node.Properties {
			props[k] = v
		}

		nodeDataList = append(nodeDataList, &api.NodeData{
			ID:         node.ID,
			Labels:     labels,
			Properties: props,
		})
	}

	return s.gs.UpsertNodes(nodeDataList)
}

// UpsertEdges inserts or updates edges in the graph store.
//
// Parameters:
//   - ctx: Context for cancellation
//   - edges: The edges to upsert
//
// Returns:
//   - error: Any error that occurred
func (s *gographStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	if len(edges) == 0 {
		return nil
	}

	edgeDataList := make([]*api.EdgeData, 0, len(edges))
	for _, edge := range edges {
		props := make(map[string]interface{})
		props["ID"] = edge.ID
		for k, v := range edge.Properties {
			props[k] = v
		}

		edgeDataList = append(edgeDataList, &api.EdgeData{
			FromNodeID: edge.Source,
			ToNodeID:   edge.Target,
			Type:       edge.Type,
			Properties: props,
		})
	}

	return s.gs.UpsertEdges(edgeDataList)
}

// GetNode retrieves a node by ID.
//
// Parameters:
//   - ctx: Context for cancellation
//   - id: The node ID
//
// Returns:
//   - *core.Node: The node
//   - error: Any error that occurred
func (s *gographStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	node, err := s.gs.GetNode(id)
	if err != nil {
		if err == api.ErrNodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	props := make(map[string]any)
	for k, v := range node.Properties {
		props[k] = propertyValueToAny(v)
	}

	nodeType := ""
	if len(node.Labels) > 0 {
		nodeType = node.Labels[0]
	}

	userID := id
	if idProp, ok := props["ID"]; ok {
		if idStr, ok := idProp.(string); ok && idStr != "" {
			userID = idStr
		}
	}

	return &core.Node{
		ID:         userID,
		Type:       nodeType,
		Properties: props,
	}, nil
}

// GetNeighbors retrieves the neighbors of a node.
//
// Parameters:
//   - ctx: Context for cancellation
//   - nodeID: The node ID
//   - depth: The depth of the search
//   - limit: The maximum number of results
//
// Returns:
//   - []*core.Node: The neighbor nodes
//   - []*core.Edge: The connecting edges
//   - error: Any error that occurred
func (s *gographStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	results, err := s.gs.GetNeighbors(nodeID, depth, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get neighbors: %w", err)
	}

	nodeMap := make(map[string]*core.Node)
	edgeMap := make(map[string]*core.Edge)

	for _, result := range results {
		if result.Node != nil {
			props := make(map[string]any)
			for k, v := range result.Node.Properties {
				props[k] = propertyValueToAny(v)
			}

			nodeType := ""
			if len(result.Node.Labels) > 0 {
				nodeType = result.Node.Labels[0]
			}

			userID := result.Node.ID
			if idProp, ok := props["ID"]; ok {
				if idStr, ok := idProp.(string); ok && idStr != "" {
					userID = idStr
				}
			}

			nodeMap[userID] = &core.Node{
				ID:         userID,
				Type:       nodeType,
				Properties: props,
			}
		}

		if result.Edge != nil {
			props := make(map[string]any)
			for k, v := range result.Edge.Properties {
				props[k] = propertyValueToAny(v)
			}

			edgeID := result.Edge.ID
			if idProp, ok := props["ID"]; ok {
				if idStr, ok := idProp.(string); ok && idStr != "" {
					edgeID = idStr
				}
			}

			sourceUserID := result.Edge.StartNodeID
			targetUserID := result.Edge.EndNodeID

			if sourceNode, err := s.gs.GetNode(result.Edge.StartNodeID); err == nil {
				if idProp, ok := sourceNode.Properties["ID"]; ok && idProp.String != nil {
					sourceUserID = *idProp.String
				}
			}
			if targetNode, err := s.gs.GetNode(result.Edge.EndNodeID); err == nil {
				if idProp, ok := targetNode.Properties["ID"]; ok && idProp.String != nil {
					targetUserID = *idProp.String
				}
			}

			edgeMap[edgeID] = &core.Edge{
				ID:         edgeID,
				Type:       result.Edge.Type,
				Source:     sourceUserID,
				Target:     targetUserID,
				Properties: props,
			}
		}
	}

	nodes := make([]*core.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	edges := make([]*core.Edge, 0, len(edgeMap))
	for _, e := range edgeMap {
		edges = append(edges, e)
	}

	return nodes, edges, nil
}

// Query executes a query on the graph store.
//
// Parameters:
//   - ctx: Context for cancellation
//   - query: The query string
//   - params: Query parameters
//
// Returns:
//   - []map[string]any: The query results
//   - error: Any error that occurred
func (s *gographStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	columns := rows.Columns()

	for rows.Next() {
		row := make(map[string]any)
		for _, col := range columns {
			row[col] = nil
		}

		vals := make([]interface{}, len(columns))
		for i := range vals {
			var v interface{}
			vals[i] = &v
		}

		if err := rows.Scan(vals...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		for i, col := range columns {
			if vp, ok := vals[i].(*interface{}); ok && *vp != nil {
				switch val := (*vp).(type) {
				case *graph.Node:
					if val != nil {
						nodeProps := make(map[string]any)
						for k, v := range val.Properties {
							nodeProps[k] = propertyValueToAny(v)
						}
						row[col] = map[string]any{
							"id":         val.ID,
							"labels":     val.Labels,
							"properties": nodeProps,
						}
					}
				case graph.Relationship:
					relProps := make(map[string]any)
					for k, v := range val.Properties {
						relProps[k] = propertyValueToAny(v)
					}
					row[col] = map[string]any{
						"id":          val.ID,
						"type":        val.Type,
						"startNodeID": val.StartNodeID,
						"endNodeID":   val.EndNodeID,
						"properties":  relProps,
					}
				default:
					row[col] = val
				}
			}
		}

		results = append(results, row)
	}

	return results, nil
}

// GetCommunitySummaries retrieves community summaries at the specified level.
//
// Parameters:
//   - ctx: Context for cancellation
//   - level: The community level
//
// Returns:
//   - []map[string]any: The community summaries
//   - error: Any error that occurred
func (s *gographStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	query := fmt.Sprintf("MATCH (c:Community) WHERE c.level = %d RETURN c.id as id, c.summary as summary, c.nodes as nodes", level)
	results, err := s.Query(ctx, query, nil)
	if err != nil {
		return []map[string]any{}, nil
	}
	return results, nil
}

// DeleteNode deletes a node and its edges.
//
// Parameters:
//   - ctx: Context for cancellation
//   - id: The node ID
//
// Returns:
//   - error: Any error that occurred
func (s *gographStore) DeleteNode(ctx context.Context, id string) error {
	// Use Cypher DETACH DELETE to remove node and its edges
	_, err := s.db.Exec(ctx, "MATCH (n {ID: $id}) DETACH DELETE n", map[string]any{"id": id})
	return err
}

// DeleteEdge deletes an edge by ID.
//
// Parameters:
//   - ctx: Context for cancellation
//   - id: The edge ID
//
// Returns:
//   - error: Any error that occurred
func (s *gographStore) DeleteEdge(ctx context.Context, id string) error {
	// Use Cypher to delete edge by ID
	_, err := s.db.Exec(ctx, "MATCH ()-[r {ID: $id}]-() DELETE r", map[string]any{"id": id})
	return err
}

// Close closes the graph store.
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns:
//   - error: Any error that occurred
func (s *gographStore) Close(ctx context.Context) error {
	return s.db.Close()
}
