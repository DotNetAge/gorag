// +build nebulagraph

package graphstore

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	"github.com/vesoft-inc/nebula-go/v3/nebula/graph"
	nebulaPool "github.com/vesoft-inc/nebula-go/v3/pool"
)

// NebulaGraphStore implements GraphStore using NebulaGraph
type NebulaGraphStore struct {
	pool      *nebulaPool.ConnectionPool
	spaceName string
}

// NewNebulaGraphStore creates a new NebulaGraphStore
func NewNebulaGraphStore() *NebulaGraphStore {
	return &NebulaGraphStore{}
}

// Initialize initializes the NebulaGraph connection
func (n *NebulaGraphStore) Initialize(options map[string]interface{}) error {
	// Extract connection details from options
	addresses, ok := options["addresses"].([]string)
	if !ok {
		return fmt.Errorf("addresses is required")
	}

	username, ok := options["username"].(string)
	if !ok {
		return fmt.Errorf("username is required")
	}

	password, ok := options["password"].(string)
	if !ok {
		return fmt.Errorf("password is required")
	}

	n.spaceName, ok = options["space"].(string)
	if !ok {
		return fmt.Errorf("space is required")
	}

	// Create connection pool
	config := nebulaPool.GetDefaultConfig()
	pool, err := nebulaPool.NewConnectionPool(addresses, config, username, password)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	n.pool = pool

	// Create space if it doesn't exist
	if err := n.createSpaceIfNotExists(); err != nil {
		return fmt.Errorf("failed to create space: %w", err)
	}

	// Create tag and edge types
	if err := n.createSchema(); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// createSpaceIfNotExists creates the space if it doesn't exist
func (n *NebulaGraphStore) createSpaceIfNotExists() error {
	conn, err := n.pool.GetConnection(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	// Check if space exists
	resp, err := conn.Execute(context.Background(), fmt.Sprintf("SHOW SPACES"))
	if err != nil {
		return err
	}

	hasSpace := false
	for _, row := range resp.GetData().GetRows() {
		spaceName := row.GetValues()[0].GetSVal()
		if spaceName == n.spaceName {
			hasSpace = true
			break
		}
	}

	// Create space if it doesn't exist
	if !hasSpace {
		resp, err := conn.Execute(context.Background(), fmt.Sprintf(
			"CREATE SPACE %s (partition_num=10, replica_factor=1)",
			n.spaceName,
		))
		if err != nil {
			return err
		}
		if !resp.IsSucceed() {
			return fmt.Errorf("failed to create space: %s", resp.GetErrorMsg())
		}
	}

	// Use the space
	resp, err = conn.Execute(context.Background(), fmt.Sprintf("USE %s", n.spaceName))
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to use space: %s", resp.GetErrorMsg())
	}

	return nil
}

// createSchema creates the necessary tag and edge types
func (n *NebulaGraphStore) createSchema() error {
	conn, err := n.pool.GetConnection(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	// Use the space
	resp, err := conn.Execute(context.Background(), fmt.Sprintf("USE %s", n.spaceName))
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to use space: %s", resp.GetErrorMsg())
	}

	// Create tag for nodes
	resp, err = conn.Execute(context.Background(), `
		CREATE TAG IF NOT EXISTS node (
			id string,
			type string
		)
	`)
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to create tag: %s", resp.GetErrorMsg())
	}

	// Create edge type
	resp, err = conn.Execute(context.Background(), `
		CREATE EDGE IF NOT EXISTS relationship (
			id string,
			type string
		)
	`)
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to create edge type: %s", resp.GetErrorMsg())
	}

	return nil
}

// CreateNode creates a new node in NebulaGraph
func (n *NebulaGraphStore) CreateNode(ctx context.Context, node *abstraction.Node) error {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return err
	}

	// Build the query
	query := fmt.Sprintf(
		"INSERT VERTEX node(id, type) VALUES \"%s\":(\"%s\", \"%s\")",
		node.ID, node.ID, node.Type,
	)

	resp, err := conn.Execute(ctx, query)
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to create node: %s", resp.GetErrorMsg())
	}

	return nil
}

// CreateEdge creates a new edge in NebulaGraph
func (n *NebulaGraphStore) CreateEdge(ctx context.Context, edge *abstraction.Edge) error {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return err
	}

	// Build the query
	query := fmt.Sprintf(
		"INSERT EDGE relationship(id, type) VALUES \"%s\"->\"%s\":(\"%s\", \"%s\")",
		edge.Source, edge.Target, edge.ID, edge.Type,
	)

	resp, err := conn.Execute(ctx, query)
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to create edge: %s", resp.GetErrorMsg())
	}

	return nil
}

