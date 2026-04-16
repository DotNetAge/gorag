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

// propsToAny converts graph properties to map[string]any.
func propsToAny(props map[string]graph.PropertyValue) map[string]any {
	result := make(map[string]any, len(props))
	for k, v := range props {
		result[k] = propertyValueToAny(v)
	}
	return result
}

// getStringProp safely extracts a string property from a map.
func getStringProp(props map[string]any, key string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getStringSliceProp safely extracts a []string property from a map.
func getStringSliceProp(props map[string]any, key string) []string {
	if v, ok := props[key]; ok {
		if slice, ok := v.([]string); ok {
			return slice
		}
	}
	return nil
}

// convertNode converts a graph.Node to a core.Node.
func convertNode(node *graph.Node) *core.Node {
	props := propsToAny(node.Properties)

	nodeType := ""
	if len(node.Labels) > 0 {
		nodeType = node.Labels[0]
	}

	// Extract ID from properties if available
	nodeID := node.ID
	if id := getStringProp(props, "ID"); id != "" {
		nodeID = id
	}

	// Extract new fields
	name := getStringProp(props, "name")
	sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
	sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

	// Remove internal fields from properties
	delete(props, "ID")
	delete(props, "name")
	delete(props, "source_chunk_ids")
	delete(props, "source_doc_ids")

	return &core.Node{
		ID:             nodeID,
		Type:           nodeType,
		Name:           name,
		Properties:     props,
		SourceChunkIDs: sourceChunkIDs,
		SourceDocIDs:   sourceDocIDs,
	}
}

// convertEdge converts a graph.Relationship to a core.Edge.
func convertEdge(rel graph.Relationship) *core.Edge {
	props := propsToAny(rel.Properties)

	// Extract ID from properties if available
	edgeID := rel.ID
	if id := getStringProp(props, "ID"); id != "" {
		edgeID = id
	}

	// Extract new fields
	predicate := getStringProp(props, "predicate")
	sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
	sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

	// Remove internal fields from properties
	delete(props, "ID")
	delete(props, "predicate")
	delete(props, "source_chunk_ids")
	delete(props, "source_doc_ids")

	return &core.Edge{
		ID:             edgeID,
		Type:           rel.Type,
		Source:         rel.StartNodeID,
		Target:         rel.EndNodeID,
		Predicate:      predicate,
		Properties:     props,
		SourceChunkIDs: sourceChunkIDs,
		SourceDocIDs:   sourceDocIDs,
	}
}

// gographStore is an implementation of core.GraphStore using gograph.
type gographStore struct {
	db *api.DB
	gs *api.GraphStore
}

// Options contains configuration options for the gograph store.
type Options struct {
	Path string
}

// Option is a function that configures Options.
type Option func(*Options)

// WithPath returns an Option that sets the database path.
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

// DefaultGraphStore creates a new gograph store with default options.
func DefaultGraphStore(opts ...Option) (core.GraphStore, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return NewGraphStore(options.Path)
}

// NewGraphStore creates a new gograph store with the specified path.
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
func (s *gographStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	nodeDataList := make([]*api.NodeData, 0, len(nodes))
	for _, node := range nodes {
		labels := []string{node.Type}
		if node.Type == "" {
			labels = []string{"Node"}
		}

		props := make(map[string]interface{}, len(node.Properties)+4)
		props["ID"] = node.ID
		props["name"] = node.Name
		for k, v := range node.Properties {
			props[k] = v
		}
		if len(node.SourceChunkIDs) > 0 {
			props["source_chunk_ids"] = node.SourceChunkIDs
		}
		if len(node.SourceDocIDs) > 0 {
			props["source_doc_ids"] = node.SourceDocIDs
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
func (s *gographStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	if len(edges) == 0 {
		return nil
	}

	edgeDataList := make([]*api.EdgeData, 0, len(edges))
	for _, edge := range edges {
		props := make(map[string]interface{}, len(edge.Properties)+4)
		props["ID"] = edge.ID
		props["predicate"] = edge.Predicate
		for k, v := range edge.Properties {
			props[k] = v
		}
		if len(edge.SourceChunkIDs) > 0 {
			props["source_chunk_ids"] = edge.SourceChunkIDs
		}
		if len(edge.SourceDocIDs) > 0 {
			props["source_doc_ids"] = edge.SourceDocIDs
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
func (s *gographStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	node, err := s.gs.GetNode(id)
	if err != nil {
		if err == api.ErrNodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	return convertNode(node), nil
}

// GetNeighbors retrieves the neighbors of a node.
func (s *gographStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	results, err := s.gs.GetNeighbors(nodeID, depth, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get neighbors: %w", err)
	}

	nodeMap := make(map[string]*core.Node)
	edgeMap := make(map[string]*core.Edge)

	for _, result := range results {
		if result.Node != nil {
			node := convertNode(result.Node)
			nodeMap[node.ID] = node
		}

		if result.Edge != nil {
			edge := convertEdge(*result.Edge)
			
			// Resolve source and target user IDs
			if sourceNode, err := s.gs.GetNode(result.Edge.StartNodeID); err == nil {
				if idProp, ok := sourceNode.Properties["ID"]; ok && idProp.String != nil {
					edge.Source = *idProp.String
				}
			}
			if targetNode, err := s.gs.GetNode(result.Edge.EndNodeID); err == nil {
				if idProp, ok := targetNode.Properties["ID"]; ok && idProp.String != nil {
					edge.Target = *idProp.String
				}
			}

			edgeMap[edge.ID] = edge
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
						row[col] = map[string]any{
							"id":         val.ID,
							"labels":     val.Labels,
							"properties": propsToAny(val.Properties),
						}
					}
				case graph.Relationship:
					row[col] = map[string]any{
						"id":          val.ID,
						"type":        val.Type,
						"startNodeID": val.StartNodeID,
						"endNodeID":   val.EndNodeID,
						"properties":  propsToAny(val.Properties),
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
func (s *gographStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	query := fmt.Sprintf("MATCH (c:Community) WHERE c.level = %d RETURN c.id as id, c.summary as summary, c.nodes as nodes", level)
	results, err := s.Query(ctx, query, nil)
	if err != nil {
		return []map[string]any{}, nil
	}
	return results, nil
}

// DeleteNode deletes a node and its edges.
func (s *gographStore) DeleteNode(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "MATCH (n {ID: $id}) DETACH DELETE n", map[string]any{"id": id})
	return err
}

// DeleteEdge deletes an edge by ID.
func (s *gographStore) DeleteEdge(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "MATCH ()-[r {ID: $id}]-() DELETE r", map[string]any{"id": id})
	return err
}

// Close closes the graph store.
func (s *gographStore) Close(ctx context.Context) error {
	return s.db.Close()
}

// GetNodesByChunkIDs retrieves all nodes associated with the given chunk IDs.
func (s *gographStore) GetNodesByChunkIDs(ctx context.Context, chunkIDs []string) ([]*core.Node, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	query := `
		MATCH (n)
		WHERE ANY(chunkID IN $chunkIDs WHERE chunkID IN n.source_chunk_ids)
		RETURN n
	`

	results, err := s.Query(ctx, query, map[string]any{"chunkIDs": chunkIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes by chunk IDs: %w", err)
	}

	nodes := make([]*core.Node, 0, len(results))
	for _, result := range results {
		nodeData, ok := result["n"].(map[string]any)
		if !ok {
			continue
		}

		props, _ := nodeData["properties"].(map[string]any)
		if props == nil {
			props = make(map[string]any)
		}

		// Extract node ID
		nodeID := getStringProp(props, "ID")
		if nodeID == "" {
			nodeID, _ = nodeData["id"].(string)
		}

		// Extract node type from labels
		nodeType := ""
		if labels, ok := nodeData["labels"].([]string); ok && len(labels) > 0 {
			nodeType = labels[0]
		}

		// Extract new fields
		name := getStringProp(props, "name")
		sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
		sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

		// Remove internal fields
		delete(props, "ID")
		delete(props, "name")
		delete(props, "source_chunk_ids")
		delete(props, "source_doc_ids")

		nodes = append(nodes, &core.Node{
			ID:             nodeID,
			Type:           nodeType,
			Name:           name,
			Properties:     props,
			SourceChunkIDs: sourceChunkIDs,
			SourceDocIDs:   sourceDocIDs,
		})
	}

	return nodes, nil
}

// GetEdgesByChunkIDs retrieves all edges associated with the given chunk IDs.
func (s *gographStore) GetEdgesByChunkIDs(ctx context.Context, chunkIDs []string) ([]*core.Edge, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	query := `
		MATCH ()-[r]->()
		WHERE ANY(chunkID IN $chunkIDs WHERE chunkID IN r.source_chunk_ids)
		RETURN r
	`

	results, err := s.Query(ctx, query, map[string]any{"chunkIDs": chunkIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to query edges by chunk IDs: %w", err)
	}

	edges := make([]*core.Edge, 0, len(results))
	for _, result := range results {
		edgeData, ok := result["r"].(map[string]any)
		if !ok {
			continue
		}

		props, _ := edgeData["properties"].(map[string]any)
		if props == nil {
			props = make(map[string]any)
		}

		// Extract edge ID
		edgeID := getStringProp(props, "ID")
		if edgeID == "" {
			edgeID, _ = edgeData["id"].(string)
		}

		// Extract edge type
		edgeType, _ := edgeData["type"].(string)

		// Extract new fields
		predicate := getStringProp(props, "predicate")
		sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
		sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

		// Extract source and target node IDs safely
		source, _ := edgeData["startNodeID"].(string)
		target, _ := edgeData["endNodeID"].(string)

		// Remove internal fields
		delete(props, "ID")
		delete(props, "predicate")
		delete(props, "source_chunk_ids")
		delete(props, "source_doc_ids")

		edges = append(edges, &core.Edge{
			ID:             edgeID,
			Type:           edgeType,
			Source:         source,
			Target:         target,
			Predicate:      predicate,
			Properties:     props,
			SourceChunkIDs: sourceChunkIDs,
			SourceDocIDs:   sourceDocIDs,
		})
	}

	return edges, nil
}
