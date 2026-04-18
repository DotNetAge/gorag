package gograph

import (
	"context"
	"fmt"
	"math"
	"strings"

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

// slicePropNames 属性名为这些值时，存储和读取时按 []string 处理
var slicePropNames = map[string]bool{
	"source_chunk_ids": true,
	"source_doc_ids":   true,
}

// propsToAny converts graph properties to map[string]any.
// 对于已知为 []string 类型的属性名，自动将逗号分隔值还原为切片。
func propsToAny(props map[string]graph.PropertyValue) map[string]any {
	result := make(map[string]any, len(props))
	for k, v := range props {
		if slicePropNames[k] && v.String != nil {
			s := strings.TrimSpace(*v.String)
			if s == "" {
				result[k] = []string{}
			} else {
				result[k] = strings.Split(s, ",")
			}
		} else {
			result[k] = propertyValueToAny(v)
		}
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
// Handles both native []string and comma-separated string formats.
// Also cleans Go slice serialization brackets (e.g., "[item1,item2]").
// NOTE: gograph may serialize []string properties with per-element brackets,
// so each element is cleaned individually.
func getStringSliceProp(props map[string]any, key string) []string {
	if v, ok := props[key]; ok {
		if slice, ok := v.([]string); ok {
			result := make([]string, 0, len(slice))
			for _, s := range slice {
				s = strings.TrimSpace(s)
				s = strings.TrimPrefix(s, "[")
				s = strings.TrimSuffix(s, "]")
				if s != "" {
					result = append(result, s)
				}
			}
			return result
		}
		if s, ok := v.(string); ok && s != "" {
			s = strings.TrimSpace(s)
			s = strings.TrimPrefix(s, "[")
			s = strings.TrimSuffix(s, "]")
			if s == "" {
				return nil
			}
			parts := strings.Split(s, ",")
			result := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				p = strings.TrimPrefix(p, "[")
				p = strings.TrimSuffix(p, "]")
				if p != "" {
					result = append(result, p)
				}
			}
			return result
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
		nodes, edges := s.multiHopFromNode(ctx, nodeID, edgeTypes, depth)
		for _, n := range nodes {
			nodeMap[n.ID] = n
		}
		for _, e := range edges {
			edgeMap[e.ID] = e
		}
		// 提前终止检查
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

// maxBFSLimit is the maximum number of nodes BFS will visit before stopping.
// Prevents unbounded traversal on large graphs.
const maxBFSLimit = 200

// multiHopFromNode traverses from a single node using BFS, with optional edge type filtering.
func (s *gographStore) multiHopFromNode(ctx context.Context, nodeID string, edgeTypes []string, maxDepth int) ([]*core.Node, []*core.Edge) {
	nodeMap := make(map[string]*core.Node)
	edgeMap := make(map[string]*core.Edge)

	// BFS
	currentLevel := []string{nodeID}
	visitedNodes := make(map[string]bool)
	visitedNodes[nodeID] = true

	for depth := 0; depth < maxDepth; depth++ {
		if len(currentLevel) == 0 {
			break
		}
		// 达到节点数上限时停止扩展
		if len(nodeMap) >= maxBFSLimit {
			break
		}

		nextLevel := make([]string, 0)

		for _, startID := range currentLevel {
			var results []*api.NeighborResult
			var err error

			if len(edgeTypes) > 0 {
				// 按边类型过滤：使用 Cypher 查询指定类型的邻居
				typeClauses := make([]string, len(edgeTypes))
				for i, t := range edgeTypes {
					// 转义单引号防止 Cypher 注入
					escaped := strings.ReplaceAll(t, "'", "''")
					typeClauses[i] = fmt.Sprintf("'%s'", escaped)
				}
				typeFilter := strings.Join(typeClauses, ", ")
				cypher := fmt.Sprintf(
					`MATCH (n {ID: $id})-[r]-(m) WHERE r.type IN [%s] RETURN n, r, m`,
					typeFilter,
				)
				rows, qErr := s.Query(ctx, cypher, map[string]any{"id": startID})
				if qErr == nil {
					results = s.rowsToNeighborResults(rows)
				}
			} else {
				results, err = s.gs.GetNeighbors(startID, 1, 50)
			}

			if err != nil {
				continue
			}

			for _, result := range results {
				if result.Node != nil {
					node := convertNode(result.Node)
					if !visitedNodes[node.ID] {
						visitedNodes[node.ID] = true
						nodeMap[node.ID] = node
						nextLevel = append(nextLevel, node.ID)
					}
				}

				if result.Edge != nil {
					edge := convertEdge(*result.Edge)

					// 将 gograph 内部节点 ID 解析为用户 ID（与 GetNeighbors 保持一致）
					if result.Edge.StartNodeID != "" {
						if sourceNode, e := s.gs.GetNode(result.Edge.StartNodeID); e == nil {
							if idProp, ok := sourceNode.Properties["ID"]; ok && idProp.String != nil {
								edge.Source = *idProp.String
							}
						}
					}
					if result.Edge.EndNodeID != "" {
						if targetNode, e := s.gs.GetNode(result.Edge.EndNodeID); e == nil {
							if idProp, ok := targetNode.Properties["ID"]; ok && idProp.String != nil {
								edge.Target = *idProp.String
							}
						}
					}
					edgeMap[edge.ID] = edge
				}
			}
		}

		currentLevel = nextLevel
	}

	nodes := make([]*core.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}
	edges := make([]*core.Edge, 0, len(edgeMap))
	for _, e := range edgeMap {
		edges = append(edges, e)
	}

	return nodes, edges
}

// rowsToNeighborResults converts Cypher query rows to NeighborResult slices.
func (s *gographStore) rowsToNeighborResults(rows []map[string]any) []*api.NeighborResult {
	results := make([]*api.NeighborResult, 0, len(rows))
	for _, row := range rows {
		nr := &api.NeighborResult{}

		// 提取目标节点 (m)
		if m, ok := row["m"].(map[string]any); ok {
			nr.Node = s.mapRowToNode(m)
		}

		// 提取边 (r)
		if r, ok := row["r"].(map[string]any); ok {
			nr.Edge = s.mapRowToEdge(r)
		}

		results = append(results, nr)
	}
	return results
}

// mapRowToNode converts a query row map to a graph.Node.
func (s *gographStore) mapRowToNode(m map[string]any) *graph.Node {
	node := &graph.Node{}

	if id, ok := m["id"].(string); ok {
		node.ID = id
	}
	if labels, ok := m["labels"].([]string); ok {
		node.Labels = labels
	}
	if props, ok := m["properties"].(map[string]any); ok {
		node.Properties = mapToGraphProperties(props)
	}

	return node
}

// mapRowToEdge converts a query row map to a graph.Relationship.
func (s *gographStore) mapRowToEdge(r map[string]any) *graph.Relationship {
	rel := &graph.Relationship{}

	if id, ok := r["id"].(string); ok {
		rel.ID = id
	}
	if typ, ok := r["type"].(string); ok {
		rel.Type = typ
	}
	if start, ok := r["startNodeID"].(string); ok {
		rel.StartNodeID = start
	}
	if end, ok := r["endNodeID"].(string); ok {
		rel.EndNodeID = end
	}
	if props, ok := r["properties"].(map[string]any); ok {
		rel.Properties = mapToGraphProperties(props)
	}

	return rel
}

// mapToGraphProperties converts map[string]any to graph.Properties (PropertyValue map).
func mapToGraphProperties(props map[string]any) map[string]graph.PropertyValue {
	result := make(map[string]graph.PropertyValue, len(props))
	for k, v := range props {
		pv := graph.PropertyValue{}
		switch val := v.(type) {
		case string:
			pv.String = &val
		case int:
			i := int64(val)
			pv.Int = &i
		case int64:
			pv.Int = &val
		case float64:
			pv.Float = &val
		case bool:
			pv.Bool = &val
		case []string:
			// 将 []string 序列化为 JSON string 存储
			s := strings.Join(val, ",")
			pv.String = &s
		}
		result[k] = pv
	}
	return result
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

	// gograph 的 Cypher 实现中 n.source_chunk_ids 属性直接访问返回 nil，
	// 需要全量扫描节点后在应用层过滤
	results, err := s.Query(ctx, "MATCH (n) RETURN n", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes by chunk IDs: %w", err)
	}

	// 构建目标 chunkID 集合
	targetSet := make(map[string]bool, len(chunkIDs))
	for _, cid := range chunkIDs {
		targetSet[cid] = true
	}

	nodes := make([]*core.Node, 0)
	for _, result := range results {
		nodeData, ok := result["n"].(map[string]any)
		if !ok {
			continue
		}

		props, _ := nodeData["properties"].(map[string]any)
		if props == nil {
			props = make(map[string]any)
		}

		// 提取 source_chunk_ids 并检查是否匹配目标
		nodeChunkIDs := getStringSliceProp(props, "source_chunk_ids")
		matched := false
		for _, cid := range nodeChunkIDs {
			if targetSet[cid] {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		nodeID := getStringProp(props, "ID")
		if nodeID == "" {
			nodeID, _ = nodeData["id"].(string)
		}

		nodeType := ""
		if labels, ok := nodeData["labels"].([]string); ok && len(labels) > 0 {
			nodeType = labels[0]
		}

		name := getStringProp(props, "name")
		sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
		sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

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

	// gograph 的 Cypher 实现中属性直接访问返回 nil，需全量扫描后应用层过滤
	results, err := s.Query(ctx, "MATCH ()-[r]->() RETURN r", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query edges by chunk IDs: %w", err)
	}

	targetSet := make(map[string]bool, len(chunkIDs))
	for _, cid := range chunkIDs {
		targetSet[cid] = true
	}

	edges := make([]*core.Edge, 0)
	for _, result := range results {
		edgeData, ok := result["r"].(map[string]any)
		if !ok {
			continue
		}

		props, _ := edgeData["properties"].(map[string]any)
		if props == nil {
			props = make(map[string]any)
		}

		edgeChunkIDs := getStringSliceProp(props, "source_chunk_ids")
		matched := false
		for _, cid := range edgeChunkIDs {
			if targetSet[cid] {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		edgeID := getStringProp(props, "ID")
		if edgeID == "" {
			edgeID, _ = edgeData["id"].(string)
		}

		edgeType, _ := edgeData["type"].(string)
		predicate := getStringProp(props, "predicate")
		sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
		sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

		source, _ := edgeData["startNodeID"].(string)
		target, _ := edgeData["endNodeID"].(string)

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
