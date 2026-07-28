package gograph

import (
	"context"
	"fmt"
	"os"
	"strings"

	api "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gograph/pkg/graph"
	"github.com/DotNetAge/gorag/v2/core"
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

	labels, _ := data["labels"].([]string)
	name := getStringProp(props, "name")
	sourceChunkIDs := getStringSliceProp(props, "source_chunk_ids")
	sourceDocIDs := getStringSliceProp(props, "source_doc_ids")

	delete(props, "ID")
	delete(props, "name")
	delete(props, "source_chunk_ids")
	delete(props, "source_doc_ids")

	return &core.Node{
		ID:             nodeID,
		Labels:         labels,
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
		Labels:         node.Labels,
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
	dbPath string          // open-close 模式
	db     *api.DB         // WrapGraphStore 模式（非空时优先使用）
	gs     *api.GraphStore // WrapGraphStore 模式（非空时优先使用）
}

// withStore 统一管理数据库连接生命周期。
// WrapGraphStore 模式（s.db != nil）时直接使用已有连接，否则打开新连接。
func (s *gographStore) withStore(fn func(db *api.DB, gs *api.GraphStore) error) error {
	if s.db != nil {
		return fn(s.db, s.gs)
	}
	db, err := api.Open(s.dbPath)
	if err != nil {
		return fmt.Errorf("gograph: 打开数据库失败: %w", err)
	}
	defer db.Close()
	gs := api.NewGraphStore(db)
	return fn(db, gs)
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
	if path == "" {
		return nil, fmt.Errorf("gograph.NewGraphStore: 数据库路径不能为空")
	}
	s := &gographStore{dbPath: path}
	return s, s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		return nil
	})
}

// WrapGraphStore 将已存在的 gograph 数据库包装为 core.GraphStore 接口。
// 适用于外部已有 graphDB 实例的场景（如 daemon 共享同一个图数据库给 GraphIndexer）。
// 调用方负责 db 和 gs 的生命周期管理（Close）。
func WrapGraphStore(db *api.DB, gs *api.GraphStore) core.GraphStore {
	return &gographStore{
		db: db,
		gs: gs,
	}
}

// UpsertNodes inserts or updates nodes in the graph store.
func (s *gographStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	nodeDataList := make([]*api.NodeData, 0, len(nodes))
	for _, node := range nodes {
		props := make(map[string]interface{}, len(node.Properties)+5)
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

		nodeLabels := node.Labels
		if nodeLabels == nil {
			nodeLabels = []string{}
		}

		nodeDataList = append(nodeDataList, &api.NodeData{
			ID:         node.ID,
			Labels:     nodeLabels,
			Properties: props,
		})
	}

	return s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		return gs.UpsertNodes(nodeDataList)
	})
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

	return s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		return gs.UpsertEdges(edgeDataList)
	})
}

// GetNode retrieves a node by ID.
func (s *gographStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	var node *core.Node
	err := s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		n, err := gs.GetNode(id)
		if err != nil {
			if err == api.ErrNodeNotFound {
				return nil
			}
			return fmt.Errorf("failed to get node: %w", err)
		}
		node = convertNode(n)
		return nil
	})
	return node, err
}

// GetNeighbors retrieves the neighbors of a node.
func (s *gographStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	var nodes []*core.Node
	var edges []*core.Edge
	err := s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		results, err := gs.GetNeighbors(nodeID, depth, limit)
		if err != nil {
			return fmt.Errorf("failed to get neighbors: %w", err)
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

		nodes = make([]*core.Node, 0, len(nodeMap))
		for _, n := range nodeMap {
			nodes = append(nodes, n)
		}
		edges = make([]*core.Edge, 0, len(edgeMap))
		for _, e := range edgeMap {
			edges = append(edges, e)
		}

		return nil
	})
	return nodes, edges, err
}

// Query executes a query on the graph store.
func (s *gographStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	var results []map[string]any
	err := s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		fmt.Fprintf(os.Stderr, "[DEBUG] gographStore.Query query=%q params=%+v\n", query, params)
		rows, err := db.Query(ctx, query, params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] gographStore.Query ERROR: %v\n", err)
			return fmt.Errorf("failed to execute query: %w", err)
		}
		defer rows.Close()

		columns := rows.Columns()
		fmt.Fprintf(os.Stderr, "[DEBUG] gographStore.Query columns=%v\n", columns)

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
				fmt.Fprintf(os.Stderr, "[DEBUG] gographStore.Query scan ERROR: %v\n", err)
				return fmt.Errorf("failed to scan row: %w", err)
			}

			for i, col := range columns {
				if vp, ok := vals[i].(*interface{}); ok && *vp != nil {
					fmt.Fprintf(os.Stderr, "[DEBUG] gographStore.Query col=%s val_type=%T val=%+v\n", col, *vp, *vp)
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

		fmt.Fprintf(os.Stderr, "[DEBUG] gographStore.Query results count=%d\n", len(results))
		return nil
	})
	return results, err
}

// DeleteNode deletes a node and its edges.
func (s *gographStore) DeleteNode(ctx context.Context, id string) error {
	return s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		_, err := db.Exec(ctx, "MATCH (n {ID: $id}) DETACH DELETE n", map[string]any{"id": id})
		if err != nil {
			return fmt.Errorf("delete node %s: %w", id, err)
		}
		return nil
	})
}

// DeleteEdge deletes an edge by ID.
func (s *gographStore) DeleteEdge(ctx context.Context, id string) error {
	return s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		_, err := db.Exec(ctx, "MATCH ()-[r {ID: $id}]-() DELETE r", map[string]any{"id": id})
		if err != nil {
			return fmt.Errorf("delete edge %s: %w", id, err)
		}
		return nil
	})
}

