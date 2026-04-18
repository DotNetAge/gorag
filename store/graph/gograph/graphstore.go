package gograph

import (
	"context"
	"fmt"
	"strings"

	api "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gograph/pkg/graph"
	"github.com/DotNetAge/gorag/core"
)

// propsToAny converts graph properties to map[string]any.
func propsToAny(props map[string]graph.PropertyValue) map[string]any {
	result := make(map[string]any, len(props))
	for k, v := range props {
		result[k] = v.InterfaceValue()
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

// getStringSliceProp safely extracts a []string from a map.
// Handles native []string and []interface{} (from gograph List property).
func getStringSliceProp(props map[string]any, key string) []string {
	v, ok := props[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// queryResultToNode converts a Query-returned node map to a core.Node.
func queryResultToNode(data map[string]any) *core.Node {
	props, _ := data["properties"].(map[string]any)
	if props == nil {
		props = make(map[string]any)
	}

	nodeID := getStringProp(props, "ID")
	if nodeID == "" {
		nodeID, _ = data["id"].(string)
	}

	nodeType := ""
	if labels, ok := data["labels"].([]string); ok && len(labels) > 0 {
		nodeType = labels[0]
	}

	name := getStringProp(props, "name")
	sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
	sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

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

// queryResultToEdge converts a Query-returned edge map to a core.Edge.
func queryResultToEdge(data map[string]any) *core.Edge {
	props, _ := data["properties"].(map[string]any)
	if props == nil {
		props = make(map[string]any)
	}

	edgeID := getStringProp(props, "ID")
	if edgeID == "" {
		edgeID, _ = data["id"].(string)
	}

	edgeType, _ := data["type"].(string)
	predicate := getStringProp(props, "predicate")
	sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
	sourceDocIDs := getStringSliceProp(props, "source_doc_ids")
	source, _ := data["startNodeID"].(string)
	target, _ := data["endNodeID"].(string)

	delete(props, "ID")
	delete(props, "predicate")
	delete(props, "source_chunk_ids")
	delete(props, "source_doc_ids")

	return &core.Edge{
		ID:             edgeID,
		Type:           edgeType,
		Source:         source,
		Target:         target,
		Predicate:      predicate,
		Properties:     props,
		SourceChunkIDs: sourceChunkIDs,
		SourceDocIDs:   sourceDocIDs,
	}
}

// convertNode converts a graph.Node to a core.Node.
func convertNode(node *graph.Node) *core.Node {
	props := propsToAny(node.Properties)

	nodeType := ""
	if len(node.Labels) > 0 {
		nodeType = node.Labels[0]
	}

	nodeID := node.ID
	if id := getStringProp(props, "ID"); id != "" {
		nodeID = id
	}

	name := getStringProp(props, "name")
	sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
	sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

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

	edgeID := rel.ID
	if id := getStringProp(props, "ID"); id != "" {
		edgeID = id
	}

	predicate := getStringProp(props, "predicate")
	sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
	sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

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
	query := fmt.Sprintf("MATCH (c:Community) WHERE c.level = %d RETURN c.id as id, c.summary as summary, c.keywords as keywords, c.nodes as nodes", level)
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

// GetAllEdgeTypes returns all distinct edge types in the graph.
func (s *gographStore) GetAllEdgeTypes(ctx context.Context) ([]string, error) {
	query := `MATCH ()-[r]->() RETURN DISTINCT r.type AS type ORDER BY type`
	results, err := s.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query edge types: %w", err)
	}

	types := make([]string, 0, len(results))
	for _, row := range results {
		if t, ok := row["type"].(string); ok && t != "" {
			types = append(types, t)
		}
	}
	return types, nil
}

// GetMultiHopPaths performs multi-hop traversal from starting nodes.
// If edgeTypes is non-empty, only edges matching those types are traversed.
func (s *gographStore) GetMultiHopPaths(ctx context.Context, nodeIDs []string, edgeTypes []string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	if len(nodeIDs) == 0 {
		return nil, nil, nil
	}
	if depth < 1 {
		depth = 1
	}
	if limit < 1 {
		limit = 10
	}

	nodeMap := make(map[string]*core.Node)
	edgeMap := make(map[string]*core.Edge)

	for _, nodeID := range nodeIDs {
		results, err := s.gs.GetNeighborsByTypes(nodeID, depth, 0, edgeTypes)
		if err != nil {
			continue
		}

		for _, result := range results {
			if result.Node != nil {
				node := convertNode(result.Node)
				nodeMap[node.ID] = node
			}
			if result.Edge != nil {
				edge := convertEdge(*result.Edge)
				edgeMap[edge.ID] = edge
			}
		}

		if len(nodeMap) >= limit {
			break
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

// Close closes the graph store.
func (s *gographStore) Close(ctx context.Context) error {
	return s.db.Close()
}

// GetNodesByChunkIDs retrieves all nodes associated with the given chunk IDs.
func (s *gographStore) GetNodesByChunkIDs(ctx context.Context, chunkIDs []string) ([]*core.Node, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	// Build OR clauses for each chunk ID to match against the list property
	whereParts := make([]string, len(chunkIDs))
	params := make(map[string]any, len(chunkIDs))
	for i, cid := range chunkIDs {
		paramName := fmt.Sprintf("cid%d", i)
		whereParts[i] = fmt.Sprintf("$%s IN n.source_chunk_ids", paramName)
		params[paramName] = cid
	}
	where := fmt.Sprintf("WHERE %s", strings.Join(whereParts, " OR "))

	results, err := s.Query(ctx, fmt.Sprintf("MATCH (n) %s RETURN n", where), params)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes by chunk IDs: %w", err)
	}

	nodes := make([]*core.Node, 0, len(results))
	for _, result := range results {
		nodeData, ok := result["n"].(map[string]any)
		if !ok {
			continue
		}
		nodes = append(nodes, queryResultToNode(nodeData))
	}

	return nodes, nil
}

// GetEdgesByChunkIDs retrieves all edges associated with the given chunk IDs.
func (s *gographStore) GetEdgesByChunkIDs(ctx context.Context, chunkIDs []string) ([]*core.Edge, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	whereParts := make([]string, len(chunkIDs))
	params := make(map[string]any, len(chunkIDs))
	for i, cid := range chunkIDs {
		paramName := fmt.Sprintf("cid%d", i)
		whereParts[i] = fmt.Sprintf("$%s IN r.source_chunk_ids", paramName)
		params[paramName] = cid
	}
	where := fmt.Sprintf("WHERE %s", strings.Join(whereParts, " OR "))

	results, err := s.Query(ctx, fmt.Sprintf("MATCH ()-[r]->() %s RETURN r", where), params)
	if err != nil {
		return nil, fmt.Errorf("failed to query edges by chunk IDs: %w", err)
	}

	edges := make([]*core.Edge, 0, len(results))
	for _, result := range results {
		edgeData, ok := result["r"].(map[string]any)
		if !ok {
			continue
		}
		edges = append(edges, queryResultToEdge(edgeData))
	}

	return edges, nil
}


