package gograph

import (
	"context"
	"fmt"
	"math"

	api "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gograph/pkg/graph"
	"github.com/DotNetAge/gorag/pkg/core"
)

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

type gographStore struct {
	db *api.DB
	gs *api.GraphStore
}

type Options struct {
	Path string
}

type Option func(*Options)

func WithPath(path string) Option {
	return func(o *Options) {
		o.Path = path
	}
}

func defaultOptions() *Options {
	return &Options{
		Path: "gograph.db",
	}
}

func DefaultGraphStore(opts ...Option) (core.GraphStore, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return NewGraphStore(options.Path)
}

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

func (s *gographStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	query := fmt.Sprintf("MATCH (c:Community) WHERE c.level = %d RETURN c.id as id, c.summary as summary, c.nodes as nodes", level)
	results, err := s.Query(ctx, query, nil)
	if err != nil {
		return []map[string]any{}, nil
	}
	return results, nil
}

func (s *gographStore) DeleteNode(ctx context.Context, id string) error {
	// Use Cypher DETACH DELETE to remove node and its edges
	_, err := s.db.Exec(ctx, "MATCH (n {ID: $id}) DETACH DELETE n", map[string]any{"id": id})
	return err
}

func (s *gographStore) DeleteEdge(ctx context.Context, id string) error {
	// Use Cypher to delete edge by ID
	_, err := s.db.Exec(ctx, "MATCH ()-[r {ID: $id}]-() DELETE r", map[string]any{"id": id})
	return err
}

func (s *gographStore) Close(ctx context.Context) error {
	return s.db.Close()
}