// GetNode retrieves a node by its ID
func (n *NebulaGraphStore) GetNode(ctx context.Context, id string) (*abstraction.Node, error) {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return nil, err
	}

	// Build the query
	query := fmt.Sprintf(
		"FETCH PROP ON node \"%s\"",
		id,
	)

	resp, err := conn.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	if !resp.IsSucceed() {
		return nil, fmt.Errorf("failed to get node: %s", resp.GetErrorMsg())
	}

	// Parse the result
	if len(resp.GetData().GetRows()) == 0 {
		return nil, nil
	}

	row := resp.GetData().GetRows()[0]
	values := row.GetValues()

	// Extract properties
	properties := make(map[string]any)
	for i, col := range resp.GetColNames() {
		if col != "id" && col != "type" {
			properties[col] = n.getValue(values[i])
		}
	}

	return &abstraction.Node{
		ID:         id,
		Type:       values[1].GetSVal(),
		Properties: properties,
	}, nil
}

// GetEdge retrieves an edge by its ID
func (n *NebulaGraphStore) GetEdge(ctx context.Context, id string) (*abstraction.Edge, error) {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return nil, err
	}

	// Build the query
	query := fmt.Sprintf(
		"GO FROM ANY OVER relationship WHERE relationship.id == \"%s\" YIELD src(edge), dst(edge), relationship.id, relationship.type",
		id,
	)

	resp, err := conn.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	if !resp.IsSucceed() {
		return nil, fmt.Errorf("failed to get edge: %s", resp.GetErrorMsg())
	}

	// Parse the result
	if len(resp.GetData().GetRows()) == 0 {
		return nil, nil
	}

	row := resp.GetData().GetRows()[0]
	values := row.GetValues()

	// Extract properties
	properties := make(map[string]any)

	return &abstraction.Edge{
		ID:         id,
		Type:       values[3].GetSVal(),
		Source:     values[0].GetSVal(),
		Target:     values[1].GetSVal(),
		Properties: properties,
	}, nil
}

// DeleteNode deletes a node from NebulaGraph
func (n *NebulaGraphStore) DeleteNode(ctx context.Context, id string) error {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return err
	}

	// Build the query
	query := fmt.Sprintf(
		"DELETE VERTEX \"%s\"",
		id,
	)

	resp, err := conn.Execute(ctx, query)
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to delete node: %s", resp.GetErrorMsg())
	}

	return nil
}

// DeleteEdge deletes an edge from NebulaGraph
func (n *NebulaGraphStore) DeleteEdge(ctx context.Context, id string) error {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return err
	}

	// First find the edge
	goQuery := fmt.Sprintf(
		"GO FROM ANY OVER relationship WHERE relationship.id == \"%s\" YIELD src(edge), dst(edge)",
		id,
	)

	resp, err := conn.Execute(ctx, goQuery)
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to find edge: %s", resp.GetErrorMsg())
	}

	if len(resp.GetData().GetRows()) == 0 {
		return nil
	}

	row := resp.GetData().GetRows()[0]
	source := row.GetValues()[0].GetSVal()
	target := row.GetValues()[1].GetSVal()

	// Delete the edge
	deleteQuery := fmt.Sprintf(
		"DELETE EDGE relationship \"%s\"->\"%s\"",
		source, target,
	)

	resp, err = conn.Execute(ctx, deleteQuery)
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to delete edge: %s", resp.GetErrorMsg())
	}

	return nil
}

// Query executes a graph query
func (n *NebulaGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return nil, err
	}

	// Execute the query
	resp, err := conn.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	if !resp.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resp.GetErrorMsg())
	}

	// Parse the result
	var results []map[string]any
	for _, row := range resp.GetData().GetRows() {
		result := make(map[string]any)
		for i, col := range resp.GetColNames() {
			result[col] = n.getValue(row.GetValues()[i])
		}
		results = append(results, result)
	}

	return results, nil
}

// GetNeighbors retrieves the neighbors of a node
func (n *NebulaGraphStore) GetNeighbors(ctx context.Context, nodeID string, limit int) ([]*abstraction.Node, error) {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return nil, err
	}

	// Build the query
	query := fmt.Sprintf(
		"GO FROM \"%s\" OVER relationship YIELD id($$)",
		nodeID,
	)

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	resp, err := conn.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	if !resp.IsSucceed() {
		return nil, fmt.Errorf("failed to get neighbors: %s", resp.GetErrorMsg())
	}

	// Parse the result
	var nodes []*abstraction.Node
	for _, row := range resp.GetData().GetRows() {
		neighborID := row.GetValues()[0].GetSVal()
		// Get the neighbor node
		neighbor, err := n.GetNode(ctx, neighborID)
		if err != nil {
			return nil, err
		}
		if neighbor != nil {
			nodes = append(nodes, neighbor)
		}
	}

	return nodes, nil
}

