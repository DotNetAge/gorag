package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	_ "modernc.org/sqlite"
)

type sqliteGraphStore struct {
	db *sql.DB
}

// NewGraphStore creates a new SQLite based graph store.
// DefaultGraphStore creates a SQLite GraphStore using a default local file "gorag_graph.db".
func DefaultGraphStore() (store.GraphStore, error) {
	return NewGraphStore("gorag_graph.db")
}

func NewGraphStore(path string) (store.GraphStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// Create tables for property graph
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			type TEXT,
			properties TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS edges (
			id TEXT PRIMARY KEY,
			type TEXT,
			source TEXT,
			target TEXT,
			properties TEXT,
			FOREIGN KEY(source) REFERENCES nodes(id),
			FOREIGN KEY(target) REFERENCES nodes(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target)`,
		// For GraphRAG Community Summaries (Optional/Advanced)
		`CREATE TABLE IF NOT EXISTS communities (
			id TEXT PRIMARY KEY,
			level INTEGER,
			summary TEXT,
			nodes TEXT
		)`,
	}

	for _, schema := range schemas {
		if _, err := db.Exec(schema); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to create table: %w", err)
		}
	}

	return &sqliteGraphStore{db: db}, nil
}

func (s *sqliteGraphStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT OR REPLACE INTO nodes (id, type, properties) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, node := range nodes {
		props, err := json.Marshal(node.Properties)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx, node.ID, node.Type, string(props)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteGraphStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT OR REPLACE INTO edges (id, type, source, target, properties) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, edge := range edges {
		props, err := json.Marshal(edge.Properties)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx, edge.ID, edge.Type, edge.Source, edge.Target, string(props)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteGraphStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	var node core.Node
	var propsStr string
	err := s.db.QueryRowContext(ctx, "SELECT id, type, properties FROM nodes WHERE id = ?", id).
		Scan(&node.ID, &node.Type, &propsStr)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(propsStr), &node.Properties); err != nil {
		return nil, err
	}

	return &node, nil
}

func (s *sqliteGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	if depth <= 0 {
		depth = 1
	}
	if limit <= 0 {
		limit = 100
	}

	// Use Recursive CTE to find neighbors up to depth
	query := `
	WITH RECURSIVE
	  neighbor_nodes(id, current_depth) AS (
		SELECT ? AS id, 0 AS current_depth
		UNION
		SELECT CASE 
				 WHEN e.source = n.id THEN e.target 
				 ELSE e.source 
			   END,
			   n.current_depth + 1
		FROM neighbor_nodes n
		JOIN edges e ON e.source = n.id OR e.target = n.id
		WHERE n.current_depth < ?
	  )
	SELECT DISTINCT id FROM neighbor_nodes WHERE id != ? LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, nodeID, depth, nodeID, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var nodeIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		nodeIDs = append(nodeIDs, id)
	}

	if len(nodeIDs) == 0 {
		return nil, nil, nil
	}

	// Fetch actual nodes
	nodes := make([]*core.Node, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		node, err := s.GetNode(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}

	// Fetch edges connecting these nodes (including the starting node)
	allIDs := append(nodeIDs, nodeID)
	// Create placeholders for IN clause
	placeholders := ""
	args := make([]any, len(allIDs))
	for i, id := range allIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = id
	}

	edgeQuery := fmt.Sprintf("SELECT id, type, source, target, properties FROM edges WHERE source IN (%s) AND target IN (%s) LIMIT ?", placeholders, placeholders)
	allArgs := make([]any, 0, len(allIDs)*2+1)
	for _, id := range allIDs {
		allArgs = append(allArgs, id)
	}
	for _, id := range allIDs {
		allArgs = append(allArgs, id)
	}
	allArgs = append(allArgs, limit)

	edgeRows, err := s.db.QueryContext(ctx, edgeQuery, allArgs...)
	if err != nil {
		return nodes, nil, err
	}
	defer edgeRows.Close()

	var edges []*core.Edge
	for edgeRows.Next() {
		var edge core.Edge
		var propsStr string
		if err := edgeRows.Scan(&edge.ID, &edge.Type, &edge.Source, &edge.Target, &propsStr); err != nil {
			return nodes, nil, err
		}
		if err := json.Unmarshal([]byte(propsStr), &edge.Properties); err != nil {
			return nodes, nil, err
		}
		edges = append(edges, &edge)
	}

	return nodes, edges, nil
}

func (s *sqliteGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	// For SQLite, we fallback to raw SQL queries as it doesn't support Cypher.
	// In the future, we could add a Cypher-to-SQL transpiler.
	
	// Note: SQLite driver uses ? or :name placeholders. 
	// This simple implementation assumes query uses ? and params are provided in order if needed, 
	// but mapping map to ordered slice is tricky. 
	// For now, let's just support simple SQL.
	
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		columns := make([]any, len(cols))
		columnPointers := make([]any, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		m := make(map[string]any)
		for i, colName := range cols {
			val := columns[i]
			b, ok := val.([]byte)
			if ok {
				m[colName] = string(b)
			} else {
				m[colName] = val
			}
		}
		results = append(results, m)
	}

	return results, nil
}

func (s *sqliteGraphStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, level, summary, nodes FROM communities WHERE level = ?", level)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id string
		var lvl int
		var summary string
		var nodesStr string
		if err := rows.Scan(&id, &lvl, &summary, &nodesStr); err != nil {
			return nil, err
		}
		
		var nodes []string
		_ = json.Unmarshal([]byte(nodesStr), &nodes)

		results = append(results, map[string]any{
			"id":      id,
			"level":   lvl,
			"summary": summary,
			"nodes":   nodes,
		})
	}

	return results, nil
}

func (s *sqliteGraphStore) Close(ctx context.Context) error {
	return s.db.Close()
}