// Clear removes all nodes and edges from the graph store.
func (s *gographStore) Clear(ctx context.Context) error {
	return s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		_, err := db.Exec(ctx, "MATCH (n) DETACH DELETE n", nil)
		if err != nil {
			return fmt.Errorf("clear graph: %w", err)
		}
		return nil
	})
}

// Close closes the graph store.
// 在 open-close 模式下（dbPath 模式）无需显式关闭，连接已由 withStore 管理。
// 在 WrapGraphStore 模式下关闭底层连接。
func (s *gographStore) Close(ctx context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetByChunkIDs 通过 ChunkID 反查引用该 Chunk 的实体 Node 及其关联 Edge。
// 一次调用同时返回 Nodes 与 Edges，用于语义检索命中 Chunk 后扩展到关系网络（双线结构双向关联）。
func (s *gographStore) GetByChunkIDs(ctx context.Context, chunkIDs []string) ([]*core.Node, []*core.Edge, error) {
	if len(chunkIDs) == 0 {
		return nil, nil, nil
	}

	// 构建 chunkID 的 OR 查询参数（Nodes 与 Edges 共用）
	whereParts := make([]string, len(chunkIDs))
	params := make(map[string]any, len(chunkIDs))
	for i, cid := range chunkIDs {
		paramName := fmt.Sprintf("cid%d", i)
		whereParts[i] = fmt.Sprintf("$%s IN n.source_chunk_ids", paramName)
		params[paramName] = cid
	}
	where := fmt.Sprintf("WHERE %s", strings.Join(whereParts, " OR "))

	var nodes []*core.Node
	var edges []*core.Edge
	err := s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		// 1. 查询 Nodes
		nodeRows, err := db.Query(ctx, fmt.Sprintf("MATCH (n) %s RETURN n", where), params)
		if err != nil {
			return fmt.Errorf("failed to query nodes by chunk IDs: %w", err)
		}
		defer nodeRows.Close()

		nodeColumns := nodeRows.Columns()
		for nodeRows.Next() {
			row := make(map[string]any)
			for _, col := range nodeColumns {
				row[col] = nil
			}
			vals := make([]interface{}, len(nodeColumns))
			for i := range vals {
				var v interface{}
				vals[i] = &v
			}
			if err := nodeRows.Scan(vals...); err != nil {
				return fmt.Errorf("failed to scan node row: %w", err)
			}
			for i, col := range nodeColumns {
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
					default:
						row[col] = val
					}
				}
			}
			nodeData, ok := row["n"].(map[string]any)
			if ok {
				nodes = append(nodes, queryResultToNode(nodeData))
			}
		}

		// 2. 查询 Edges（使用相同的 chunkIDs，但匹配 r.source_chunk_ids）
		edgeWhereParts := make([]string, len(chunkIDs))
		edgeParams := make(map[string]any, len(chunkIDs))
		for i, cid := range chunkIDs {
			paramName := fmt.Sprintf("cid%d", i)
			edgeWhereParts[i] = fmt.Sprintf("$%s IN r.source_chunk_ids", paramName)
			edgeParams[paramName] = cid
		}
		edgeWhere := fmt.Sprintf("WHERE %s", strings.Join(edgeWhereParts, " OR "))

		edgeRows, err := db.Query(ctx, fmt.Sprintf("MATCH ()-[r]->() %s RETURN r", edgeWhere), edgeParams)
		if err != nil {
			return fmt.Errorf("failed to query edges by chunk IDs: %w", err)
		}
		defer edgeRows.Close()

		edgeColumns := edgeRows.Columns()
		for edgeRows.Next() {
			row := make(map[string]any)
			for _, col := range edgeColumns {
				row[col] = nil
			}
			vals := make([]interface{}, len(edgeColumns))
			for i := range vals {
				var v interface{}
				vals[i] = &v
			}
			if err := edgeRows.Scan(vals...); err != nil {
				return fmt.Errorf("failed to scan edge row: %w", err)
			}
			for i, col := range edgeColumns {
				if vp, ok := vals[i].(*interface{}); ok && *vp != nil {
					switch val := (*vp).(type) {
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
			edgeData, ok := row["r"].(map[string]any)
			if ok {
				edges = append(edges, queryResultToEdge(edgeData))
			}
		}

		return nil
	})
	return nodes, edges, err
}

// GetByLabels 按 Label 查询节点（如查询所有 Label="Region" 的节点）。
// 用于 Indexer.Tree() 基于 Region 节点组装知识树。
func (s *gographStore) GetByLabels(ctx context.Context, labels []string, limit int) ([]*core.Node, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	if limit < 1 {
		limit = 100
	}

	// 构建 Label 的 OR 查询
	whereParts := make([]string, len(labels))
	params := make(map[string]any, len(labels))
	for i, label := range labels {
		paramName := fmt.Sprintf("lbl%d", i)
		whereParts[i] = fmt.Sprintf("$%s IN n.labels", paramName)
		params[paramName] = label
	}
	where := fmt.Sprintf("WHERE %s", strings.Join(whereParts, " OR "))

	var nodes []*core.Node
	err := s.withStore(func(db *api.DB, gs *api.GraphStore) error {
		query := fmt.Sprintf("MATCH (n) %s RETURN n LIMIT %d", where, limit)
		rows, err := db.Query(ctx, query, params)
		if err != nil {
			return fmt.Errorf("failed to query nodes by labels: %w", err)
		}
		defer rows.Close()

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
				return fmt.Errorf("failed to scan row: %w", err)
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
					default:
						row[col] = val
					}
				}
			}
			nodeData, ok := row["n"].(map[string]any)
			if ok {
				nodes = append(nodes, queryResultToNode(nodeData))
			}
		}

		return nil
	})
	return nodes, err
}