// GetCommunitySummaries retrieves community summaries
func (n *NebulaGraphStore) GetCommunitySummaries(ctx context.Context, limit int) ([]map[string]any, error) {
	conn, err := n.pool.GetConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Use the space
	if err := n.useSpace(conn, ctx); err != nil {
		return nil, err
	}

	// This is a placeholder implementation
	// In a real implementation, you would use NebulaGraph's community detection algorithms
	query := `
		LOOKUP ON node YIELD id($$), node.type
		| GROUP BY node.type YIELD node.type as community, count(*) as size
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	resp, err := conn.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	if !resp.IsSucceed() {
		return nil, fmt.Errorf("failed to get community summaries: %s", resp.GetErrorMsg())
	}

	// Parse the result
	var results []map[string]any
	for _, row := range resp.GetData().GetRows() {
		result := make(map[string]any)
		for i, col := range resp.GetColNames() {
			result[col] = n.getValue(row.GetValues()[i])
		}
		results = append(results, result)
	}

	return results, nil
}

// UpsertNodes batch updates or inserts nodes
func (n *NebulaGraphStore) UpsertNodes(ctx context.Context, nodes []*abstraction.Node) error {
	for _, node := range nodes {
		// Check if node exists
		existingNode, err := n.GetNode(ctx, node.ID)
		if err != nil {
			return err
		}

		if existingNode == nil {
			// Create new node
			if err := n.CreateNode(ctx, node); err != nil {
				return err
			}
		} else {
			// Update existing node
			conn, err := n.pool.GetConnection(ctx)
			if err != nil {
				return err
			}
			defer conn.Close()

			// Use the space
			if err := n.useSpace(conn, ctx); err != nil {
				return err
			}

			// Build the query
			query := fmt.Sprintf(
				"UPDATE VERTEX \"%s\" SET node.type = \"%s\"",
				node.ID, node.Type,
			)

			resp, err := conn.Execute(ctx, query)
			if err != nil {
				return err
			}
			if !resp.IsSucceed() {
				return fmt.Errorf("failed to update node: %s", resp.GetErrorMsg())
			}
		}
	}

	return nil
}

// UpsertEdges batch updates or inserts edges
func (n *NebulaGraphStore) UpsertEdges(ctx context.Context, edges []*abstraction.Edge) error {
	for _, edge := range edges {
		// Check if edge exists
		existingEdge, err := n.GetEdge(ctx, edge.ID)
		if err != nil {
			return err
		}

		if existingEdge == nil {
			// Create new edge
			if err := n.CreateEdge(ctx, edge); err != nil {
				return err
			}
		} else {
			// Update existing edge
			conn, err := n.pool.GetConnection(ctx)
			if err != nil {
				return err
			}
			defer conn.Close()

			// Use the space
			if err := n.useSpace(conn, ctx); err != nil {
				return err
			}

			// Build the query
			query := fmt.Sprintf(
				"UPDATE EDGE \"%s\"->\"%s\" OF relationship SET relationship.type = \"%s\"",
				edge.Source, edge.Target, edge.Type,
			)

			resp, err := conn.Execute(ctx, query)
			if err != nil {
				return err
			}
			if !resp.IsSucceed() {
				return fmt.Errorf("failed to update edge: %s", resp.GetErrorMsg())
			}
		}
	}

	return nil
}

// Close closes the NebulaGraph connection pool
func (n *NebulaGraphStore) Close(ctx context.Context) error {
	if n.pool != nil {
		return n.pool.Close()
	}
	return nil
}

// useSpace sets the current space for the connection
func (n *NebulaGraphStore) useSpace(conn *nebulaPool.Connection, ctx context.Context) error {
	resp, err := conn.Execute(ctx, fmt.Sprintf("USE %s", n.spaceName))
	if err != nil {
		return err
	}
	if !resp.IsSucceed() {
		return fmt.Errorf("failed to use space: %s", resp.GetErrorMsg())
	}
	return nil
}

// getValue extracts the value from a Nebula value
func (n *NebulaGraphStore) getValue(value *nebula.Value) any {
	switch value.GetVal().(type) {
	case *nebula.Value_NullVal:
		return nil
	case *nebula.Value_BoolVal:
		return value.GetBoolVal()
	case *nebula.Value_IntVal:
		return value.GetIntVal()
	case *nebula.Value_DoubleVal:
		return value.GetDoubleVal()
	case *nebula.Value_StringVal:
		return value.GetStringVal()
	case *nebula.Value_TimeVal:
		return value.GetTimeVal()
	case *nebula.Value_DateVal:
		return value.GetDateVal()
	case *nebula.Value_DatetimeVal:
		return value.GetDatetimeVal()
	default:
		return nil
	}
}
